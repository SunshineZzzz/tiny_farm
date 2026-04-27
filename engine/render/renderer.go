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
