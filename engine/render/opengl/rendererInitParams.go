package opengl

import (
	"encoding/json"
	"math"
	"os"

	"github.com/SunshineZzzz/purego-sdl3/sdl"
)

const (
	// 表示保持交换间隔不变
	swapIntervalDontCare int32 = math.MinInt32
)

// 是 OpenGL 渲染初始化参数
type rendererInitParams struct {
	// 请求的 OpenGL 主版本号
	GLMajorVersion int32 `json:"gl_major_version"`
	// 请求的 OpenGL 次版本号
	GLMinorVersion int32 `json:"gl_minor_version"`
	// 上下文配置掩码
	ProfileMask sdl.GLProfile `json:"profile_mask"`
	// 额外的 SDL GL 上下文标记
	ContextFlags sdl.GLContextFlag `json:"context_flags"`
	// 表示是否请求双缓冲
	DoubleBuffer bool `json:"double_buffer"`
	// 默认帧缓冲的深度缓冲位数
	DepthBits int32 `json:"depth_bits"`
	// 默认帧缓冲的模板缓冲位数
	StencilBits int32 `json:"stencil_bits"`
	// 多重采样缓冲数量
	MultiSampleBuffers int32 `json:"multi_sample_buffers"`
	// 多重采样采样数
	MultiSampleSamples int32 `json:"multi_sample_samples"`
	// 表示是否请求支持 sRGB 的帧缓冲
	FramebufferSRGBCapable int32 `json:"framebuffer_SRGB_capable"`
	// 交换间隔，1 表示启用垂直同步，0 表示立即交换
	SwapInterval int32 `json:"swap_interval"`
}

// 对应渲染配置 JSON 根结构
type rendererInitParamsWrapper struct {
	Params *rendererInitParams `json:"params"`
}

// 创建渲染初始化参数
func NewRendererInitParams() *rendererInitParams {
	return &rendererInitParams{
		GLMajorVersion: 3,
		GLMinorVersion: 3,
		// 使用核心模式并禁用过时的固定管线功能
		ProfileMask:            sdl.GLContextProfileCore,
		ContextFlags:           0,
		DoubleBuffer:           true,
		DepthBits:              24,
		StencilBits:            8,
		MultiSampleBuffers:     0,
		MultiSampleSamples:     0,
		FramebufferSRGBCapable: 1,
		SwapInterval:           1,
	}
}

// 从文件加载渲染初始化参数
func loadConfigFromFile(path string) (*rendererInitParams, error) {
	params := NewRendererInitParams()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var wrapper rendererInitParamsWrapper
	wrapper.Params = params

	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}
	params = wrapper.Params

	if params.SwapInterval < 0 {
		params.SwapInterval = swapIntervalDontCare
	}

	return params, nil
}
