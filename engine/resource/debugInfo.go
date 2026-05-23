package resource

import "github.com/go-gl/mathgl/mgl32"

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
