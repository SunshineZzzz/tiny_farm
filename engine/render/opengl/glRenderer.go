package opengl

import (
	"github.com/SunshineZzzz/purego-sdl3/sdl"
	"github.com/go-gl/mathgl/mgl32"
)

// 是 OpenGL 渲染器
type GLRenderer struct {
	// 管理 SDL OpenGL 上下文
	renderCtx *renderContext
	// 游戏逻辑窗口大小
	logicalSize mgl32.Vec2
}

// 创建 GLRenderer 实例
func NewGLRenderer(window *sdl.Window, logicalSize mgl32.Vec2, paramsJsonPath string) (*GLRenderer, error) {
	rc, err := newRenderContext(window, paramsJsonPath)
	if err != nil {
		return nil, err
	}
	return &GLRenderer{
		renderCtx:   rc,
		logicalSize: logicalSize,
	}, nil
}

// 设置垂直同步
func (gr *GLRenderer) SetVSyncEnabled(enabled bool) {
	if gr.renderCtx == nil {
		return
	}
	interval := int32(1)
	if !enabled {
		interval = 0
	}
	gr.renderCtx.setSwapInterval(interval)
}

// 关闭渲染器并释放上下文
func (gr *GLRenderer) Close() {
	if gr.renderCtx != nil {
		gr.renderCtx.clean()
		gr.renderCtx = nil
	}
}
