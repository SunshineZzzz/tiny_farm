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
	// 世界光照离屏渲染目标
	lightingPass *lightingPass
	// 世界自发光离屏渲染目标
	emissivePass *emissivePass
	// 世界自发光辉光后处理目标
	bloomPass *bloomPass
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

	lightingShader, err := gr.shaderLibrary.get(shaderLight)
	if err != nil {
		return err
	}
	lightingPass, err := newLightingPass(rc.glContext, logicalSize, lightingShader)
	if err != nil {
		return err
	}
	gr.lightingPass = lightingPass

	emissiveShader, err := gr.shaderLibrary.get(shaderEmissive)
	if err != nil {
		return err
	}
	emissivePass, err := newEmissivePass(rc.glContext, logicalSize, emissiveShader)
	if err != nil {
		return err
	}
	gr.emissivePass = emissivePass

	bloomShader, err := gr.shaderLibrary.get(shaderBloom)
	if err != nil {
		return err
	}
	bloomPass, err := newBloomPass(rc.glContext, logicalSize, bloomShader)
	if err != nil {
		return err
	}
	gr.bloomPass = bloomPass

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
// Lighting、Emissive、Bloom 等 pass 按自己的绘制语义显式设置 OpenGL 状态
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

// 设置世界层环境光颜色
func (gr *GLRenderer) SetAmbientLightColor(color mgl32.Vec4) {
	if gr == nil || gr.compositePass == nil {
		return
	}
	gr.compositePass.setAmbientColor(color)
}

// 设置是否启用世界层光照合成
func (gr *GLRenderer) SetLightingEnabled(enabled bool) {
	if gr == nil || gr.lightingPass == nil {
		return
	}
	gr.lightingPass.setEnabled(enabled)
}

// 设置是否启用世界层点光源
func (gr *GLRenderer) SetPointLightEnabled(enabled bool) {
	if gr == nil || gr.lightingPass == nil {
		return
	}
	gr.lightingPass.setPointLightEnabled(enabled)
}

// 设置是否启用世界层聚光灯
func (gr *GLRenderer) SetSpotLightEnabled(enabled bool) {
	if gr == nil || gr.lightingPass == nil {
		return
	}
	gr.lightingPass.setSpotLightEnabled(enabled)
}

// 设置是否启用世界层方向光
func (gr *GLRenderer) SetDirectionalLightEnabled(enabled bool) {
	if gr == nil || gr.lightingPass == nil {
		return
	}
	gr.lightingPass.setDirectionalLightEnabled(enabled)
}

// 设置是否启用世界层自发光合成
func (gr *GLRenderer) SetEmissiveEnabled(enabled bool) {
	if gr == nil || gr.emissivePass == nil {
		return
	}
	gr.emissivePass.setEnabled(enabled)
}

// 设置是否启用自发光 Bloom 后处理
func (gr *GLRenderer) SetBloomEnabled(enabled bool) {
	if gr == nil || gr.bloomPass == nil {
		return
	}
	gr.bloomPass.setEnabled(enabled)
}

// 设置 Bloom 降采样层数
func (gr *GLRenderer) SetBloomLevelCount(levelCount int) error {
	if gr == nil || gr.bloomPass == nil {
		return nil
	}
	return gr.bloomPass.setLevelCount(levelCount)
}

// 设置 Bloom 高斯模糊 Sigma
func (gr *GLRenderer) SetBloomSigma(sigma float32) {
	if gr == nil || gr.bloomPass == nil {
		return
	}
	gr.bloomPass.setSigma(sigma)
}

// 设置 Bloom 合成强度
func (gr *GLRenderer) SetBloomStrength(strength float32) {
	if gr == nil || gr.compositePass == nil {
		return
	}
	gr.compositePass.setBloomStrength(strength)
}

// 提交 logical 坐标系下的点光源
func (gr *GLRenderer) AddPointLight(position mgl32.Vec2, radius float32, color mgl32.Vec4, intensity float32) error {
	if gr == nil || gr.lightingPass == nil {
		return errors.New("gl renderer or lighting pass is nil")
	}
	return gr.lightingPass.queuePointLight(position, radius, color, intensity)
}

// 提交 logical 坐标系下的聚光源
func (gr *GLRenderer) AddSpotLight(position mgl32.Vec2, radius float32, direction mgl32.Vec2, color mgl32.Vec4, intensity float32, innerAngleDeg float32, outerAngleDeg float32) error {
	if gr == nil || gr.lightingPass == nil {
		return errors.New("gl renderer or lighting pass is nil")
	}
	return gr.lightingPass.queueSpotLight(position, radius, direction, color, intensity, innerAngleDeg, outerAngleDeg)
}

// 提交屏幕空间方向光
func (gr *GLRenderer) AddDirectionalLight(direction mgl32.Vec2, color mgl32.Vec4, intensity float32, offset float32, softness float32, middayBlend float32) error {
	if gr == nil || gr.lightingPass == nil {
		return errors.New("gl renderer or lighting pass is nil")
	}
	return gr.lightingPass.queueDirectionalLight(direction, color, intensity, offset, softness, middayBlend)
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
	if gr.lightingPass != nil {
		gr.lightingPass.clear()
	}
	if gr.emissivePass != nil {
		gr.emissivePass.clear()
	}
	if gr.bloomPass != nil {
		gr.bloomPass.clear()
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

// 绘制一条逻辑坐标系下的纯色线段
//
// start 和 end 表示线段中心线的起点和终点
// thickness 表示线段总宽度，实际绘制时会沿中心线法线方向向两侧各扩展 thickness/2
// color 表示线段颜色，使用 RGBA，取值范围为 0 到 1
//
// 当前实现不使用 OpenGL 线宽，而是把线段展开成四边形后加入场景批处理
func (gr *GLRenderer) DrawLine(start, end mgl32.Vec2, thickness float32, color mgl32.Vec4) error {
	if gr == nil || gr.scenePass == nil {
		return errors.New("gl renderer or scene pass is nil")
	}
	if thickness <= 0.0 {
		return errors.New("line thickness is invalid")
	}

	// 计算线段方向向量
	delta := end.Sub(start)
	// 计算线段长度
	length := delta.Len()
	// 计算线段半宽度
	halfThickness := thickness / 2.0

	// 如果线段长度太短了，直接绘制一个矩形
	if length <= 0.00001 {
		return gr.DrawRect(mgl32.Vec4{
			start.X() - halfThickness,
			start.Y() - halfThickness,
			thickness,
			thickness,
		}, color)
	}

	// 垂直方向 = 把 delta 转 90 度
	// 单位法线 = 垂直方向 / length
	// normal = 单位法线 * halfThickness
	normal := mgl32.Vec2{-delta.Y(), delta.X()}.Mul(halfThickness / length)
	// 构造四边形
	points := [4]mgl32.Vec2{
		start.Add(normal),
		end.Add(normal),
		end.Sub(normal),
		start.Sub(normal),
	}
	return gr.scenePass.queueQuad(points, color)
}

// 绘制一个逻辑坐标系下的贴图矩形
//
// uvRect 按左上原点语义传入，(0,0) 表示纹理左上，(1,1) 表示纹理右下
func (gr *GLRenderer) DrawTexture(texture *Texture, dstRect mgl32.Vec4, uvRect mgl32.Vec4) error {
	return gr.DrawTextureColor(texture, dstRect, uvRect, mgl32.Vec4{1.0, 1.0, 1.0, 1.0})
}

// 绘制一个带颜色调制的逻辑坐标系贴图矩形
func (gr *GLRenderer) DrawTextureColor(texture *Texture, dstRect mgl32.Vec4, uvRect mgl32.Vec4, color mgl32.Vec4) error {
	if gr == nil || gr.scenePass == nil {
		return errors.New("gl renderer or scene pass is nil")
	}
	if dstRect.Z() <= 0 || dstRect.W() <= 0 {
		return errors.New("dst rect width or height is invalid")
	}

	return gr.scenePass.queueTextureColor(texture, dstRect, uvRect, color)
}

// 绘制一个带颜色参数的逻辑坐标系贴图矩形
func (gr *GLRenderer) DrawTextureColorOptions(texture *Texture, dstRect mgl32.Vec4, uvRect mgl32.Vec4, color ColorOptions) error {
	if gr == nil || gr.scenePass == nil {
		return errors.New("gl renderer or scene pass is nil")
	}
	if dstRect.Z() <= 0 || dstRect.W() <= 0 {
		return errors.New("dst rect width or height is invalid")
	}

	return gr.scenePass.queueTextureColorOptions(texture, dstRect, uvRect, color)
}

// 绘制一个逻辑坐标系下的贴图源矩形
func (gr *GLRenderer) DrawTextureSourceRect(texture *Texture, dstRect mgl32.Vec4, srcRect mgl32.Vec4) error {
	uvRect, err := textureSourceRectUV(texture, srcRect)
	if err != nil {
		return err
	}
	return gr.DrawTexture(texture, dstRect, uvRect)
}

// 绘制一个逻辑坐标系下的自发光纯色矩形
func (gr *GLRenderer) DrawEmissiveRect(rect mgl32.Vec4, color mgl32.Vec4) error {
	if gr == nil || gr.emissivePass == nil {
		return errors.New("gl renderer or emissive pass is nil")
	}
	if rect.Z() <= 0 || rect.W() <= 0 {
		return errors.New("emissive rect width or height is invalid")
	}
	return gr.emissivePass.queueRect(rect, color)
}

// 绘制一个逻辑坐标系下的自发光贴图矩形
func (gr *GLRenderer) DrawEmissiveTexture(texture *Texture, dstRect mgl32.Vec4, uvRect mgl32.Vec4, color mgl32.Vec4) error {
	if gr == nil || gr.emissivePass == nil {
		return errors.New("gl renderer or emissive pass is nil")
	}
	if texture == nil || texture.id == 0 {
		return errors.New("texture is nil")
	}
	if dstRect.Z() <= 0 || dstRect.W() <= 0 {
		return errors.New("emissive dst rect width or height is invalid")
	}
	return gr.emissivePass.queueTexture(texture, dstRect, uvRect, color)
}

// 绘制一个逻辑坐标系下的自发光贴图源矩形
func (gr *GLRenderer) DrawEmissiveTextureSourceRect(texture *Texture, dstRect mgl32.Vec4, srcRect mgl32.Vec4, color mgl32.Vec4) error {
	uvRect, err := textureSourceRectUV(texture, srcRect)
	if err != nil {
		return err
	}
	return gr.DrawEmissiveTexture(texture, dstRect, uvRect, color)
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
	return gr.DrawUITextureColor(texture, dstRect, uvRect, mgl32.Vec4{1.0, 1.0, 1.0, 1.0})
}

// 绘制一个带颜色调制的 UI 逻辑坐标系贴图矩形
func (gr *GLRenderer) DrawUITextureColor(texture *Texture, dstRect mgl32.Vec4, uvRect mgl32.Vec4, color mgl32.Vec4) error {
	if gr == nil || gr.uiPass == nil {
		return errors.New("gl renderer or ui pass is nil")
	}
	if dstRect.Z() <= 0 || dstRect.W() <= 0 {
		return errors.New("ui dst rect width or height is invalid")
	}
	return gr.uiPass.queueTextureColor(texture, dstRect, uvRect, color)
}

// 绘制一个带颜色参数的 UI 逻辑坐标系贴图矩形
func (gr *GLRenderer) DrawUITextureColorOptions(texture *Texture, dstRect mgl32.Vec4, uvRect mgl32.Vec4, color ColorOptions) error {
	if gr == nil || gr.uiPass == nil {
		return errors.New("gl renderer or ui pass is nil")
	}
	if dstRect.Z() <= 0 || dstRect.W() <= 0 {
		return errors.New("ui dst rect width or height is invalid")
	}
	return gr.uiPass.queueTextureColorOptions(texture, dstRect, uvRect, color)
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

// 创建一张空白可绘制纹理
//
// 当前用于后续字体 atlas 这类运行时写入纹理
func (gr *GLRenderer) CreateEmptyTexture(width, height int32, filter TextureFilter) (*Texture, error) {
	if gr == nil || gr.renderCtx == nil || gr.renderCtx.glContext == nil {
		return nil, errors.New("gl renderer context is nil")
	}

	return newEmptyTexture(gr.renderCtx.glContext, width, height, filter)
}

// 交换窗口前后缓冲，提交本帧画面
func (gr *GLRenderer) Present() error {
	if gr == nil || gr.renderCtx == nil {
		return errors.New("gl renderer or render context is nil")
	}

	if err := gr.flushScenePass(); err != nil {
		return err
	}
	if err := gr.flushLightingPass(); err != nil {
		return err
	}
	if err := gr.flushEmissivePass(); err != nil {
		return err
	}
	if err := gr.flushBloomPass(); err != nil {
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

// 返回上一帧各 pass 的渲染统计
func (gr *GLRenderer) RenderStats() RenderStats {
	if gr == nil {
		return RenderStats{}
	}

	var stats RenderStats
	if gr.scenePass != nil {
		stats.Scene = gr.scenePass.renderStats()
	}
	if gr.lightingPass != nil {
		stats.Lighting = gr.lightingPass.renderStats()
	}
	if gr.emissivePass != nil {
		stats.Emissive = gr.emissivePass.renderStats()
	}
	if gr.bloomPass != nil {
		stats.Bloom = gr.bloomPass.renderStats()
	}
	if gr.compositePass != nil {
		stats.Composite = gr.compositePass.renderStats()
	}
	if gr.uiPass != nil {
		stats.UI = gr.uiPass.renderStats()
	}
	return stats
}

// 返回阶段 8 核心中间纹理的调试信息
func (gr *GLRenderer) DebugTextures() DebugTextures {
	if gr == nil {
		return DebugTextures{}
	}

	var textures DebugTextures
	if gr.scenePass != nil {
		textures.SceneColor = textureDebugInfo("scene_color", gr.scenePass.texture())
	}
	if gr.lightingPass != nil {
		textures.LightColor = textureDebugInfo("light_color", gr.lightingPass.colorTexture)
	}
	if gr.emissivePass != nil {
		textures.EmissiveColor = textureDebugInfo("emissive_color", gr.emissivePass.colorTexture)
	}
	if gr.bloomPass != nil && len(gr.bloomPass.levels) > 0 {
		textures.BloomColor = textureDebugInfo("bloom_color", gr.bloomPass.levels[0].pongTexture)
	}
	return textures
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
	if gr.lightingPass != nil {
		gr.lightingPass.clean()
		gr.lightingPass = nil
	}
	if gr.emissivePass != nil {
		gr.emissivePass.clean()
		gr.emissivePass = nil
	}
	if gr.bloomPass != nil {
		gr.bloomPass.clean()
		gr.bloomPass = nil
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

// 提交世界光照到 logical size FBO
func (gr *GLRenderer) flushLightingPass() error {
	if gr == nil || gr.lightingPass == nil {
		return nil
	}

	return gr.lightingPass.render()
}

// 提交世界自发光到 logical size FBO
func (gr *GLRenderer) flushEmissivePass() error {
	if gr == nil || gr.emissivePass == nil {
		return nil
	}

	return gr.emissivePass.render()
}

// 提交自发光纹理到 Bloom 后处理
func (gr *GLRenderer) flushBloomPass() error {
	if gr == nil || gr.bloomPass == nil {
		return nil
	}

	return gr.bloomPass.render(gr.emissiveTexture())
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
		sceneColor:    gr.scenePass.texture(),
		lightColor:    gr.lightingTexture(),
		emissiveColor: gr.emissiveTexture(),
		bloomColor:    gr.bloomTexture(),
	})
}

func (gr *GLRenderer) lightingTexture() *Texture {
	if gr == nil || gr.lightingPass == nil {
		return nil
	}
	return gr.lightingPass.texture()
}

func (gr *GLRenderer) emissiveTexture() *Texture {
	if gr == nil || gr.emissivePass == nil {
		return nil
	}
	return gr.emissivePass.texture()
}

func (gr *GLRenderer) bloomTexture() *Texture {
	if gr == nil || gr.bloomPass == nil {
		return nil
	}
	return gr.bloomPass.texture()
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
