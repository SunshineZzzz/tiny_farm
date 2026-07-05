package defs

import (
	"tiny_farm/engine/utils/dispatch"

	"github.com/go-gl/mathgl/mgl32"
)

// 标识资源映射中的稳定语义 key
//
// 当前先用字符串保持调用和测试简单，后续如需数字 ID 可在内部补 hash
type ResourceKey string

// 动作名称在 Go 侧的标识
//
// 当前直接使用配置中的动作名称，避免为精简客户端额外引入哈希层
type ActionID string

// 鼠标左键在输入系统中的动作标识
const MouseLeftAction = ActionID("mouse_left")

// 表示动作在当前帧里的输入状态
//
// 该状态同时用于选择回调列表；Inactive 只表示未激活，不作为可绑定回调阶段
type ActionState int

const (
	// 动作在本帧刚被按下
	Pressed ActionState = iota
	// 动作已经持续按下
	Held
	// 动作在本帧刚被释放
	Released
	// 动作当前没有输入
	Inactive
)

// 表示动作回调的注册入口
type ActionSink = dispatch.SignalSink[bool]

// 表示一条可释放的动作回调连接
type ActionConnection = dispatch.SignalConnection[bool]

// 控制单次绘制的颜色或顶点渐变
type ColorOptions struct {
	// 渐变起始颜色，未启用渐变时作为单色调制颜色
	StartColor mgl32.Vec4
	// 渐变结束颜色
	EndColor mgl32.Vec4
	// 是否启用顶点渐变
	UseGradient bool
	// 渐变方向角度，单位弧度
	// 0 从左向右
	// π/2 从上向下
	// π 从右向左
	// 3π/2 从下向上
	AngleRadians float32
}

// 使用单色生成颜色参数
func SolidColorOptions(color mgl32.Vec4) ColorOptions {
	return ColorOptions{
		StartColor: color,
		EndColor:   color,
	}
}

// 使用单色生成文本颜色参数
func SolidTextColorOptions(color mgl32.Vec4) ColorOptions {
	return ColorOptions{
		StartColor: color,
		EndColor:   color,
	}
}

// 补全颜色参数默认值
func NormalizeColorOptions(options *ColorOptions) ColorOptions {
	if options == nil {
		return SolidColorOptions(mgl32.Vec4{1.0, 1.0, 1.0, 1.0})
	}
	resolved := *options
	if resolved.StartColor == (mgl32.Vec4{}) {
		resolved.StartColor = mgl32.Vec4{1.0, 1.0, 1.0, 1.0}
	}
	if resolved.EndColor == (mgl32.Vec4{}) {
		resolved.EndColor = resolved.StartColor
	}
	return resolved
}

// 字体路径参数，未传时默认使用 fontKey 字符串
type FontPath string

// 保证传参合法而已，不需要具体实现
func (FontPath) Dummy() {}

// 确保FontPath实现iTextArg接口
// 这个在 abstract.go 中定义，避免循环依赖
// var _ ITextArg = defs.FontPath("")

// 控制文本布局的基础参数
type LayoutOptions struct {
	// 字符额外间距，单位像素
	LetterSpacing float32
	// 行距缩放，0.0 表示使用 1.0
	LineSpacingScale float32
	// 字形缩放，0.0 分量表示使用 1.0
	GlyphScale mgl32.Vec2
}

// 保证传参合法而已，不需要具体实现
func (*LayoutOptions) Dummy() {}

// 确保LayoutOptions实现iTextArg接口
// 这个在 abstract.go 中定义，避免循环依赖
// var _ ITextArg = (*LayoutOptions)(nil)

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
	// 文字颜色或渐变参数
	Color ColorOptions
	// 阴影参数
	Shadow *ShadowOptions
	// 布局参数
	Layout *LayoutOptions
}

// 保证传参合法而已，不需要具体实现
func (*TextRenderParams) Dummy() {}

// 确保TextRenderParams实现iTextArg接口
// 这个在 abstract.go 中定义，避免循环依赖
// var _ ITextArg = (*TextRenderParams)(nil)

// 文本样式 key 参数
type TextStyleKey string

// 保证传参合法而已，不需要具体实现
func (TextStyleKey) Dummy() {}

// 确保TextStyleKey实现iTextArg接口
// 这个在 abstract.go 中定义，避免循环依赖
// var _ ITextArg = TextStyleKey("")

// 在样式基础上覆盖本次文本绘制参数
type TextRenderOverrides struct {
	// 覆盖文字颜色或渐变参数，nil表示使用样式颜色
	Color *ColorOptions
	// 覆盖阴影参数，nil表示使用样式阴影
	Shadow *ShadowOptions
	// 覆盖布局参数，nil表示使用样式布局
	Layout *LayoutOptions
}

// 保证传参合法而已，不需要具体实现
func (*TextRenderOverrides) Dummy() {}

// 确保TextRenderOverrides实现iTextArg接口
// 这个在 abstract.go 中定义，避免循环依赖
// var _ ITextArg = (*TextRenderOverrides)(nil)
