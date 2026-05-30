package render

import (
	"errors"

	"tiny_farm/engine/abstract"
	"tiny_farm/engine/utils/defs"

	"github.com/go-gl/mathgl/mgl32"
)

// 文本渲染可选参数接口
type iTextArg interface {
	// 保证传参合法而已，不需要具体实现
	dummy()
}

// 控制文本布局的基础参数
//
// 当前只按 rune 顺序排版，复杂文本 shaping 后续由 typesetting 阶段接入
type LayoutOptions struct {
	// 字符额外间距，单位像素
	LetterSpacing float32
	// 行距缩放，0.0表示使用1.0
	LineSpacingScale float32
	// 字形缩放，0.0分量表示使用1.0
	GlyphScale mgl32.Vec2
}

// 保证传参合法而已，不需要具体实现
func (*LayoutOptions) dummy() {}

// 确保LayoutOptions实现iTextArg接口
var _ iTextArg = (*LayoutOptions)(nil)

// 控制文本阴影绘制
type ShadowOptions struct {
	// 是否绘制阴影
	Enabled bool
	// 阴影相对文字位置的偏移，单位像素
	Offset mgl32.Vec2
	// 阴影颜色
	Color mgl32.Vec4
}

// 控制一次文本绘制的颜色和布局
type TextRenderParams struct {
	// 文字颜色，零值表示白色
	Color mgl32.Vec4
	// 阴影参数
	Shadow *ShadowOptions
	// 布局参数
	Layout *LayoutOptions
}

// 保证传参合法而已，不需要具体实现
func (*TextRenderParams) dummy() {}

// 确保TextRenderParams实现iTextArg接口
var _ iTextArg = (*TextRenderParams)(nil)

// 字体路径参数，未传时默认使用 fontKey 字符串
type FontPath string

// 保证传参合法而已，不需要具体实现
func (FontPath) dummy() {}

// 确保FontPath实现iTextArg接口
var _ iTextArg = FontPath("")

// 负责把字体 glyph 串成可测量和可绘制文本
//
// 当前复用 FontManager 的 rune 级 glyph cache，并调用 Renderer 绘制 atlas 子区域
type TextRenderer struct {
	// 用于查询已加载字体资源
	resourceManager abstract.IResourceManager
	// 用于提交 glyph 贴图绘制命令
	renderer *Renderer
}

// 创建文本渲染器
//
// 当前只依赖已加载字体查询接口，绘制调用由上层显式持有 TextRenderer 发起
func NewTextRenderer(resourceManager abstract.IResourceManager, renderer *Renderer) (*TextRenderer, error) {
	if resourceManager == nil {
		return nil, errors.New("text renderer resource manager is nil")
	}
	if renderer == nil {
		return nil, errors.New("text renderer renderer is nil")
	}
	return &TextRenderer{
		resourceManager: resourceManager,
		renderer:        renderer,
	}, nil
}

// 测量已加载字体下的文本包围尺寸
func (t *TextRenderer) MeasureText(text string, fontKey defs.ResourceKey, pixelSize int, otherArgs ...iTextArg) (mgl32.Vec2, error) {
	fontPath := string(fontKey)
	var options *LayoutOptions
	for _, arg := range otherArgs {
		switch value := arg.(type) {
		case FontPath:
			fontPath = string(value)
		case *LayoutOptions:
			options = value
		case *TextRenderParams:
			if value != nil {
				options = value.Layout
			}
		}
	}
	font, err := t.resourceManager.LoadFont(fontKey, pixelSize, fontPath)
	if err != nil {
		return mgl32.Vec2{}, err
	}
	return measureTextWithFont(text, font, normalizeLayoutOptions(options))
}

// 绘制 UI 逻辑坐标系文本
func (t *TextRenderer) DrawUIText(text string, fontKey defs.ResourceKey, pixelSize int, position mgl32.Vec2, otherArgs ...iTextArg) error {
	fontPath := string(fontKey)
	var params *TextRenderParams
	for _, arg := range otherArgs {
		switch value := arg.(type) {
		case FontPath:
			fontPath = string(value)
		case *TextRenderParams:
			params = value
		case *LayoutOptions:
			params = &TextRenderParams{Layout: value}
		}
	}
	return t.drawText(text, fontKey, pixelSize, position, fontPath, normalizeTextRenderParams(params), true)
}

// 绘制世界坐标系文本
func (t *TextRenderer) DrawWorldText(text string, fontKey defs.ResourceKey, pixelSize int, position mgl32.Vec2, otherArgs ...iTextArg) error {
	fontPath := string(fontKey)
	var params *TextRenderParams
	for _, arg := range otherArgs {
		switch value := arg.(type) {
		case FontPath:
			fontPath = string(value)
		case *TextRenderParams:
			params = value
		case *LayoutOptions:
			params = &TextRenderParams{Layout: value}
		}
	}
	return t.drawText(text, fontKey, pixelSize, position, fontPath, normalizeTextRenderParams(params), false)
}

// 绘制一段文本，可按参数决定使用 UI 坐标或世界坐标
//
// 当前先绘制阴影再绘制正文，阴影复用同一套 glyph 布局
func (t *TextRenderer) drawText(text string, fontKey defs.ResourceKey, pixelSize int, position mgl32.Vec2, fontPath string, params *TextRenderParams, ui bool) error {
	if text == "" {
		return nil
	}
	if t == nil || t.renderer == nil {
		return errors.New("text renderer renderer is nil")
	}
	font, err := t.resourceManager.LoadFont(fontKey, pixelSize, fontPath)
	if err != nil {
		return err
	}
	if params.Shadow != nil && params.Shadow.Enabled {
		shadowPosition := position.Add(params.Shadow.Offset)
		if err := t.drawTextRunes(text, font, shadowPosition, params.Layout, params.Shadow.Color, ui); err != nil {
			return err
		}
	}
	return t.drawTextRunes(text, font, position, params.Layout, params.Color, ui)
}

// 按 rune 顺序把文本拆成 glyph 并提交绘制命令
//
// 当前通过字体接口按需生成 atlas 条目，后续 shaping 接入后会改为按 glyph index 序列绘制
func (t *TextRenderer) drawTextRunes(text string, font abstract.IFont, position mgl32.Vec2, options *LayoutOptions, color mgl32.Vec4, ui bool) error {
	// 笔尖位置
	penX := float32(0)
	// 当前行索引
	lineIndex := 0
	// 每换一行时向下推进的距离 = 字体默认行高 * 行距缩放 * 垂直字形缩放
	lineStep := font.LineHeight() * options.LineSpacingScale * options.GlyphScale.Y()
	if lineStep <= 0 {
		lineStep = float32(font.PixelSize())
	}

	// 按rune遍历文本绘制
	// 当前实现就是"一个rune对一个glyph"的简化排版模型
	for _, r := range text {
		if r == '\n' {
			lineIndex++
			penX = 0
			continue
		}

		glyph, err := font.TextGlyph(r)
		if err != nil {
			return err
		}
		if glyph == nil {
			continue
		}

		// 当前行baselineY = 文本起始顶部Y + 字体上升高度 + 当前行偏移
		// 一行里的所有glyph共用同一条baseline
		baselineY := position.Y() + font.Ascender()*options.GlyphScale.Y() + float32(lineIndex)*lineStep
		// glyph最终绘制矩形{x, y, width, height}
		dstRect := mgl32.Vec4{
			// glyph顶部X = 文本起始X + 当前行笔尖X + glyph相对笔尖的水平偏移
			position.X() + penX + glyph.GlyphBearing().X()*options.GlyphScale.X(),
			// glyph顶部Y = baselineY - 顶部到baseline的距离
			baselineY - glyph.GlyphBearing().Y()*options.GlyphScale.Y(),
			// glyph宽度 = 字体宽度 * 水平字形缩放
			glyph.GlyphSize().X() * options.GlyphScale.X(),
			// glyph高度 = 字体高度 * 垂直字形缩放
			glyph.GlyphSize().Y() * options.GlyphScale.Y(),
		}
		// 渲染
		texture, ok := glyph.GlyphTexture().(*Texture)
		if ok && texture != nil && dstRect.Z() > 0 && dstRect.W() > 0 {
			if ui {
				err = t.renderer.DrawUITextureColor(texture, dstRect, glyph.GlyphUVRect(), color)
			} else {
				err = t.renderer.DrawWorldTextureColor(texture, dstRect, glyph.GlyphUVRect(), color)
			}
			if err != nil {
				return err
			}
		}
		// 笔尖前进到下一个字符的位置
		penX += glyph.GlyphAdvance()*options.GlyphScale.X() + options.LetterSpacing
	}
	return nil
}

// 使用指定字体测量文本绘制包围尺寸
//
// 当前测量逻辑与绘制逻辑保持同一套 rune 顺序布局，测量过程会触发 glyph 按需缓存
func measureTextWithFont(text string, font abstract.IFont, options *LayoutOptions) (mgl32.Vec2, error) {
	if font == nil {
		return mgl32.Vec2{}, errors.New("font is nil")
	}
	if text == "" {
		return mgl32.Vec2{}, nil
	}

	// 当前代码一个字形对应一个字符，实际上字形和字符不是一个概念，并不是一一对应
	// 真实排版是，多个字符 -> 一个 glyph，一个字符 -> 多个 glyph，等等

	// 笔尖位置
	penX := float32(0.0)
	// 记录所有行里出现过的最大宽度
	maxWidth := float32(0.0)
	// 记录行数，默认至少有一行
	lineCount := 1
	// 逐个rune模拟排版并测量最宽行；过程中每个rune会通过字体接口获取glyph，如果缓存不存在会按需生成、上传到atlas并缓存
	for _, r := range text {
		if r == '\n' {
			// 结算当前行宽度，penX在每个字符后面都会加一次LetterSpacing，所以行尾要减掉最后一次多加的字距
			maxWidth = max(maxWidth, penX-options.LetterSpacing)
			penX = 0
			lineCount++
			continue
		}
		glyph, err := font.TextGlyph(r)
		if err != nil {
			return mgl32.Vec2{}, err
		}
		if glyph == nil {
			continue
		}
		// glyph实际绘制区域的右边界
		drawRight := penX + glyph.GlyphBearing().X()*options.GlyphScale.X() + glyph.GlyphSize().X()*options.GlyphScale.X()
		// 画完这个glyph后笔尖前进后的右边界位置
		advanceRight := penX + glyph.GlyphAdvance()*options.GlyphScale.X()
		// 普通字体、普通字符、非斜体场景下，大多数时候 drawRight <= advanceRight
		maxWidth = max(maxWidth, drawRight, advanceRight)
		// 笔尖前进到下一个字符的位置
		penX += glyph.GlyphAdvance()*options.GlyphScale.X() + options.LetterSpacing
	}
	maxWidth = max(maxWidth, penX-options.LetterSpacing)

	// 行与行之间的基线距离 = 字体默认行高 * 行距缩放 * 垂直字形缩放
	lineStep := font.LineHeight() * options.LineSpacingScale * options.GlyphScale.Y()
	if lineStep <= 0 {
		lineStep = float32(font.PixelSize())
	}
	// 第一行自身高度只看字体行高和glyph垂直缩放，不看行距缩放
	height := font.LineHeight() * options.GlyphScale.Y()
	// 如果文本不止一行
	if lineCount > 1 {
		// 总高度 = 第一行高度 + 后续每增加一行加一个lineStep
		height += float32(lineCount-1) * lineStep
	}
	// 返回测量结果，x = 最大行宽，y = 总高度
	return mgl32.Vec2{maxWidth, height}, nil
}

// 规范化文本绘制参数
//
// 当前零值颜色按白色处理，布局参数交给 normalizeLayoutOptions 填默认值
func normalizeTextRenderParams(params *TextRenderParams) *TextRenderParams {
	if params == nil {
		params = &TextRenderParams{}
	} else {
		copied := *params
		params = &copied
	}

	params.Layout = normalizeLayoutOptions(params.Layout)
	if (params.Color == mgl32.Vec4{}) {
		params.Color = mgl32.Vec4{1.0, 1.0, 1.0, 1.0}
	}
	return params
}

// 规范化布局参数
//
// 当前把行高缩放和 glyph 缩放的零值分量视为 1，方便调用方使用结构体零值
func normalizeLayoutOptions(options *LayoutOptions) *LayoutOptions {
	if options == nil {
		return &LayoutOptions{
			LetterSpacing:    0.0,
			LineSpacingScale: 1.0,
			GlyphScale:       mgl32.Vec2{1.0, 1.0},
		}
	}

	normalized := *options
	if normalized.LineSpacingScale == 0.0 {
		normalized.LineSpacingScale = 1.0
	}
	if normalized.GlyphScale.X() == 0.0 {
		normalized.GlyphScale[0] = 1.0
	}
	if normalized.GlyphScale.Y() == 0.0 {
		normalized.GlyphScale[1] = 1.0
	}
	return &normalized
}
