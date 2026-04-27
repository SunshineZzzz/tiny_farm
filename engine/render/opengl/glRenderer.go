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
	// 统一管理内置 shader program
	shaderLibrary *shaderLibrary
	// 场景离屏渲染目标
	scenePass *scenePass
	// 默认帧缓冲合成输出
	compositePass *compositePass
	// 默认帧缓冲 UI 输出
	uiPass *uiPass
	// 游戏逻辑窗口大小
	logicalSize mgl32.Vec2
	// 默认帧缓冲清屏颜色
	clearColor mgl32.Vec4
}

// 创建 GLRenderer 实例
func NewGLRenderer(window *sdl.Window, logicalSize mgl32.Vec2, paramsJsonPath string) (*GLRenderer, error) {
	gr := &GLRenderer{}
	if err := gr.init(window, logicalSize, paramsJsonPath); err != nil {
		gr.Close()
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

	shaderLibrary, err := newShaderLibrary(rc.glContext)
	if err != nil {
		return err
	}
	gr.shaderLibrary = shaderLibrary

	sceneShader, err := gr.shaderLibrary.get(shaderSceneSprite)
	if err != nil {
		return err
	}
	scenePass, err := newScenePass(rc.glContext, logicalSize, sceneShader)
	if err != nil {
		return err
	}
	gr.scenePass = scenePass

	compositeShader, err := gr.shaderLibrary.get(shaderComposite)
	if err != nil {
		return err
	}
	compositePass, err := newCompositePass(rc.glContext, compositeShader)
	if err != nil {
		return err
	}
	gr.compositePass = compositePass

	uiShader, err := gr.shaderLibrary.get(shaderUI)
	if err != nil {
		return err
	}
	uiPass, err := newUIPass(rc.glContext, uiShader)
	if err != nil {
		return err
	}
	gr.uiPass = uiPass

	gr.initBlendState()

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

// 初始化当前阶段共用的临时混合状态
//
// 现在 ScenePass、CompositePass 和 UIPass 都沿用普通 alpha blend，先放在 GLRenderer 统一设置
// 后续 Lighting、Emissive、Bloom 接入后，每个 pass 应该按自己的绘制语义显式设置 OpenGL 状态
func (gr *GLRenderer) initBlendState() {
	if gr == nil || gr.renderCtx == nil || gr.renderCtx.glContext == nil {
		return
	}

	glCtx := gr.renderCtx.glContext
	glCtx.Enable(gl.BLEND)
	glCtx.BlendFuncSeparate(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA, gl.ONE, gl.ONE_MINUS_SRC_ALPHA)
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
// 当前阶段先清默认 framebuffer 的黑边区域，再切到场景 FBO 清理场景内容
func (gr *GLRenderer) Clear() {
	if gr == nil || gr.renderCtx == nil || gr.renderCtx.glContext == nil {
		return
	}

	glCtx := gr.renderCtx.glContext
	glCtx.BindFramebuffer(gl.FRAMEBUFFER, 0)
	glCtx.ClearColor(0.0, 0.0, 0.0, 1.0)
	glCtx.Clear(gl.COLOR_BUFFER_BIT)

	if gr.scenePass != nil {
		gr.scenePass.clear(gr.clearColor)
	}
}

// 绘制一个逻辑坐标系下的纯色矩形
func (gr *GLRenderer) DrawRect(rect mgl32.Vec4, color mgl32.Vec4) error {
	// 检查参数是否有效
	if gr == nil || gr.scenePass == nil {
		return errors.New("gl renderer or scene pass is nil")
	}

	// X = 左上角 x，Y = 左上角 y，Z = 宽度 width，W = 高度 height
	if rect.Z() <= 0 || rect.W() <= 0 {
		return errors.New("rect width or height is invalid")
	}

	return gr.scenePass.queueRect(rect, color)
}

// 绘制一个逻辑坐标系下的贴图矩形
//
// uvRect 按左上原点语义传入，(0,0) 表示纹理左上，(1,1) 表示纹理右下
func (gr *GLRenderer) DrawTexture(texture *Texture, dstRect mgl32.Vec4, uvRect mgl32.Vec4) error {
	if gr == nil || gr.scenePass == nil {
		return errors.New("gl renderer or scene pass is nil")
	}
	if dstRect.Z() <= 0 || dstRect.W() <= 0 {
		return errors.New("dst rect width or height is invalid")
	}

	return gr.scenePass.queueTexture(texture, dstRect, uvRect)
}

// 绘制一个逻辑坐标系下的贴图源矩形
func (gr *GLRenderer) DrawTextureSourceRect(texture *Texture, dstRect mgl32.Vec4, srcRect mgl32.Vec4) error {
	uvRect, err := textureSourceRectUV(texture, srcRect)
	if err != nil {
		return err
	}
	return gr.DrawTexture(texture, dstRect, uvRect)
}

// 绘制一个 UI 逻辑坐标系下的纯色矩形
func (gr *GLRenderer) DrawUIRect(rect mgl32.Vec4, color mgl32.Vec4) error {
	if gr == nil || gr.uiPass == nil {
		return errors.New("gl renderer or ui pass is nil")
	}
	if rect.Z() <= 0 || rect.W() <= 0 {
		return errors.New("ui rect width or height is invalid")
	}
	return gr.uiPass.queueRect(rect, color)
}

// 绘制一个 UI 逻辑坐标系下的贴图矩形
func (gr *GLRenderer) DrawUITexture(texture *Texture, dstRect mgl32.Vec4, uvRect mgl32.Vec4) error {
	if gr == nil || gr.uiPass == nil {
		return errors.New("gl renderer or ui pass is nil")
	}
	if dstRect.Z() <= 0 || dstRect.W() <= 0 {
		return errors.New("ui dst rect width or height is invalid")
	}
	return gr.uiPass.queueTexture(texture, dstRect, uvRect)
}

// 绘制一个 UI 逻辑坐标系下的贴图源矩形
func (gr *GLRenderer) DrawUITextureSourceRect(texture *Texture, dstRect mgl32.Vec4, srcRect mgl32.Vec4) error {
	uvRect, err := textureSourceRectUV(texture, srcRect)
	if err != nil {
		return err
	}
	return gr.DrawUITexture(texture, dstRect, uvRect)
}

// 从图像文件创建可绘制纹理
func (gr *GLRenderer) LoadTexture(path string) (*Texture, error) {
	if gr == nil || gr.renderCtx == nil || gr.renderCtx.glContext == nil {
		return nil, errors.New("gl renderer context is nil")
	}

	return newTexture(gr.renderCtx.glContext, path)
}

// 交换窗口前后缓冲，提交本帧画面
func (gr *GLRenderer) Present() error {
	if gr == nil || gr.renderCtx == nil {
		return errors.New("gl renderer or render context is nil")
	}

	if err := gr.flushScenePass(); err != nil {
		return err
	}
	if err := gr.flushCompositePass(); err != nil {
		return err
	}
	if err := gr.flushUIPass(); err != nil {
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
	if gr.scenePass != nil {
		gr.scenePass.clean()
		gr.scenePass = nil
	}
	if gr.compositePass != nil {
		gr.compositePass.clean()
		gr.compositePass = nil
	}
	if gr.uiPass != nil {
		gr.uiPass.clean()
		gr.uiPass = nil
	}
	if gr.shaderLibrary != nil {
		gr.shaderLibrary.clean()
		gr.shaderLibrary = nil
	}
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

// 提交场景批处理到 logical size FBO
func (gr *GLRenderer) flushScenePass() error {
	if gr == nil || gr.scenePass == nil {
		return nil
	}

	return gr.scenePass.render()
}

// 将场景 FBO 输出交给最终合成 pass
func (gr *GLRenderer) flushCompositePass() error {
	if gr == nil || gr.scenePass == nil || gr.scenePass.texture() == nil || gr.compositePass == nil || gr.viewportManager == nil {
		return nil
	}

	// 确保 letterbox viewport 是最新的
	if gr.viewportManager != nil && gr.viewportManager.dirty {
		gr.viewportManager.update()
	}

	return gr.compositePass.render(gr.viewportManager.viewport, compositePassInput{
		sceneColor: gr.scenePass.texture(),
	})
}

// 将 UI 批处理输出到默认 framebuffer 的 letterbox viewport
func (gr *GLRenderer) flushUIPass() error {
	if gr == nil || gr.uiPass == nil || gr.viewportManager == nil {
		return nil
	}

	// 确保 letterbox viewport 是最新的
	if gr.viewportManager != nil && gr.viewportManager.dirty {
		gr.viewportManager.update()
	}

	return gr.uiPass.render(gr.viewportManager.viewport, gr.logicalSize)
}
