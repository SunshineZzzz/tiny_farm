package resource

import (
	"errors"
	"fmt"

	"tiny_farm/engine/render"

	"github.com/go-gl/mathgl/mgl32"
)

// 管理纹理资源的加载、缓存和释放
//
// 当前只通过 Renderer 创建底层纹理，资源层负责 key 到纹理实例的生命周期
type textureManager struct {
	// 用于创建底层可绘制纹理
	renderer *render.Renderer
	// 按资源 key 缓存已加载的纹理
	textures map[ResourceKey]*textureEntry
}

// 保存单个纹理资源的运行时信息
type textureEntry struct {
	// 可提交给 Renderer 绘制的纹理
	texture *render.Texture
	// 纹理来源路径
	sourcePath string
	// 纹理像素尺寸
	size mgl32.Vec2
}

// 创建纹理资源管理器
//
// 当前必须依赖已初始化的 Renderer，因为纹理创建需要有效 OpenGL 上下文
func newTextureManager(renderer *render.Renderer) (*textureManager, error) {
	if renderer == nil {
		return nil, errors.New("renderer is nil")
	}
	return &textureManager{
		renderer: renderer,
		textures: make(map[ResourceKey]*textureEntry),
	}, nil
}

// 加载纹理并按 key 缓存
//
// 如果 key 已加载，直接返回缓存纹理，避免重复创建底层 OpenGL texture
func (m *textureManager) loadTexture(key ResourceKey, paths ...string) (*render.Texture, error) {
	if m == nil || m.renderer == nil {
		return nil, errors.New("texture manager is nil")
	}
	if key == "" {
		return nil, errors.New("texture key is empty")
	}
	if entry, ok := m.textures[key]; ok && entry != nil && entry.texture != nil {
		return entry.texture, nil
	}

	path := string(key)
	if len(paths) != 0 {
		path = paths[0]
	}
	texture, err := m.renderer.LoadTexture(path)
	if err != nil {
		return nil, fmt.Errorf("load texture %q from %q: %w", key, path, err)
	}
	m.textures[key] = &textureEntry{
		texture:    texture,
		sourcePath: path,
		size:       texture.Size(),
	}
	return texture, nil
}

// 获取已加载纹理
//
// 当前只查询缓存，不把 key 当作路径隐式加载
func (m *textureManager) getTexture(key ResourceKey) (*render.Texture, bool) {
	if m == nil {
		return nil, false
	}
	entry, ok := m.textures[key]
	if !ok || entry == nil || entry.texture == nil {
		return nil, false
	}
	return entry.texture, true
}

// 返回已加载纹理的像素尺寸
func (m *textureManager) textureSize(key ResourceKey) (mgl32.Vec2, bool) {
	if m == nil {
		return mgl32.Vec2{}, false
	}
	entry, ok := m.textures[key]
	if !ok || entry == nil {
		return mgl32.Vec2{}, false
	}
	return entry.size, true
}

// 卸载指定纹理
func (m *textureManager) unloadTexture(key ResourceKey) {
	if m == nil {
		return
	}
	entry, ok := m.textures[key]
	if !ok || entry == nil {
		return
	}
	if entry.texture != nil {
		entry.texture.Close()
	}
	delete(m.textures, key)
}

// 清空全部纹理缓存
func (m *textureManager) clearTextures() {
	if m == nil {
		return
	}
	for key := range m.textures {
		m.unloadTexture(key)
	}
	m.textures = make(map[ResourceKey]*textureEntry)
}
