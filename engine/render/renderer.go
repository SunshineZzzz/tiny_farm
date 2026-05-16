package render

import (
	"errors"

	"tiny_farm/engine/render/opengl"
	emath "tiny_farm/engine/utils/math"

	"github.com/SunshineZzzz/purego-sdl3/sdl"
	"github.com/go-gl/mathgl/mgl32"
)

// 渲染外观层
//
// 当前阶段继续通过 facade 隔离 game 层和 OpenGL 后端
// 同时把 camera、世界坐标转换、可见区域裁剪和像素对齐收口到这一层
type Renderer struct {
	// 后端渲染器
	backend *opengl.GLRenderer
	// 当前帧使用的相机
	currentCamera *Camera
	// 是否启用可见区域裁剪
	viewportClippingEnabled bool
	// 是否启用像素对齐
	pixelSnapEnabled bool
}

// 持有可绘制纹理句柄
//
// 当前只做最小包装，资源生命周期仍由创建它的 Renderer 管理
type Texture struct {
	// 后端纹理句柄
	backend *opengl.Texture
}

// 渲染器上一帧统计
type RenderStats = opengl.RenderStats

// 渲染器中间纹理调试入口
type DebugTextures = opengl.DebugTextures

// 点光源绘制参数
type PointLightOptions struct {
	// 光源颜色，RGB 表示颜色，Alpha 当前不参与计算
	Color mgl32.Vec4
	// 光源强度
	Intensity float32
}

// 聚光灯绘制参数
type SpotLightOptions struct {
	// 光源颜色，RGB 表示颜色，Alpha 当前不参与计算
	Color mgl32.Vec4
	// 光源强度
	Intensity float32
	// 内锥角度，单位为度
	InnerAngleDeg float32
	// 外锥角度，单位为度
	OuterAngleDeg float32
}

// 方向光绘制参数
type DirectionalLightOptions struct {
	// 光源颜色，RGB 表示颜色，Alpha 当前不参与计算
	Color mgl32.Vec4
	// 光源强度
	Intensity float32
	// 屏幕空间渐变中心偏移，范围为 0 到 1
	Offset float32
	// 屏幕空间渐变柔和范围，范围为 0 到 0.5
	Softness float32
	// 正午混合强度，范围为 0 到 1
	MiddayBlend float32
}

// 创建渲染器 facade
func NewRenderer(window *sdl.Window, logicalSize mgl32.Vec2, paramsJSONPath string) (*Renderer, error) {
	backend, err := opengl.NewGLRenderer(window, logicalSize, paramsJSONPath)
	if err != nil {
		return nil, err
	}
	return &Renderer{
		backend:                 backend,
		viewportClippingEnabled: true,
		pixelSnapEnabled:        true,
	}, nil
}

// 返回逻辑分辨率
func (r *Renderer) LogicalSize() mgl32.Vec2 {
	if r == nil || r.backend == nil {
		return mgl32.Vec2{}
	}
	return r.backend.LogicalSize()
}

// 设置清屏颜色
func (r *Renderer) SetClearColor(color mgl32.Vec4) {
	if r == nil || r.backend == nil {
		return
	}
	r.backend.SetClearColor(color)
}

// 设置世界层环境光颜色
func (r *Renderer) SetAmbientLightColor(color mgl32.Vec4) {
	if r == nil || r.backend == nil {
		return
	}
	r.backend.SetAmbientLightColor(color)
}

// 设置是否启用世界层光照合成
func (r *Renderer) SetLightingEnabled(enabled bool) {
	if r == nil || r.backend == nil {
		return
	}
	r.backend.SetLightingEnabled(enabled)
}

// 设置是否启用世界自发光合成
func (r *Renderer) SetEmissiveEnabled(enabled bool) {
	if r == nil || r.backend == nil {
		return
	}
	r.backend.SetEmissiveEnabled(enabled)
}

// 设置是否启用世界自发光 Bloom 后处理
func (r *Renderer) SetBloomEnabled(enabled bool) {
	if r == nil || r.backend == nil {
		return
	}
	r.backend.SetBloomEnabled(enabled)
}

// 设置世界自发光 Bloom 合成强度
func (r *Renderer) SetBloomStrength(strength float32) {
	if r == nil || r.backend == nil {
		return
	}
	r.backend.SetBloomStrength(strength)
}

// 提交 logical 坐标系下的点光源
func (r *Renderer) AddPointLight(position mgl32.Vec2, radius float32, options *PointLightOptions) error {
	if r == nil || r.backend == nil {
		return errors.New("renderer is nil")
	}

	resolved := resolvePointLightOptions(options)
	return r.backend.AddPointLight(position, radius, resolved.Color, resolved.Intensity)
}

// 提交 world 坐标系下的点光源
func (r *Renderer) AddWorldPointLight(position mgl32.Vec2, radius float32, options *PointLightOptions) error {
	if r == nil || r.backend == nil {
		return errors.New("renderer is nil")
	}

	logicalPosition := position
	logicalRadius := radius
	if r.currentCamera != nil {
		logicalPosition = r.currentCamera.WorldToLogical(position)
		logicalRadius = radius * r.currentCamera.Zoom()
	}
	return r.AddPointLight(logicalPosition, logicalRadius, options)
}

// 提交 logical 坐标系下的聚光灯
func (r *Renderer) AddSpotLight(position mgl32.Vec2, radius float32, direction mgl32.Vec2, options *SpotLightOptions) error {
	if r == nil || r.backend == nil {
		return errors.New("renderer is nil")
	}

	resolved := resolveSpotLightOptions(options)
	return r.backend.AddSpotLight(position, radius, direction, resolved.Color, resolved.Intensity, resolved.InnerAngleDeg, resolved.OuterAngleDeg)
}

// 提交 world 坐标系下的聚光灯
func (r *Renderer) AddWorldSpotLight(position mgl32.Vec2, radius float32, direction mgl32.Vec2, options *SpotLightOptions) error {
	if r == nil || r.backend == nil {
		return errors.New("renderer is nil")
	}

	logicalPosition := position
	logicalRadius := radius
	if r.currentCamera != nil {
		logicalPosition = r.currentCamera.WorldToLogical(position)
		logicalRadius = radius * r.currentCamera.Zoom()
	}
	return r.AddSpotLight(logicalPosition, logicalRadius, direction, options)
}

// 提交屏幕空间方向光
func (r *Renderer) AddDirectionalLight(direction mgl32.Vec2, options *DirectionalLightOptions) error {
	if r == nil || r.backend == nil {
		return errors.New("renderer is nil")
	}

	resolved := resolveDirectionalLightOptions(options)
	return r.backend.AddDirectionalLight(direction, resolved.Color, resolved.Intensity, resolved.Offset, resolved.Softness, resolved.MiddayBlend)
}

// 开始一帧世界渲染
func (r *Renderer) BeginFrame(camera *Camera) {
	if r == nil || r.backend == nil {
		return
	}
	r.currentCamera = camera
}

// 清空当前帧
func (r *Renderer) Clear() {
	if r == nil || r.backend == nil {
		return
	}
	r.backend.Clear()
}

// 绘制逻辑坐标系下的纯色矩形
func (r *Renderer) DrawRect(rect mgl32.Vec4, color mgl32.Vec4) error {
	if r == nil || r.backend == nil {
		return errors.New("renderer is nil")
	}
	return r.backend.DrawRect(rect, color)
}

// 绘制世界坐标系下的纯色矩形
func (r *Renderer) DrawWorldRect(rect mgl32.Vec4, color mgl32.Vec4) error {
	if r == nil || r.backend == nil {
		return errors.New("renderer is nil")
	}
	logicalRect, ok := r.worldRectToLogical(rect)
	if !ok {
		return nil
	}
	return r.backend.DrawRect(logicalRect, color)
}

// 绘制逻辑坐标系下的贴图矩形
//
// uvRect 按左上原点语义传入，(0,0) 表示纹理左上，(1,1) 表示纹理右下
func (r *Renderer) DrawTexture(texture *Texture, dstRect mgl32.Vec4, uvRect mgl32.Vec4) error {
	if r == nil || r.backend == nil {
		return errors.New("renderer is nil")
	}
	if texture == nil || texture.backend == nil {
		return errors.New("texture is nil")
	}
	return r.backend.DrawTexture(texture.backend, dstRect, uvRect)
}

// 绘制世界坐标系下的贴图矩形
func (r *Renderer) DrawWorldTexture(texture *Texture, dstRect mgl32.Vec4, uvRect mgl32.Vec4) error {
	if r == nil || r.backend == nil {
		return errors.New("renderer is nil")
	}
	if texture == nil || texture.backend == nil {
		return errors.New("texture is nil")
	}
	logicalRect, ok := r.worldRectToLogical(dstRect)
	if !ok {
		return nil
	}
	return r.backend.DrawTexture(texture.backend, logicalRect, uvRect)
}

// 绘制逻辑坐标系下的贴图源矩形
func (r *Renderer) DrawTextureSourceRect(texture *Texture, dstRect mgl32.Vec4, srcRect mgl32.Vec4) error {
	if r == nil || r.backend == nil {
		return errors.New("renderer is nil")
	}
	if texture == nil || texture.backend == nil {
		return errors.New("texture is nil")
	}
	return r.backend.DrawTextureSourceRect(texture.backend, dstRect, srcRect)
}

// 绘制 UI 逻辑坐标系下的纯色矩形
func (r *Renderer) DrawUIRect(rect mgl32.Vec4, color mgl32.Vec4) error {
	if r == nil || r.backend == nil {
		return errors.New("renderer is nil")
	}
	return r.backend.DrawUIRect(rect, color)
}

// 绘制 UI 逻辑坐标系下的贴图矩形
func (r *Renderer) DrawUITexture(texture *Texture, dstRect mgl32.Vec4, uvRect mgl32.Vec4) error {
	if r == nil || r.backend == nil {
		return errors.New("renderer is nil")
	}
	if texture == nil || texture.backend == nil {
		return errors.New("texture is nil")
	}
	return r.backend.DrawUITexture(texture.backend, dstRect, uvRect)
}

// 绘制 UI 逻辑坐标系下的贴图源矩形
func (r *Renderer) DrawUITextureSourceRect(texture *Texture, dstRect mgl32.Vec4, srcRect mgl32.Vec4) error {
	if r == nil || r.backend == nil {
		return errors.New("renderer is nil")
	}
	if texture == nil || texture.backend == nil {
		return errors.New("texture is nil")
	}
	return r.backend.DrawUITextureSourceRect(texture.backend, dstRect, srcRect)
}

// 绘制世界坐标系下的贴图源矩形
func (r *Renderer) DrawWorldTextureSourceRect(texture *Texture, dstRect mgl32.Vec4, srcRect mgl32.Vec4) error {
	if r == nil || r.backend == nil {
		return errors.New("renderer is nil")
	}
	if texture == nil || texture.backend == nil {
		return errors.New("texture is nil")
	}
	logicalRect, ok := r.worldRectToLogical(dstRect)
	if !ok {
		return nil
	}
	return r.backend.DrawTextureSourceRect(texture.backend, logicalRect, srcRect)
}

// 绘制逻辑坐标系下的自发光纯色矩形
func (r *Renderer) DrawEmissiveRect(rect mgl32.Vec4, color mgl32.Vec4) error {
	if r == nil || r.backend == nil {
		return errors.New("renderer is nil")
	}
	return r.backend.DrawEmissiveRect(rect, color)
}

// 绘制世界坐标系下的自发光纯色矩形
func (r *Renderer) DrawWorldEmissiveRect(rect mgl32.Vec4, color mgl32.Vec4) error {
	if r == nil || r.backend == nil {
		return errors.New("renderer is nil")
	}
	logicalRect, ok := r.worldRectToLogical(rect)
	if !ok {
		return nil
	}
	return r.backend.DrawEmissiveRect(logicalRect, color)
}

// 绘制逻辑坐标系下的自发光贴图矩形
func (r *Renderer) DrawEmissiveTexture(texture *Texture, dstRect mgl32.Vec4, uvRect mgl32.Vec4, color mgl32.Vec4) error {
	if r == nil || r.backend == nil {
		return errors.New("renderer is nil")
	}
	if texture == nil || texture.backend == nil {
		return errors.New("texture is nil")
	}
	return r.backend.DrawEmissiveTexture(texture.backend, dstRect, uvRect, color)
}

// 绘制世界坐标系下的自发光贴图矩形
func (r *Renderer) DrawWorldEmissiveTexture(texture *Texture, dstRect mgl32.Vec4, uvRect mgl32.Vec4, color mgl32.Vec4) error {
	if r == nil || r.backend == nil {
		return errors.New("renderer is nil")
	}
	if texture == nil || texture.backend == nil {
		return errors.New("texture is nil")
	}
	logicalRect, ok := r.worldRectToLogical(dstRect)
	if !ok {
		return nil
	}
	return r.backend.DrawEmissiveTexture(texture.backend, logicalRect, uvRect, color)
}

// 绘制逻辑坐标系下的自发光贴图源矩形
func (r *Renderer) DrawEmissiveTextureSourceRect(texture *Texture, dstRect mgl32.Vec4, srcRect mgl32.Vec4, color mgl32.Vec4) error {
	if r == nil || r.backend == nil {
		return errors.New("renderer is nil")
	}
	if texture == nil || texture.backend == nil {
		return errors.New("texture is nil")
	}
	return r.backend.DrawEmissiveTextureSourceRect(texture.backend, dstRect, srcRect, color)
}

// 绘制世界坐标系下的自发光贴图源矩形
func (r *Renderer) DrawWorldEmissiveTextureSourceRect(texture *Texture, dstRect mgl32.Vec4, srcRect mgl32.Vec4, color mgl32.Vec4) error {
	if r == nil || r.backend == nil {
		return errors.New("renderer is nil")
	}
	if texture == nil || texture.backend == nil {
		return errors.New("texture is nil")
	}
	logicalRect, ok := r.worldRectToLogical(dstRect)
	if !ok {
		return nil
	}
	return r.backend.DrawEmissiveTextureSourceRect(texture.backend, logicalRect, srcRect, color)
}

// 加载纹理
func (r *Renderer) LoadTexture(path string) (*Texture, error) {
	if r == nil || r.backend == nil {
		return nil, errors.New("renderer is nil")
	}
	texture, err := r.backend.LoadTexture(path)
	if err != nil {
		return nil, err
	}
	return &Texture{backend: texture}, nil
}

// 提交当前帧
func (r *Renderer) Present() error {
	if r == nil || r.backend == nil {
		return errors.New("renderer is nil")
	}
	return r.backend.Present()
}

// 返回上一帧各 pass 的渲染统计
func (r *Renderer) RenderStats() RenderStats {
	if r == nil || r.backend == nil {
		return RenderStats{}
	}
	return r.backend.RenderStats()
}

// 返回阶段 8 核心中间纹理的调试信息
func (r *Renderer) DebugTextures() DebugTextures {
	if r == nil || r.backend == nil {
		return DebugTextures{}
	}
	return r.backend.DebugTextures()
}

// 设置垂直同步
func (r *Renderer) SetVSyncEnabled(enabled bool) {
	if r == nil || r.backend == nil {
		return
	}
	r.backend.SetVSyncEnabled(enabled)
}

// 设置是否启用可见区域裁剪
func (r *Renderer) SetViewportClippingEnabled(enabled bool) {
	if r == nil {
		return
	}
	r.viewportClippingEnabled = enabled
}

// 返回当前是否启用可见区域裁剪
func (r *Renderer) IsViewportClippingEnabled() bool {
	if r == nil {
		return false
	}
	return r.viewportClippingEnabled
}

// 设置是否启用像素对齐
func (r *Renderer) SetPixelSnapEnabled(enabled bool) {
	if r == nil {
		return
	}
	r.pixelSnapEnabled = enabled
}

// 返回当前是否启用像素对齐
func (r *Renderer) IsPixelSnapEnabled() bool {
	if r == nil {
		return false
	}
	return r.pixelSnapEnabled
}

// 更新窗口尺寸
func (r *Renderer) Resize(width, height int32) {
	if r == nil || r.backend == nil {
		return
	}
	r.backend.Resize(width, height)
}

// 关闭渲染器
func (r *Renderer) Close() {
	if r == nil || r.backend == nil {
		return
	}
	r.backend.Close()
	r.backend = nil
	r.currentCamera = nil
}

// 释放纹理资源
func (t *Texture) Close() {
	if t == nil || t.backend == nil {
		return
	}
	t.backend.Close()
	t.backend = nil
}

// 返回当前相机可见的世界区域
func (r *Renderer) CurrentViewRect() (emath.Rect, bool) {
	if r == nil || r.currentCamera == nil {
		return emath.Rect{}, false
	}
	return r.currentCamera.ViewRect(), true
}

func (r *Renderer) worldRectToLogical(rect mgl32.Vec4) (mgl32.Vec4, bool) {
	if r == nil {
		return mgl32.Vec4{}, false
	}
	if rect.Z() <= 0.0 || rect.W() <= 0.0 {
		return mgl32.Vec4{}, false
	}
	if r.currentCamera == nil {
		return rect, true
	}
	if r.viewportClippingEnabled && r.shouldCullWorldRect(rect) {
		return mgl32.Vec4{}, false
	}
	return r.currentCamera.worldRectToLogical(rect, r.pixelSnapEnabled), true
}

func (r *Renderer) shouldCullWorldRect(rect mgl32.Vec4) bool {
	if r == nil || r.currentCamera == nil {
		return false
	}
	viewRect := r.currentCamera.ViewRect()
	rectMin := mgl32.Vec2{rect.X(), rect.Y()}
	rectMax := mgl32.Vec2{rect.X() + rect.Z(), rect.Y() + rect.W()}
	viewMin := viewRect.Position
	viewMax := viewRect.Position.Add(viewRect.Size)
	return rectMax.X() <= viewMin.X() ||
		rectMin.X() >= viewMax.X() ||
		rectMax.Y() <= viewMin.Y() ||
		rectMin.Y() >= viewMax.Y()
}

// 补全点光源绘制参数
// 当前允许调用方传nil，使用白色、强度为1的默认点光源
func resolvePointLightOptions(options *PointLightOptions) PointLightOptions {
	if options == nil {
		return PointLightOptions{
			Color:     mgl32.Vec4{1.0, 1.0, 1.0, 1.0},
			Intensity: 1.0,
		}
	}
	resolved := *options
	if resolved.Color.X() == 0.0 &&
		resolved.Color.Y() == 0.0 &&
		resolved.Color.Z() == 0.0 &&
		resolved.Color.W() == 0.0 {
		resolved.Color = mgl32.Vec4{1.0, 1.0, 1.0, 1.0}
	}
	if resolved.Intensity <= 0.0 {
		resolved.Intensity = 1.0
	}
	return resolved
}

// 补全聚光灯绘制参数
// 当前允许调用方传nil， 使用白色、强度为1的默认聚光灯角度
func resolveSpotLightOptions(options *SpotLightOptions) SpotLightOptions {
	if options == nil {
		return SpotLightOptions{
			Color:         mgl32.Vec4{1.0, 1.0, 1.0, 1.0},
			Intensity:     1.0,
			InnerAngleDeg: 20.0,
			OuterAngleDeg: 35.0,
		}
	}

	resolved := *options
	if resolved.Color.X() == 0.0 &&
		resolved.Color.Y() == 0.0 &&
		resolved.Color.Z() == 0.0 &&
		resolved.Color.W() == 0.0 {
		resolved.Color = mgl32.Vec4{1.0, 1.0, 1.0, 1.0}
	}
	if resolved.Intensity <= 0.0 {
		resolved.Intensity = 1.0
	}
	if resolved.InnerAngleDeg <= 0.0 {
		resolved.InnerAngleDeg = 20.0
	}
	if resolved.OuterAngleDeg <= 0.0 {
		resolved.OuterAngleDeg = 35.0
	}
	return resolved
}

// 补全方向光绘制参数
// 当前允许调用方传nil， 使用白色、强度为1的默认屏幕渐变参数
func resolveDirectionalLightOptions(options *DirectionalLightOptions) DirectionalLightOptions {
	if options == nil {
		return DirectionalLightOptions{
			Color:       mgl32.Vec4{1.0, 1.0, 1.0, 1.0},
			Intensity:   1.0,
			Offset:      0.5,
			Softness:    0.1,
			MiddayBlend: 0.0,
		}
	}
	resolved := *options
	if resolved.Color.X() == 0.0 &&
		resolved.Color.Y() == 0.0 &&
		resolved.Color.Z() == 0.0 &&
		resolved.Color.W() == 0.0 {
		resolved.Color = mgl32.Vec4{1.0, 1.0, 1.0, 1.0}
	}
	if resolved.Intensity <= 0.0 {
		resolved.Intensity = 1.0
	}
	if resolved.Softness <= 0.0 {
		resolved.Softness = 0.1
	}
	return resolved
}
