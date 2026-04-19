package opengl

import (
	"errors"
	"log/slog"

	"tiny_farm/engine/utils"
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

	// 阶段 2 使用的纯色矩形着色器
	rectShader *shaderProgram
	// 纯色矩形顶点数组对象
	rectVAO uint32
	// 纯色矩形顶点缓冲对象
	rectVBO uint32
	// 纯色矩形索引缓冲对象
	rectEBO uint32
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
func (gr *GLRenderer) DrawRect(rect mgl32.Vec4, color mgl32.Vec4) {
	// 检查参数是否有效
	if gr == nil || gr.renderCtx == nil || gr.renderCtx.glContext == nil || gr.rectShader == nil {
		return
	}

	// X = 左上角 x，Y = 左上角 y，Z = 宽度 width，W = 高度 height
	if rect.Z() <= 0 || rect.W() <= 0 {
		return
	}

	// 生成顶点数据
	vertices := []float32{
		rect.X(), rect.Y(), color.X(), color.Y(), color.Z(), color.W(),
		rect.X() + rect.Z(), rect.Y(), color.X(), color.Y(), color.Z(), color.W(),
		rect.X() + rect.Z(), rect.Y() + rect.W(), color.X(), color.Y(), color.Z(), color.W(),
		rect.X(), rect.Y() + rect.W(), color.X(), color.Y(), color.Z(), color.W(),
	}
	// 生成索引数据
	indices := []uint32{0, 1, 2, 2, 3, 0}

	glCtx := gr.renderCtx.glContext
	// 确保矩形画到窗口默认framebuffer上
	glCtx.BindFramebuffer(gl.FRAMEBUFFER, 0)
	// 使用纯色矩形着色器
	gr.rectShader.use()
	// 设置视图投影矩阵
	/*
	* |  2/w    0      0     -1 |
	* |   0   -2/h     0      1 |
	* |   0     0     -1      0 |
	* |   0     0      0      1 |
	*
	*  2*y/h - 1
	*
	* 对一个点，p = (x, y, z, 1)，乘完以后，
	* clipX =  2*x/w - 1
	* clipY = -2*y/h + 1， y越大，clipY越小，从而实现了Y轴向下增长的坐标系
	* clipZ = -z
	* clipW = 1
	 */
	viewProj := mgl32.Ortho(0, gr.logicalSize.X(), gr.logicalSize.Y(), 0, -1, 1)
	glCtx.UniformMatrix4fv(gr.rectViewProjLocation, viewProj[:])

	// 绑定之前准备好的VAO，恢复这套顶点解释规则
	glCtx.BindVertexArray(gr.rectVAO)

	// 绑定VBO，上传顶点数据
	glCtx.BindBuffer(gl.ARRAY_BUFFER, gr.rectVBO)
	glCtx.BufferSubData(gl.ARRAY_BUFFER, 0, utils.Float32Bytes(vertices))

	// 绑定EBO，上传索引数据
	glCtx.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, gr.rectEBO)
	glCtx.BufferSubData(gl.ELEMENT_ARRAY_BUFFER, 0, utils.Uint32Bytes(indices))

	// 绘制矩形
	glCtx.DrawElements(gl.TRIANGLES, int32(len(indices)), gl.UNSIGNED_INT, 0)
	// 解绑VAO
	glCtx.BindVertexArray(0)
	// 解绑着色器程序
	glCtx.UseProgram(0)
}

// 交换窗口前后缓冲，提交本帧画面
func (gr *GLRenderer) Present() {
	if gr == nil || gr.renderCtx == nil {
		return
	}

	gr.renderCtx.swapWindow()
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

// 初始化阶段 2 的最小矩形绘制资源
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

	// 创建VAO，VBO，EBO
	gr.rectVAO = glCtx.CreateVertexArray()
	gr.rectVBO = glCtx.CreateBuffer()
	gr.rectEBO = glCtx.CreateBuffer()
	if gr.rectVAO == 0 || gr.rectVBO == 0 || gr.rectEBO == 0 {
		gr.cleanRectRenderer()
		return errors.New("create rect renderer buffers failed")
	}

	// 每个顶点有 6 个 float32，x，y，r，g，b，a，每个 float32 是 4 字节
	const vertexSizeBytes = 6 * 4

	// 绑定VAO，接下来设置的顶点格式，都记录到这个 VAO 里
	glCtx.BindVertexArray(gr.rectVAO)
	// 绑定VBO，并且分配显存空间
	glCtx.BindBuffer(gl.ARRAY_BUFFER, gr.rectVBO)
	// 矩形有4个顶点
	glCtx.BufferInit(gl.ARRAY_BUFFER, 4*vertexSizeBytes, gl.DYNAMIC_DRAW)

	// 绑定EBO，并且分配显存空间
	glCtx.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, gr.rectEBO)
	// 矩形有6个索引(uint32)
	glCtx.BufferInit(gl.ELEMENT_ARRAY_BUFFER, 6*4, gl.DYNAMIC_DRAW)

	// 设置位置描述信息
	// 0 - location = 0
	// 2 - 这个属性有 2 个 float：x, y
	// gl.FLOAT - 类型是 float
	// false - 不归一化
	// vertexSizeBytes - 每个顶点间隔 24 字节
	// 0 - 从每个顶点的第 0 字节开始读
	glCtx.VertexAttribPointer(0, 2, gl.FLOAT, false, vertexSizeBytes, 0)
	glCtx.EnableVertexAttribArray(0)

	// 设置颜色描述信息
	// 1 - location = 1
	// 4 - 这个属性有 4 个 float：r, g, b, a
	// gl.FLOAT - 类型是 float
	// false - 不归一化
	// vertexSizeBytes - 每个顶点间隔 24 字节
	// 2*4 - 从每个顶点的第 8 字节开始读
	glCtx.VertexAttribPointer(1, 4, gl.FLOAT, false, vertexSizeBytes, 2*4)
	glCtx.EnableVertexAttribArray(1)

	// 解除VAO绑定，初始化完成
	glCtx.BindVertexArray(0)
	return nil
}

// 释放阶段 2 的矩形绘制资源
func (gr *GLRenderer) cleanRectRenderer() {
	if gr == nil || gr.renderCtx == nil || gr.renderCtx.glContext == nil {
		return
	}

	ctx := gr.renderCtx.glContext
	if gr.rectEBO != 0 {
		ctx.DeleteBuffer(gr.rectEBO)
		gr.rectEBO = 0
	}
	if gr.rectVBO != 0 {
		ctx.DeleteBuffer(gr.rectVBO)
		gr.rectVBO = 0
	}
	if gr.rectVAO != 0 {
		ctx.DeleteVertexArray(gr.rectVAO)
		gr.rectVAO = 0
	}
	if gr.rectShader != nil {
		gr.rectShader.clean()
		gr.rectShader = nil
	}
	gr.rectViewProjLocation = -1
}
