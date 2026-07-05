package resource

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"slices"

	"tiny_farm/engine/abstract"
	"tiny_farm/engine/render"

	"github.com/go-gl/mathgl/mgl32"
	textfont "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/harfbuzz"
	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
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
// 当前按 glyph index 缓存，与 HarfBuzz shaping 输出保持同一套索引语义
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
// 当前阶段支持按需光栅化 glyph index，并将 glyph 位图写入 atlas
type Font struct {
	// 字体资源语义 key
	key ResourceKey
	// 字体像素字号，比如：24px
	// 字体里的 24px 通常对应的是字体的 em size，也就是字体设计坐标被缩放到 24 像素这个规格
	// em size 不是某个字的实际宽高，而是字体系统用来计算这些东西的基准
	// 字形缩放比例
	// 基线位置
	// 上升高度 ascender
	// 下降深度 descender
	// 行高 line height
	// 字符前进距离 advance
	pixelSize int
	// 字体来源路径
	sourcePath string
	// 原始字体文件数据
	data []byte
	// 用于创建atlas纹理的渲染器
	renderer *render.Renderer
	// 解析后的字体对象
	parsed *opentype.Font
	// 字体面：某个字体文件在指定字号、指定渲染配置下生成出来的可用字体对象。
	// 指定字号后的字体面
	face xfont.Face
	// HarfBuzz 使用的字体面
	shapingFace *textfont.Face
	// HarfBuzz 使用的字体对象
	shapingFont *harfbuzz.Font
	// 复用的 sfnt 临时缓冲
	sfntBuffer sfnt.Buffer
	// 复用的 glyph 光栅化器
	rasterizer vector.Rasterizer
	// 复用的 glyph alpha mask
	glyphMask image.Alpha
	// 字体上升高度，从文字基线baseline向上到字体顶部的大致高度
	ascender float32
	// 字体下降深度，从文字基线baseline往下，到字体底部的大致距离
	descender float32
	// 字体默认行高，两行文字基线之间的垂直距离，包含字体本身高度(ascent + descent)和行间距(leading)，控制文本行与行的间隔
	lineHeight float32
	// 已缓存的glyph，key 是字体内部 glyph index
	glyphs map[uint32]*FontGlyph
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
// 当前是兼容旧调用的 rune 入口，内部先映射到字体 glyph index
func (f *Font) Glyph(r rune) (*FontGlyph, error) {
	if f == nil {
		return nil, errors.New("font is nil")
	}
	if f.parsed == nil {
		return nil, errors.New("font parsed data is nil")
	}

	index, err := f.parsed.GlyphIndex(&f.sfntBuffer, r)
	if err != nil || index == 0 {
		if r != '?' {
			return f.Glyph('?')
		}
		return nil, fmt.Errorf("glyph %q is not available in font %q", r, f.key)
	}
	return f.GlyphByIndex(uint32(index))
}

// 按需获取 glyph index 对应的 glyph atlas 条目
func (f *Font) GlyphByIndex(index uint32) (*FontGlyph, error) {
	if f == nil {
		return nil, errors.New("font is nil")
	}
	if glyph, ok := f.glyphs[index]; ok && glyph != nil {
		return glyph, nil
	}
	if f.parsed == nil {
		return nil, errors.New("font parsed data is nil")
	}
	if index > uint32(^sfnt.GlyphIndex(0)) {
		return nil, fmt.Errorf("glyph index %d is out of range in font %q", index, f.key)
	}

	// 类型转换为 sfnt.GlyphIndex 类型
	glyphIndex := sfnt.GlyphIndex(index)
	// 将像素字号转换为 fixed.Int26_6，后面的字体度量和光栅化都按照该尺寸进行
	ppem := fixed.I(f.pixelSize)
	// 当前字形画完以后，光标应该向右移动多少距离
	advance, err := f.parsed.GlyphAdvance(&f.sfntBuffer, glyphIndex, ppem, xfont.HintingFull)
	if err != nil {
		return nil, fmt.Errorf("load glyph advance %d from font %q: %w", index, f.key, err)
	}
	// 光栅化字形，将字体中的矢量轮廓转换为像素遮罩
	// dr，glyph要画到目标图像上的矩形区域，类型是 type Rectangle struct { Min, Max Point }， 左上角和右下角的坐标
	// mask，作为alpha蒙版，把颜色填到目标图像上
	// maskp，在mask这张图里，从哪个坐标开始取字形像素
	dr, mask, maskp, err := f.rasterizeGlyph(glyphIndex, ppem)
	if err != nil {
		return nil, fmt.Errorf("rasterize glyph %d from font %q: %w", index, f.key, err)
	}

	// 保存字形度量
	glyph := &FontGlyph{
		Size: mgl32.Vec2{float32(dr.Dx()), float32(dr.Dy())},
		// 这里dr.Min.Y用的是image/font这套坐标约定：Y轴向下为正，所以需要取负
		Bearing: mgl32.Vec2{float32(dr.Min.X), float32(-dr.Min.Y)},
		Advance: fixedToFloat32(advance),
	}

	// 在字体渲染场景里，dr 很可能是某个字形 glyph 的绘制区域。这个判断通常是为了跳过空字形。
	if dr.Dx() > 0 && dr.Dy() > 0 {
		// 把字体光栅化出来的 mask 转成 RGBA 像素数据。
		pixels := glyphMaskToRGBA(mask, maskp, dr)
		// 在字体图集 atlas 里，为这个字形申请一块区域。
		page, position, err := f.allocateAtlasRegion(int32(dr.Dx()), int32(dr.Dy()))
		if err != nil {
			return nil, err
		}
		if err := page.texture.UpdateRGBA(int32(position.X()), int32(position.Y()), int32(dr.Dx()), int32(dr.Dy()), pixels); err != nil {
			return nil, fmt.Errorf("upload glyph %d to atlas: %w", index, err)
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

	f.glyphs[index] = glyph
	return glyph, nil
}

// 按需获取 rune 对应的文本渲染 glyph
func (f *Font) TextGlyph(r rune) (abstract.IFontGlyph, error) {
	return f.Glyph(r)
}

// 按需获取 glyph index 对应的文本渲染 glyph
func (f *Font) TextGlyphByIndex(index uint32) (abstract.IFontGlyph, error) {
	return f.GlyphByIndex(index)
}

// 返回 HarfBuzz shaping 字体对象
func (f *Font) TextShapingFont() any {
	if f == nil {
		return nil
	}
	return f.shapingFont
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
	f.shapingFace = nil
	f.shapingFont = nil
	f.data = nil
}

// 根据 glyph index 加载指定 ppem 大小的字形轮廓，然后将矢量轮廓转换成 Alpha mask 和绘制矩形
//
// Alpha mask 生成流程参考 golang.org/x/image/font/opentype.Face.Glyph
// font库只把 rune 版本公开了，没有公开 glyph-index 版本
func (f *Font) rasterizeGlyph(index sfnt.GlyphIndex, ppem fixed.Int26_6) (image.Rectangle, image.Image, image.Point, error) {
	// 加载字形轮廓
	segments, err := f.parsed.LoadGlyph(&f.sfntBuffer, index, ppem, nil)
	if err != nil {
		return image.Rectangle{}, nil, image.Point{}, err
	}

	// 计算字形边界
	dot := fixed.Point26_6{}
	bounds := segments.Bounds().Add(dot)
	dr := image.Rectangle{}
	// 转换为整数像素矩形
	dr.Min.X = bounds.Min.X.Floor()
	dr.Min.Y = bounds.Min.Y.Floor()
	dr.Max.X = bounds.Max.X.Ceil()
	dr.Max.Y = bounds.Max.Y.Ceil()
	width := dr.Dx()
	height := dr.Dy()
	if width < 0 || height < 0 {
		return image.Rectangle{}, nil, image.Point{}, errors.New("glyph bounds is invalid")
	}

	// 生成alpha mask和绘制矩形

	biasX := dot.X - fixed.Int26_6(dr.Min.X<<6)
	biasY := dot.Y - fixed.Int26_6(dr.Min.Y<<6)
	pixelCount := width * height
	if cap(f.glyphMask.Pix) < pixelCount {
		f.glyphMask.Pix = make([]byte, pixelCount*2)
	}
	f.glyphMask.Pix = f.glyphMask.Pix[:pixelCount]
	clear(f.glyphMask.Pix)
	f.glyphMask.Stride = width
	f.glyphMask.Rect = image.Rect(0, 0, width, height)

	f.rasterizer.Reset(width, height)
	f.rasterizer.DrawOp = draw.Src
	for _, segment := range segments {
		switch segment.Op {
		case sfnt.SegmentOpMoveTo:
			f.rasterizer.MoveTo(
				float32(segment.Args[0].X+biasX)/64.0,
				float32(segment.Args[0].Y+biasY)/64.0,
			)
		case sfnt.SegmentOpLineTo:
			f.rasterizer.LineTo(
				float32(segment.Args[0].X+biasX)/64.0,
				float32(segment.Args[0].Y+biasY)/64.0,
			)
		case sfnt.SegmentOpQuadTo:
			f.rasterizer.QuadTo(
				float32(segment.Args[0].X+biasX)/64.0,
				float32(segment.Args[0].Y+biasY)/64.0,
				float32(segment.Args[1].X+biasX)/64.0,
				float32(segment.Args[1].Y+biasY)/64.0,
			)
		case sfnt.SegmentOpCubeTo:
			f.rasterizer.CubeTo(
				float32(segment.Args[0].X+biasX)/64.0,
				float32(segment.Args[0].Y+biasY)/64.0,
				float32(segment.Args[1].X+biasX)/64.0,
				float32(segment.Args[1].Y+biasY)/64.0,
				float32(segment.Args[2].X+biasX)/64.0,
				float32(segment.Args[2].Y+biasY)/64.0,
			)
		}
	}
	f.rasterizer.Draw(&f.glyphMask, f.glyphMask.Bounds(), image.Opaque, image.Point{})
	return dr, &f.glyphMask, f.glyphMask.Rect.Min, nil
}

// 给glyph位图分配atlas区域
// return:
// *fontAtlasPage，表示分配到的 atlas 页。字体 atlas 可能不止一张纹理，一张放满了就新建下一张。这个返回值告诉调用方：这次 glyph 应该写进哪一张 atlas 纹理。
// mgl32.Vec2，表示 glyph 位图在这张 atlas 纹理里的 写入位置，也就是左上角坐标。
// error，表示分配失败的原因。
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
	// 创建一张 pageSize x pageSize 的空纹理，并加入 f.atlasPages。
	_, err := f.createAtlasPage(pageSize)
	if err != nil {
		return nil, mgl32.Vec2{}, err
	}
	// 递归调用，为新创建的 atlas 页分配区域。
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
	// 从“字体文件数据”变成“可渲染字体面”
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{
		// 字体点数
		//
		// point，pt，排版常用长度单位，1 pt = 1/72 英寸，1英寸 = 约 2.54 厘米
		// DPI，dots per inch，在当前语境下，就是PPI，Pixels Per Inch，每英寸多少像素/点，常见 72、96、144 等
		// pixel，px，屏幕上的最小显示单位
		//
		// 像素大小（px）= 字号（pt）× PPI ÷ 72 = pixelSize
		// 所以当 DPI = 72 时，Size = pixelSize 就等价于：1 point ≈ 1 像素。
		// 这样 pixelSize = 16 就是 16px 字号，直观好配。
		Size: float64(pixelSize),
		// 配合 Size 决定 em size
		// 例如 Size = 16，DPI = 96：
		// px = 16 × (96 ÷ 72) = 16 × 1.333... ≈ 21.3 像素
		DPI: 72,
		// 启用完整 hinting，让较小字号的字形尽量对齐像素网格
		Hinting: xfont.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("create font face %q from %q: %w", key, path, err)
	}
	// 创建 HarfBuzz shaping 要使用的字体面
	shapingFace, err := textfont.ParseTTF(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("parse shaping font %q from %q: %w", key, path, err)
	}
	// 创建 HarfBuzz 字体实例
	shapingFont := harfbuzz.NewFont(shapingFace)
	// 设置字号，单位是 point
	// 项目把 pixelSize 直接放进去，是因为整体约定使用 DPI = 72，让 pt 数值和 px 数值 1:1
	shapingFont.Ptem = float32(pixelSize)
	// 横向、纵向排版尺寸。乘 64 这是为了与 FreeType 等字体系统的 1/64px 单位保持一致。
	// HarfBuzz 很多位置数据不是直接用 float 像素，而是用 26.6 fixed point，也就是：
	// 1px = 64 个单位
	// 为什么要乘 64？
	// 因为字体排版需要亚像素精度。比如一个 glyph 的 advance 可能不是整数像素：
	// 7.5px
	// 用 1/64px 表示就是：
	// 7.5 * 64 = 480
	// 这样可以用整数保存更精细的位置，避免浮点误差，也符合 FreeType/HarfBuzz 常用格式。
	shapingFont.XScale = int32(pixelSize * 64)
	shapingFont.YScale = int32(pixelSize * 64)
	// 获取字体度量信息
	metrics := face.Metrics()

	font := &Font{
		key:           key,
		pixelSize:     pixelSize,
		sourcePath:    path,
		data:          data,
		renderer:      m.renderer,
		parsed:        parsed,
		face:          face,
		shapingFace:   shapingFace,
		shapingFont:   shapingFont,
		ascender:      fixedToFloat32(metrics.Ascent),
		descender:     fixedToFloat32(metrics.Descent),
		lineHeight:    fixedToFloat32(metrics.Height),
		glyphs:        make(map[uint32]*FontGlyph),
		atlasPageSize: calculateAtlasPageSize(pixelSize),
	}
	m.fonts[fontKey] = font
	return font, nil
}

// 卸载指定字体实例
//
// 返回 true 表示确实释放了一个已缓存字体
func (m *fontManager) unloadFont(key ResourceKey, pixelSize int) bool {
	if m == nil {
		return false
	}
	fontKey := fontKey{key: key, pixelSize: pixelSize}
	if font, ok := m.fonts[fontKey]; ok && font != nil {
		font.close()
		delete(m.fonts, fontKey)
		return true
	}
	delete(m.fonts, fontKey)
	return false
}

// 清空全部字体缓存
//
// 返回 true 表示清理前存在字体缓存
func (m *fontManager) clearFonts() bool {
	if m == nil {
		return false
	}
	hadFonts := len(m.fonts) != 0
	for _, font := range m.fonts {
		if font != nil {
			font.close()
		}
	}
	m.fonts = make(map[fontKey]*Font)
	return hadFonts
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
// 目的：
// 让整数也能表示小数像素，比如，字距不一定是整数像素，8.5 像素，但 int32 只能保存整数，不能保存 8.5
// 所以约定：1 像素 = 64 个小单位，这样 8.5 像素 就能写成整数：8.5 × 64 = 544
// 程序保存 544，需要转换回像素时：544 ÷ 64 = 8.5 像素
// 所以乘以 64，就是把“小数像素”放大为整数保存
func fixedToFloat32(value fixed.Int26_6) float32 {
	//3.5 => 3 * 2^6 + 64*0.5 = 224 => / 64 = 3.5
	return float32(value) / 64.0
}

// 将glyph alpha mask转换成RGBA像素
func glyphMaskToRGBA(mask image.Image, maskp image.Point, dr image.Rectangle) []byte {
	width := dr.Dx()
	height := dr.Dy()
	pixels := make([]byte, width*height*4)
	for y := range height {
		for x := range width {
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
