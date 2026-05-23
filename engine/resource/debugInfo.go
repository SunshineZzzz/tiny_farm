package resource

import (
	"time"

	"github.com/go-gl/mathgl/mgl32"
)

// 描述已缓存纹理资源的调试信息
//
// 当前用于日志、测试和后续 Debug UI，不暴露底层 OpenGL 句柄
type TextureDebugInfo struct {
	// 资源语义 key
	Key ResourceKey
	// 纹理来源路径
	SourcePath string
	// 纹理像素尺寸
	Size mgl32.Vec2
	// 按 RGBA8 估算的纹理内存字节数
	MemoryBytes int
}

// 描述已缓存音频资源的调试信息
//
// 当前用于确认音效和音乐是否已完成解码缓存
type AudioDebugInfo struct {
	// 资源语义 key
	Key ResourceKey
	// 音频来源路径
	SourcePath string
	// 采样率
	SampleRate int
	// 声道数量
	NumChannels int
	// 单声道采样精度字节数
	Precision int
	// 缓存中的采样帧数量
	SampleCount int
	// 根据采样率估算的时长
	Duration time.Duration
	// 按 buffer 格式估算的内存字节数
	MemoryBytes int
}
