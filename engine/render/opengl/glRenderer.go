package opengl

import (
	"errors"
	"log/slog"

	gl "tiny_farm/engine/utils/opengl"

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
	// 默认帧缓冲清屏颜色
	clearColor mgl32.Vec4

	// 纯色矩形着色器
	rectShader *shaderProgram
	// 纯色矩形批处理器
	rectBatch *spriteBatch
	// 纯色矩形视图投影 uniform 位置
	rectViewProjLocation int32
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
	gr.clearColor = mgl32.Vec4{0.2, 0.3, 0.3, 1.0}
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

	if err := gr.initRectRenderer(); err != nil {
		return err
	}

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
	vm, err := newViewportManager(rc.glContext, windowSize, logicalSize)
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

// 设置默认帧缓冲清屏颜色
func (gr *GLRenderer) SetClearColor(color mgl32.Vec4) {
	if gr == nil {
		return
	}

	gr.clearColor = color
}

// 清空当前帧的默认帧缓冲
//
// 当前阶段还没有离屏渲染目标，只负责把默认帧缓冲清理到已知颜色
func (gr *GLRenderer) Clear() {
	if gr == nil || gr.renderCtx == nil || gr.renderCtx.glContext == nil {
		return
	}

	if gr.viewportManager != nil && gr.viewportManager.dirty {
		gr.viewportManager.update()
	}

	glCtx := gr.renderCtx.glContext
	// 绑定默认 framebuffer
	glCtx.BindFramebuffer(gl.FRAMEBUFFER, 0)
	// 设置清屏颜色
	glCtx.ClearColor(gr.clearColor.X(), gr.clearColor.Y(), gr.clearColor.Z(), gr.clearColor.W())
	// 清空窗口颜色缓冲
	glCtx.Clear(gl.COLOR_BUFFER_BIT)
}

// 绘制一个逻辑坐标系下的纯色矩形
func (gr *GLRenderer) DrawRect(rect mgl32.Vec4, color mgl32.Vec4) error {
	// 检查参数是否有效
	if gr == nil || gr.rectBatch == nil {
		return errors.New("gl renderer or rect batch is nil")
	}

	// X = 左上角 x，Y = 左上角 y，Z = 宽度 width，W = 高度 height
	if rect.Z() <= 0 || rect.W() <= 0 {
		return errors.New("rect width or height is invalid")
	}

	return gr.rectBatch.queueRect(rect, color)
}

// 交换窗口前后缓冲，提交本帧画面
func (gr *GLRenderer) Present() error {
	if gr == nil || gr.renderCtx == nil {
		return errors.New("gl renderer or render context is nil")
	}

	if err := gr.flushRectRenderer(); err != nil {
		return err
	}

	gr.renderCtx.swapWindow()

	return nil
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
	gr.cleanRectRenderer()
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

// 初始化纯色矩形批量绘制资源
func (gr *GLRenderer) initRectRenderer() error {
	if gr == nil || gr.renderCtx == nil || gr.renderCtx.glContext == nil {
		return errors.New("gl renderer context is nil")
	}

	// 矩形顶点着色器
	const rectVertexShaderSource = `
	#version 330 core
	layout(location = 0) in vec2 aPos;
	layout(location = 1) in vec4 aColor;

	uniform mat4 uViewProj;

	out vec4 vColor;

	void main() {
		vColor = aColor;
		gl_Position = uViewProj * vec4(aPos, 0.0, 1.0);
	}
	`

	// 矩形片段着色器
	const rectFragmentShaderSource = `
	#version 330 core
	in vec4 vColor;

	out vec4 FragColor;

	void main() {
		FragColor = vColor;
	}
	`

	// 创建着色器程序
	glCtx := gr.renderCtx.glContext
	shader, err := newShaderProgram(glCtx, rectVertexShaderSource, rectFragmentShaderSource)
	if err != nil {
		return err
	}
	gr.rectShader = shader

	// 获取着色器程序中的视图投影矩阵位置
	gr.rectViewProjLocation = shader.uniformLocation("uViewProj")

	batch, err := newSpriteBatch(glCtx, minSpriteBatchCapacity)
	if err != nil {
		gr.cleanRectRenderer()
		return err
	}
	gr.rectBatch = batch

	return nil
}

// 提交当前帧已入队的矩形
func (gr *GLRenderer) flushRectRenderer() error {
	if gr == nil || gr.renderCtx == nil || gr.renderCtx.glContext == nil || gr.rectShader == nil || gr.rectBatch == nil {
		return nil
	}

	glCtx := gr.renderCtx.glContext
	glCtx.BindFramebuffer(gl.FRAMEBUFFER, 0)
	gr.rectShader.use()
	/*
	* |  2/w    0      0     -1 |
	* |   0   -2/h     0      1 |
	* |   0     0     -1      0 |
	* |   0     0      0      1 |
	*
	* 对一个点，p = (x, y, z, 1)，乘完以后，
	* clipX =  2*x/w - 1
	* clipY = -2*y/h + 1，y越大，clipY越小，从而实现 Y 轴向下增长
	* clipZ = -z
	* clipW = 1
	 */
	viewProj := mgl32.Ortho(0, gr.logicalSize.X(), gr.logicalSize.Y(), 0, -1, 1)
	glCtx.UniformMatrix4fv(gr.rectViewProjLocation, viewProj[:])
	if err := gr.rectBatch.flush(); err != nil {
		glCtx.UseProgram(0)
		return err
	}
	glCtx.UseProgram(0)
	return nil
}

// 释放纯色矩形批量绘制资源
func (gr *GLRenderer) cleanRectRenderer() {
	if gr == nil || gr.renderCtx == nil || gr.renderCtx.glContext == nil {
		return
	}

	if gr.rectBatch != nil {
		gr.rectBatch.clean()
		gr.rectBatch = nil
	}

	if gr.rectShader != nil {
		gr.rectShader.clean()
		gr.rectShader = nil
	}

	gr.rectViewProjLocation = -1
}
