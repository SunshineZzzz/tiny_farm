package opengl

import "github.com/go-gl/mathgl/mgl32"

// 单个渲染 pass 的上一帧统计
//
// 当前只记录 CPU 可直接得到的提交规模，用于后续 DebugPanel 或日志查看
type PassStats struct {
	// 本 pass 是否启用
	Enabled bool
	// 本 pass 实际提交的 draw call 数量
	DrawCalls int
	// 本 pass 提交的精灵或 quad 数量
	Sprites int
	// 本 pass 提交的顶点数量
	Vertices int
	// 本 pass 提交的索引数量
	Indices int
	// 本 pass 处理的光源数量
	Lights int
	// Bloom pass 当前持有的降采样层数
	BloomLevels int
	// Bloom pass 当前使用的高斯模糊 Sigma
	BloomSigma float32
}

// 渲染器上一帧统计
type RenderStats struct {
	// 世界场景 pass 的上一帧统计
	Scene PassStats
	// 光照 pass 的上一帧统计
	Lighting PassStats
	// 自发光 pass 的上一帧统计
	Emissive PassStats
	// Bloom pass 的上一帧统计
	Bloom PassStats
	// 合成 pass 的上一帧统计
	Composite PassStats
	// UI pass 的上一帧统计
	UI PassStats
}

// 中间渲染纹理调试信息
//
// 当前只暴露句柄、尺寸和调试名称，不转移纹理生命周期所有权
type TextureDebugInfo struct {
	// 调试显示名称
	Name string
	// OpenGL 纹理句柄
	ID uint32
	// 纹理像素尺寸
	Size mgl32.Vec2
	// 纹理来源路径，运行时生成纹理为空
	Path string
}

// 渲染器中间纹理调试入口
//
// 当前覆盖阶段 8 的核心离屏纹理，缺失或关闭的纹理会返回零值
type DebugTextures struct {
	// 世界场景颜色中间纹理
	SceneColor TextureDebugInfo
	// 光照颜色中间纹理
	LightColor TextureDebugInfo
	// 自发光颜色中间纹理
	EmissiveColor TextureDebugInfo
	// Bloom 合成输入纹理
	BloomColor TextureDebugInfo
}

// spriteBatch 当前队列的提交规模
//
// 当前只在 pass flush 前临时读取，不跨帧保存
type spriteBatchStats struct {
	// 当前批处理队列预计提交的 draw call 数量
	drawCalls int
	// 当前批处理队列包含的精灵或 quad 数量
	sprites int
	// 当前批处理队列包含的顶点数量
	vertices int
	// 当前批处理队列包含的索引数量
	indices int
}

// 将批处理队列统计转换成 pass 统计
func passStatsFromBatch(enabled bool, stats spriteBatchStats) PassStats {
	return PassStats{
		Enabled:   enabled,
		DrawCalls: stats.drawCalls,
		Sprites:   stats.sprites,
		Vertices:  stats.vertices,
		Indices:   stats.indices,
	}
}

// 将纹理转换成调试信息
func textureDebugInfo(name string, texture *Texture) TextureDebugInfo {
	if texture == nil {
		return TextureDebugInfo{Name: name}
	}
	return TextureDebugInfo{
		Name: name,
		ID:   texture.ID(),
		Size: texture.Size(),
		Path: texture.path,
	}
}
