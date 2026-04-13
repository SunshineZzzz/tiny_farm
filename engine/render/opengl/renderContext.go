package opengl

import (
	"errors"
	"fmt"
	"log/slog"

	gl "tiny_farm/engine/utils/opengl"

	"github.com/SunshineZzzz/purego-sdl3/sdl"
)

// 封装 SDL OpenGL 属性配置、上下文创建和销毁
//
// 当前把 SDL OpenGL 设置和生命周期集中在这里，GLRenderer 只处理渲染入口
type renderContext struct {
	// 窗口指针
	window *sdl.Window
	// 图形上下文指针
	sdlGLContext sdl.GLContext
	// 当前线程 OpenGL 函数调用入口
	glContext gl.Context
	// 渲染初始化参数
	params *rendererInitParams
}

// 创建 renderContext 实例
//
// 调用方必须已经锁定 OS 线程，SDL_GL_CreateContext 会把上下文设为当前上下文
// 函数加载依赖当前上下文提供 OpenGL 函数地址
func newRenderContext(window *sdl.Window, paramsJsonPath string) (*renderContext, error) {
	rc := &renderContext{}
	if err := rc.init(window, paramsJsonPath); err != nil {
		return nil, err
	}
	return rc, nil
}

// 初始化渲染上下文
func (rc *renderContext) init(window *sdl.Window, paramsJsonPath string) error {
	rc.clean()

	if window == nil {
		return errors.New("window is nil")
	}
	rc.window = window

	// 加载渲染配置文件
	if paramsJsonPath != "" {
		params, err := loadConfigFromFile(paramsJsonPath)
		if err != nil {
			return err
		}
		rc.params = params
	}

	// 创建 GL 上下文之前需要先设置属性
	if err := rc.setAttributes(); err != nil {
		return err
	}

	// 上下文创建成功后立即加载 OpenGL 函数地址
	if err := rc.createContext(); err != nil {
		return err
	}

	// 设置交换间隔
	if rc.params.SwapInterval != swapIntervalDontCare {
		sdl.GLSetSwapInterval(rc.params.SwapInterval)
	}

	return nil
}

// 设置 SDL OpenGL 上下文属性
func (rc *renderContext) setAttributes() error {
	requireAttr := func(attr sdl.GLAttr, value int32) bool {
		if !sdl.GLSetAttribute(attr, value) {
			return false
		}
		return true
	}

	doubleBuffer := int32(1)
	if !rc.params.DoubleBuffer {
		doubleBuffer = 0
	}
	// 单独设置每个属性，便于返回 SDL 聚合的错误信息
	if !requireAttr(sdl.GLContextMajorVersion, rc.params.GLMajorVersion) ||
		!requireAttr(sdl.GLContextMinorVersion, rc.params.GLMinorVersion) ||
		!requireAttr(sdl.GLContextProfileMask, int32(rc.params.ProfileMask)) ||
		!requireAttr(sdl.GLContextFlags, int32(rc.params.ContextFlags)) ||
		!requireAttr(sdl.GLDoubleBuffer, doubleBuffer) ||
		!requireAttr(sdl.GLDepthSize, rc.params.DepthBits) ||
		!requireAttr(sdl.GLStencilSize, rc.params.StencilBits) ||
		!requireAttr(sdl.GLMultisampleBuffers, rc.params.MultiSampleBuffers) ||
		!requireAttr(sdl.GLMultisampleSamples, rc.params.MultiSampleSamples) ||
		!requireAttr(sdl.GLFramebufferSRGBCapable, rc.params.FramebufferSRGBCapable) {
		return fmt.Errorf("%s", sdl.GetError())
	}

	return nil
}

// 创建 SDL OpenGL 上下文
func (rc *renderContext) createContext() error {
	context := sdl.GLCreateContext(rc.window)
	if context == nil {
		rc.clean()
		return fmt.Errorf("%s", sdl.GetError())
	}
	rc.sdlGLContext = context

	if !sdl.GLMakeCurrent(rc.window, context) {
		rc.clean()
		return fmt.Errorf("%s", sdl.GetError())
	}

	// 这一步不是“再创建一个 OpenGL context”。它只是创建一个 Go 侧的函数入口表对象，类似 GLAD loader 的承载结构。
	glContext, err := gl.NewDefaultContext()
	if err != nil {
		rc.clean()
		return fmt.Errorf("%s", err)
	}

	// 等价于 gladLoadGLLoader(reinterpret_cast<GLADloadproc>(SDL_GL_GetProcAddress))
	if err := glContext.LoadFunctions(); err != nil {
		rc.clean()
		return fmt.Errorf("%s", err)
	}

	rc.glContext = glContext
	return nil
}

// 清理 renderContext 占用的资源
func (r *renderContext) clean() {
	if r.sdlGLContext != nil {
		if r.window != nil {
			if !sdl.GLMakeCurrent(r.window, nil) {
				slog.Warn("make current failed", slog.String("err", sdl.GetError()))
			}
		}
		if !sdl.GLDestroyContext(r.sdlGLContext) {
			slog.Warn("destroy opengl context failed", slog.String("err", sdl.GetError()))
		}
		r.sdlGLContext = nil
	}
	r.glContext = nil
	r.window = nil
	r.params = nil
}

// 交换窗口缓冲区
func (rc *renderContext) swapWindow() {
	if rc.window == nil {
		return
	}
	sdl.GLSwapWindow(rc.window)
}

// 设置交换间隔，1-表示启用垂直同步，0-表示立即交换
func (rc *renderContext) setSwapInterval(interval int32) error {
	if !sdl.GLSetSwapInterval(interval) {
		return fmt.Errorf("%s", sdl.GetError())
	}
	return nil
}

// 获取交换间隔
func (rc *renderContext) getSwapInterval(interval *int32) error {
	if !sdl.GL_GetSwapInterval(interval) {
		return fmt.Errorf("%s", sdl.GetError())
	}
	return nil
}
