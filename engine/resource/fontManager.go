package resource

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"os"
	"slices"

	"tiny_farm/engine/abstract"
	"tiny_farm/engine/render"

	"github.com/go-gl/mathgl/mgl32"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// 字形图集里，每个glyph四周预留1像素间距
const glyphAtlasPadding = 1

// 标识一个已加载字体实例
//
// 同一字体资源可以按不同像素字号分别缓存
type fontKey struct {
	// 字体资源语义 key
	key ResourceKey
	// 字体像素字号
	pixelSize int
}

// 字符光栅化后写入atlas的缓存条目
//
// 当前按 rune 缓存，后续接入 typesetting 后可扩展为 glyph index 缓存
type FontGlyph struct {
	// glyph所在atlas纹理
	texture *render.Texture
	// glyph像素尺寸
	Size mgl32.Vec2
	// glyph相对基线原点的偏移，理解为"相对绘制起始点的偏移"，不是"绘制起始点本身"，需要结合光标
	Bearing mgl32.Vec2
	// 绘制后光标前进距离，单位像素
	Advance float32
	// glyph在atlas纹理中的UV范围，左上原点语义
	UVRect mgl32.Vec4
}

// 确保FontGlyph实现IFontGlyph接口
var _ abstract.IFontGlyph = (*FontGlyph)(nil)

// 返回glyph所在的atlas纹理
func (g *FontGlyph) Texture() *render.Texture {
	if g == nil {
		return nil
	}
	return g.texture
}

// 返回 glyph 所在纹理对象
func (g *FontGlyph) GlyphTexture() any {
	return g.Texture()
}

// 返回 glyph 像素尺寸
func (g *FontGlyph) GlyphSize() mgl32.Vec2 {
	if g == nil {
		return mgl32.Vec2{}
	}
	return g.Size
}

// 返回 glyph 相对基线原点的偏移
func (g *FontGlyph) GlyphBearing() mgl32.Vec2 {
	if g == nil {
		return mgl32.Vec2{}
	}
	return g.Bearing
}

// 返回绘制后光标前进距离，单位像素
func (g *FontGlyph) GlyphAdvance() float32 {
	if g == nil {
		return 0
	}
	return g.Advance
}

// 返回 glyph 在 atlas 纹理中的 UV 范围
func (g *FontGlyph) GlyphUVRect() mgl32.Vec4 {
	if g == nil {
		return mgl32.Vec4{}
	}
	return g.UVRect
}

// 字体atlas页
//
// 当前使用简单行分配策略，后续可替换为更紧凑的装箱算法
type fontAtlasPage struct {
	// atlas纹理
	texture *render.Texture
	// 当前行写入位置
	cursorX int32
	// 当前列写入位置
	cursorY int32
	// 当前行最高的glyph高度
	rowHeight int32
}

// 单个字体资源实例
//
// 当前阶段支持按需光栅化rune，并将glyph位图写入atlas
type Font struct {
	// 字体资源语义 key
	key ResourceKey
	// 字体像素字号
	pixelSize int
	// 字体来源路径
	sourcePath string
	// 原始字体文件数据
	data []byte
	// 用于创建atlas纹理的渲染器
	renderer *render.Renderer
	// 解析后的字体对象
	parsed *opentype.Font
	// 指定字号后的字体面
	face font.Face
	// 字体上升高度，从文字基线baseline向上到字体顶部的大致高度
	ascender float32
	// 字体下降深度，从文字基线baseline往下，到字体底部的大致距离
	descender float32
	// 字体默认行高，两行文字基线之间的垂直距离，包含字体本身高度(ascent + descent)和行间距(leading)，控制文本行与行的间隔
	lineHeight float32
	// 已缓存的glyph
	glyphs map[rune]*FontGlyph
	// atlas页
	atlasPages []*fontAtlasPage
	// atlas页的尺寸
	atlasPageSize int32
}

// 确保Font实现IFont接口
var _ abstract.IFont = (*Font)(nil)

// 返回字体资源语义 key
func (f *Font) Key() ResourceKey {
	if f == nil {
		return ""
	}
	return f.key
}

// 返回字体像素字号
func (f *Font) PixelSize() int {
	if f == nil {
		return 0
	}
	return f.pixelSize
}

// 返回字体来源路径
func (f *Font) SourcePath() string {
	if f == nil {
		return ""
	}
	return f.sourcePath
}

// 返回字体上升高度，单位像素
func (f *Font) Ascender() float32 {
	if f == nil {
		return 0
	}
	return f.ascender
}

// 返回字体下降深度，单位像素
func (f *Font) Descender() float32 {
	if f == nil {
		return 0
	}
	return f.descender
}

// 返回字体默认行高，单位像素
func (f *Font) LineHeight() float32 {
	if f == nil {
		return 0
	}
	return f.lineHeight
}

// 按需获取 rune 对应的 glyph atlas 条目
//
// 当前是 rune 级光栅化入口，暂不包含 HarfBuzz shaping
func (f *Font) Glyph(r rune) (*FontGlyph, error) {
	if f == nil {
		return nil, errors.New("font is nil")
	}
	if glyph, ok := f.glyphs[r]; ok && glyph != nil {
		return glyph, nil
	}
	if f.face == nil {
		return nil, errors.New("font face is nil")
	}

	// 从当前的字体文件(f.face)中，把字符(r)的位图(灰度位图遮罩)以及排版度量数据取出来
	// fixed.Point26_6{}，让字体库以(0,0)作为这个字符的绘制原点，计算它的字形区域、透明度mask和advance值
	// dr，glyph要画到目标图像上的矩形区域，类型是 type Rectangle struct { Min, Max Point }， 左上角和右下角的坐标
	// mask，作为alpha蒙版，把颜色填到目标图像上
	// maskp，在mask这张图里，从哪个坐标开始取字形像素
	// advance，绘制完这个字符后，笔的位置应该向右移动多少
	dr, mask, maskp, advance, ok := f.face.Glyph(fixed.Point26_6{}, r)
	if !ok {
		if r != '?' {
			return f.Glyph('?')
		}
		return nil, fmt.Errorf("glyph %q is not available in font %q", r, f.key)
	}

	glyph := &FontGlyph{
		Size: mgl32.Vec2{float32(dr.Dx()), float32(dr.Dy())},
		// 这里dr.Min.Y用的是image/font这套坐标约定：Y轴向下为正，所以需要取负
		Bearing: mgl32.Vec2{float32(dr.Min.X), float32(-dr.Min.Y)},
		Advance: fixedToFloat32(advance),
	}

	if dr.Dx() > 0 && dr.Dy() > 0 {
		pixels := glyphMaskToRGBA(mask, maskp, dr)
		page, position, err := f.allocateAtlasRegion(int32(dr.Dx()), int32(dr.Dy()))
		if err != nil {
			return nil, err
		}
		if err := page.texture.UpdateRGBA(int32(position.X()), int32(position.Y()), int32(dr.Dx()), int32(dr.Dy()), pixels); err != nil {
			return nil, fmt.Errorf("upload glyph %q to atlas: %w", r, err)
		}

		size := page.texture.Size()
		glyph.texture = page.texture
		// 把字形在字体图集中的像素矩形，记录成归一化UV矩形，后续渲染文字时根据这个UVRect从纹理中取出对应字形
		glyph.UVRect = mgl32.Vec4{
			position.X() / size.X(),
			position.Y() / size.Y(),
			(position.X() + float32(dr.Dx())) / size.X(),
			(position.Y() + float32(dr.Dy())) / size.Y(),
		}
	}

	f.glyphs[r] = glyph
	return glyph, nil
}

// 按需获取 rune 对应的文本渲染 glyph
func (f *Font) TextGlyph(r rune) (abstract.IFontGlyph, error) {
	return f.Glyph(r)
}

// 释放字体持有的 atlas 纹理
func (f *Font) close() {
	if f == nil {
		return
	}
	for _, page := range f.atlasPages {
		if page != nil && page.texture != nil {
			page.texture.Close()
		}
	}
	f.atlasPages = nil
	f.glyphs = nil
	if closer, ok := f.face.(interface{ Close() error }); ok {
		_ = closer.Close()
	}
	f.face = nil
	f.parsed = nil
	f.data = nil
}

// 给glyph位图分配atlas区域
func (f *Font) allocateAtlasRegion(width, height int32) (*fontAtlasPage, mgl32.Vec2, error) {
	if width <= 0 || height <= 0 {
		return nil, mgl32.Vec2{}, errors.New("glyph atlas region size is invalid")
	}

	paddedWidth := width + glyphAtlasPadding*2
	paddedHeight := height + glyphAtlasPadding*2
	for _, page := range f.atlasPages {
		if page == nil || page.texture == nil {
			continue
		}
		if paddedWidth > int32(page.texture.Size().X()) || paddedHeight > int32(page.texture.Size().Y()) {
			continue
		}

		cursorX := page.cursorX
		cursorY := page.cursorY
		rowHeight := page.rowHeight
		if cursorX+paddedWidth > int32(page.texture.Size().X()) {
			cursorX = 0
			cursorY += rowHeight
			rowHeight = 0
		}
		if cursorY+paddedHeight > int32(page.texture.Size().Y()) {
			continue
		}

		position := mgl32.Vec2{float32(cursorX + glyphAtlasPadding), float32(cursorY + glyphAtlasPadding)}
		page.cursorX = cursorX + paddedWidth
		page.cursorY = cursorY
		page.rowHeight = max(rowHeight, paddedHeight)
		return page, position, nil
	}

	// 新页大小至少是默认atlas大小；如果glyph特别大，就创建一个能放下它的更大页面
	pageSize := max(f.atlasPageSize, max(paddedWidth, paddedHeight))
	_, err := f.createAtlasPage(pageSize)
	if err != nil {
		return nil, mgl32.Vec2{}, err
	}
	return f.allocateAtlasRegion(width, height)
}

// 创建新的atlas页
func (f *Font) createAtlasPage(size int32) (*fontAtlasPage, error) {
	if f.renderer == nil {
		return nil, errors.New("font renderer is nil")
	}
	texture, err := f.renderer.CreateEmptyTexture(size, size, render.TextureFilterLinear)
	if err != nil {
		return nil, fmt.Errorf("create font atlas page: %w", err)
	}

	page := &fontAtlasPage{texture: texture}
	f.atlasPages = append(f.atlasPages, page)
	return page, nil
}

// 管理字体资源的加载、缓存和释放
//
// 当前阶段负责字体文件缓存、rune 级 glyph 光栅化和 atlas 生命周期
type fontManager struct {
	// 用于创建字体 atlas 纹理
	renderer *render.Renderer
	// 按字体 key 和字号缓存字体实例
	fonts map[fontKey]*Font
}

// 创建字体资源管理器
func newFontManager(renderer *render.Renderer) *fontManager {
	return &fontManager{
		renderer: renderer,
		fonts:    make(map[fontKey]*Font),
	}
}

// 加载字体并按 key 与字号缓存
//
// 如果同一 key 和字号已经加载，直接返回缓存实例
func (m *fontManager) loadFont(key ResourceKey, pixelSize int, paths ...string) (*Font, error) {
	if m == nil {
		return nil, errors.New("font manager is nil")
	}
	if key == "" {
		return nil, errors.New("font key is empty")
	}
	if pixelSize <= 0 {
		return nil, errors.New("font pixel size is invalid")
	}

	fontKey := fontKey{key: key, pixelSize: pixelSize}
	if font, ok := m.fonts[fontKey]; ok && font != nil {
		return font, nil
	}

	path := string(key)
	if len(paths) != 0 {
		path = paths[0]
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load font %q from %q: %w", key, path, err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("load font %q from %q: font file is empty", key, path)
	}
	parsed, err := opentype.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse font %q from %q: %w", key, path, err)
	}
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{
		Size:    float64(pixelSize),
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("create font face %q from %q: %w", key, path, err)
	}
	metrics := face.Metrics()

	font := &Font{
		key:           key,
		pixelSize:     pixelSize,
		sourcePath:    path,
		data:          data,
		renderer:      m.renderer,
		parsed:        parsed,
		face:          face,
		ascender:      fixedToFloat32(metrics.Ascent),
		descender:     fixedToFloat32(metrics.Descent),
		lineHeight:    fixedToFloat32(metrics.Height),
		glyphs:        make(map[rune]*FontGlyph),
		atlasPageSize: calculateAtlasPageSize(pixelSize),
	}
	m.fonts[fontKey] = font
	return font, nil
}

// 卸载指定字体实例
func (m *fontManager) unloadFont(key ResourceKey, pixelSize int) {
	if m == nil {
		return
	}
	fontKey := fontKey{key: key, pixelSize: pixelSize}
	if font, ok := m.fonts[fontKey]; ok && font != nil {
		font.close()
	}
	delete(m.fonts, fontKey)
}

// 清空全部字体缓存
func (m *fontManager) clearFonts() {
	if m == nil {
		return
	}
	for _, font := range m.fonts {
		if font != nil {
			font.close()
		}
	}
	m.fonts = make(map[fontKey]*Font)
}

// 返回按 key 和字号排序的字体调试信息
func (m *fontManager) fontDebugInfo() []FontDebugInfo {
	if m == nil || len(m.fonts) == 0 {
		return nil
	}

	keys := make([]fontKey, 0, len(m.fonts))
	for key, font := range m.fonts {
		if font == nil {
			continue
		}
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(a, b fontKey) int {
		if a.key < b.key {
			return -1
		}
		if a.key > b.key {
			return 1
		}
		return a.pixelSize - b.pixelSize
	})

	info := make([]FontDebugInfo, 0, len(keys))
	for _, key := range keys {
		font := m.fonts[key]
		info = append(info, FontDebugInfo{
			Key:            font.key,
			SourcePath:     font.sourcePath,
			PixelSize:      font.pixelSize,
			GlyphCount:     len(font.glyphs),
			AtlasPageCount: len(font.atlasPages),
			MemoryBytes:    estimateFontMemoryBytes(font),
		})
	}
	return info
}

// 根据字号选择 atlas 页尺寸
func calculateAtlasPageSize(pixelSize int) int32 {
	if pixelSize <= 24 {
		return 512
	}
	if pixelSize <= 60 {
		return 1024
	}
	return 2048
}

// 将 26.6 定点数转换成float32像素值
// 26.6，本质上还是一个整数，但约定为26位整数，6位小数(1px=64份)
// 3.5px，存到fixed.Int26_6里时，会变成
// 整数部分3左移6位，给小数腾出6位：
// 3 << 6 = 192
// 小数部分32放到低6位：
// 192 + 32 = 224
func fixedToFloat32(value fixed.Int26_6) float32 {
	//3.5 => 3 * 2^6 + 64*0.5 = 224 => / 64 = 3.5
	return float32(value) / 64.0
}

// 将glyph alpha mask转换成RGBA像素
func glyphMaskToRGBA(mask image.Image, maskp image.Point, dr image.Rectangle) []byte {
	width := dr.Dx()
	height := dr.Dy()
	pixels := make([]byte, width*height*4)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			alpha := color.AlphaModel.Convert(mask.At(maskp.X+x, maskp.Y+y)).(color.Alpha).A
			index := (y*width + x) * 4
			pixels[index+0] = 255
			pixels[index+1] = 255
			pixels[index+2] = 255
			pixels[index+3] = alpha
		}
	}
	return pixels
}

// 估算字体缓存当前占用的内存与显存
func estimateFontMemoryBytes(font *Font) int {
	if font == nil {
		return 0
	}
	total := len(font.data)
	for _, page := range font.atlasPages {
		if page == nil || page.texture == nil {
			continue
		}
		size := page.texture.Size()
		total += int(size.X()) * int(size.Y()) * 4
	}
	return total
}
