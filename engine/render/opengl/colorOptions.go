package opengl

import "github.com/go-gl/mathgl/mgl32"

// 控制单次绘制的颜色或顶点渐变
type ColorOptions struct {
	// 渐变起始颜色，未启用渐变时作为单色调制颜色
	StartColor mgl32.Vec4
	// 渐变结束颜色
	EndColor mgl32.Vec4
	// 是否启用顶点渐变
	UseGradient bool
	// 渐变方向角度，单位弧度
	AngleRadians float32
}

// 使用单色生成颜色参数
func solidColorOptions(color mgl32.Vec4) ColorOptions {
	return ColorOptions{
		StartColor: color,
		EndColor:   color,
	}
}

// 补全颜色参数默认值
func normalizeColorOptions(options *ColorOptions) ColorOptions {
	if options == nil {
		return solidColorOptions(mgl32.Vec4{1.0, 1.0, 1.0, 1.0})
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
