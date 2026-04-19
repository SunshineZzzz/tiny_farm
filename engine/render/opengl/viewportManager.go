package opengl

import (
	"errors"

	emath "tiny_farm/engine/utils/math"
	gl "tiny_farm/engine/utils/opengl"

	"github.com/go-gl/mathgl/mgl32"
)

/**
 * @file viewportManager.go
 * @brief 管理窗口尺寸与游戏逻辑渲染尺寸的转换
 *
 * 此类负责窗口实际尺寸与游戏请求的逻辑渲染尺寸之间的转换，自动计算信箱(letterbox)区域。
 * 当窗口或逻辑尺寸变化时，提供简单接口供渲染器查询当前视口矩形。
 */
type viewportManager struct {
	// 当前线程 OpenGL 函数调用入口
	glCtx gl.Context
	// 当前视口矩形
	viewport emath.Rect
	// window pixel size (SDL_GetWindowSizeInPixels / drawable size)
	windowSize mgl32.Vec2
	// logical render size (fixed render target size)
	logicalSize mgl32.Vec2
	// dirty flag
	dirty bool
}

// 初始化视口管理器
func newViewportManager(glCtx gl.Context, windowSize, logicalSize mgl32.Vec2) (*viewportManager, error) {
	if glCtx == nil {
		return nil, errors.New("gl context is nil")
	}
	vm := &viewportManager{}
	vm.glCtx = glCtx
	vm.setWindowSize(windowSize)
	vm.setLogicalSize(logicalSize)
	vm.update()
	return vm, nil
}

// 设置窗口尺寸
func (vm *viewportManager) setWindowSize(windowSize mgl32.Vec2) {
	vm.windowSize = emath.Mgl32Vec2Max(mgl32.Vec2{1.0, 1.0}, windowSize)
	vm.dirty = true
}

// 设置逻辑渲染尺寸
func (vm *viewportManager) setLogicalSize(logical_size mgl32.Vec2) {
	vm.logicalSize = emath.Mgl32Vec2Max(mgl32.Vec2{1.0, 1.0}, logical_size)
	vm.dirty = true
}

// 更新视口
func (vm *viewportManager) update() {
	if !vm.dirty || vm.glCtx == nil {
		return
	}

	// 注意：这里的 windowSize 是 drawable 的像素尺寸（高 DPI 下可能大于 SDL_GetWindowSize()）。
	// glViewport 的单位也是像素，因此整个 letterbox 计算与 viewport 应用都在“像素坐标系”中完成。
	metrics := emath.ComputeLetterboxMetrics(vm.windowSize, vm.logicalSize)
	vm.viewport = metrics.Viewport

	// 设置视口，用的是像素单位
	vm.glCtx.Viewport(
		int32(vm.viewport.Position.X()),
		int32(vm.viewport.Position.Y()),
		int32(vm.viewport.Size.X()),
		int32(vm.viewport.Size.Y()),
	)
	vm.dirty = false
}
