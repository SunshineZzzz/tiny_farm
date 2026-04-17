package opengl

import (
	"log/slog"

	"github.com/SunshineZzzz/purego-sdl3/sdl"
	"github.com/go-gl/mathgl/mgl32"
)

// OpenGL 渲染器
//
// 当前只持有 SDL OpenGL 上下文和游戏逻辑尺寸，具体绘制入口后续再补
type GLRenderer struct {
	// 管理 SDL OpenGL 上下文
	renderCtx *renderContext
	// 视口管理器
	viewportManager *viewportManager
	// 游戏逻辑窗口大小
	logicalSize mgl32.Vec2
}

// 创建 GLRenderer 实例
func NewGLRenderer(window *sdl.Window, logicalSize mgl32.Vec2, paramsJsonPath string) (*GLRenderer, error) {
	gr := &GLRenderer{}
	if err := gr.init(window, logicalSize, paramsJsonPath); err != nil {
		return nil, err
	}
	return gr, nil
}

// 初始化渲染器
func (gr *GLRenderer) init(window *sdl.Window, logicalSize mgl32.Vec2, paramsJsonPath string) error {
	gr.logicalSize = logicalSize
	rc, err := newRenderContext(window, paramsJsonPath)
	if err != nil {
		return err
	}
	gr.renderCtx = rc

	vm, err := gr.initViewportManager(rc, logicalSize)
	if err != nil {
		return err
	}
	gr.viewportManager = vm

	slog.Debug("GLRenderer init success")
	return nil
}

// 初始化视口管理器
func (gr *GLRenderer) initViewportManager(rc *renderContext, logicalSize mgl32.Vec2) (*viewportManager, error) {
	// 获取窗口实际像素尺寸（高DPI下可能和Config的窗口大小不一致），未来任何窗口变动，都会通过 onResize() 函数更新视口
	w, h := int32(0), int32(0)
	sdl.GetWindowSizeInPixels(rc.window, &w, &h)
	windowSize := mgl32.Vec2{float32(w), float32(h)}

	// ViewportManager管理窗口大小。其中逻辑分辨率会自动计算带信箱效果的视口（letterboxed viewport）。
	vm, err := newViewportManager(rc, windowSize, logicalSize)
	if err != nil {
		return nil, err
	}
	return vm, nil
}

// 返回游戏逻辑坐标系尺寸
func (gr *GLRenderer) LogicalSize() mgl32.Vec2 {
	if gr == nil {
		return mgl32.Vec2{}
	}

	return gr.logicalSize
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

// 重置渲染器视口
func (gr *GLRenderer) Resize(width, height int32) {
	// 仅更新视口管理器（letterbox），离屏缓冲保持逻辑分辨率
	gr.viewportManager.setWindowSize(mgl32.Vec2{float32(width), float32(height)})
	gr.viewportManager.update()
}
