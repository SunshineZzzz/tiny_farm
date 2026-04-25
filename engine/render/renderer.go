package render

import (
	"errors"

	"tiny_farm/engine/render/opengl"

	"github.com/SunshineZzzz/purego-sdl3/sdl"
	"github.com/go-gl/mathgl/mgl32"
)

// 渲染外观层(Facade)
//
// 当前阶段先做一层最小 facade，把 game 层和 OpenGL 后端隔开
// 内部仍然直接委托给 GLRenderer，后续再在这里演进更高层的绘制接口
type Renderer struct {
	// 后端渲染器
	backend *opengl.GLRenderer
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
	return &Renderer{backend: backend}, nil
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

// 清空当前帧
func (r *Renderer) Clear() {
	if r == nil || r.backend == nil {
		return
	}
	r.backend.Clear()
}

// 绘制纯色矩形
func (r *Renderer) DrawRect(rect mgl32.Vec4, color mgl32.Vec4) error {
	if r == nil || r.backend == nil {
		return errors.New("renderer is nil")
	}
	return r.backend.DrawRect(rect, color)
}

// 绘制贴图矩形
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

// 绘制贴图源矩形
func (r *Renderer) DrawTextureSourceRect(texture *Texture, dstRect mgl32.Vec4, srcRect mgl32.Vec4) error {
	if r == nil || r.backend == nil {
		return errors.New("renderer is nil")
	}
	if texture == nil || texture.backend == nil {
		return errors.New("texture is nil")
	}
	return r.backend.DrawTextureSourceRect(texture.backend, dstRect, srcRect)
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
}

// 释放纹理资源
func (t *Texture) Close() {
	if t == nil || t.backend == nil {
		return
	}
	t.backend.Close()
	t.backend = nil
}
