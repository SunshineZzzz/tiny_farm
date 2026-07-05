package opengl

import (
	"errors"

	emath "tiny_farm/engine/utils/math"
	gl "tiny_farm/engine/utils/opengl"

	"github.com/go-gl/mathgl/mgl32"
)

// 管理窗口像素尺寸与逻辑渲染尺寸之间的转换
//
// 窗口或逻辑尺寸变化后重新计算 letterbox 视口
type viewportManager struct {
	// 当前线程 OpenGL 函数调用入口
	glCtx gl.Context
	// 当前视口矩形
	viewport emath.Rect
	// SDL_GetWindowSizeInPixels 返回的窗口像素尺寸
	windowSize mgl32.Vec2
	// 固定逻辑渲染尺寸
	logicalSize mgl32.Vec2
	// 是否需要重新计算视口
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

	// windowSize 和 glViewport 均使用像素单位，高 DPI 下可能大于窗口逻辑尺寸
	metrics := emath.ComputeLetterboxMetrics(vm.windowSize, vm.logicalSize)
	vm.viewport = metrics.Viewport

	// OpenGL 视口使用窗口像素坐标
	vm.glCtx.Viewport(
		int32(vm.viewport.Position.X()),
		int32(vm.viewport.Position.Y()),
		int32(vm.viewport.Size.X()),
		int32(vm.viewport.Size.Y()),
	)
	vm.dirty = false
}
