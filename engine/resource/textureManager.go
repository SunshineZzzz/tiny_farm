package resource

import (
	"errors"
	"fmt"
	"slices"

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

// 返回可用纹理，缓存未命中时按传入路径或已登记路径加载
func (m *textureManager) texture(key ResourceKey, paths ...string) (*render.Texture, error) {
	if m == nil || m.renderer == nil {
		return nil, errors.New("texture manager is nil")
	}
	entry, ok := m.textures[key]
	if ok && entry != nil && entry.texture != nil {
		return entry.texture, nil
	}
	path := ""
	if len(paths) > 0 {
		path = paths[0]
	}
	if path == "" {
		path = string(key)
	}
	return m.loadTexture(key, path)
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

// 返回按 key 排序的纹理调试信息
func (m *textureManager) textureDebugInfo() []TextureDebugInfo {
	if m == nil || len(m.textures) == 0 {
		return nil
	}

	keys := make([]ResourceKey, 0, len(m.textures))
	for key, entry := range m.textures {
		if entry == nil {
			continue
		}
		keys = append(keys, key)
	}
	slices.Sort(keys)

	info := make([]TextureDebugInfo, 0, len(keys))
	for _, key := range keys {
		entry := m.textures[key]
		info = append(info, TextureDebugInfo{
			Key:         key,
			SourcePath:  entry.sourcePath,
			Size:        entry.size,
			MemoryBytes: estimateTextureMemoryBytes(entry.size),
		})
	}
	return info
}

// 按当前纹理上传格式估算内存占用
func estimateTextureMemoryBytes(size mgl32.Vec2) int {
	width := int(size.X())
	height := int(size.Y())
	if width <= 0 || height <= 0 {
		return 0
	}
	return width * height * 4
}
