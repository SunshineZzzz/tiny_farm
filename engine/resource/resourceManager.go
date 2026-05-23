package resource

import (
	"errors"
	"fmt"

	"tiny_farm/engine/render"

	"github.com/go-gl/mathgl/mgl32"
)

// 对外提供统一资源访问入口
//
// 当前阶段先接入纹理资源，音频、字体和地图等类型后续继续挂在这一层下面
type ResourceManager struct {
	// 负责纹理加载、查询和释放
	textures *textureManager
}

// 创建资源管理器
//
// 当前纹理加载依赖 Renderer，所以需要在 Renderer 初始化完成后创建
func NewResourceManager(renderer *render.Renderer) (*ResourceManager, error) {
	textureManager, err := newTextureManager(renderer)
	if err != nil {
		return nil, err
	}
	return &ResourceManager{
		textures: textureManager,
	}, nil
}

// 按资源映射文件预加载当前阶段支持的资源
//
// 当前只真实加载 texture，sound 和 music 字段先由 ResourceMapping 解析保留
func (m *ResourceManager) LoadResources(path string) error {
	if m == nil {
		return errors.New("resource manager is nil")
	}
	mapping, err := loadResourceMapping(path)
	if err != nil {
		return err
	}

	for key, texturePath := range mapping.Texture {
		if _, err := m.GetTexture(key, texturePath); err != nil {
			return fmt.Errorf("load resources %q: %w", path, err)
		}
	}
	return nil
}

// 释放资源管理器持有的全部缓存
func (m *ResourceManager) Clear() {
	if m == nil {
		return
	}
	if m.textures != nil {
		m.textures.clearTextures()
	}
}

// 获取纹理资源，如果未加载则尝试加载key或者paths[0]
func (m *ResourceManager) GetTexture(key ResourceKey, paths ...string) (*render.Texture, error) {
	if m == nil || m.textures == nil {
		return nil, errors.New("resource manager texture manager is nil")
	}
	return m.textures.loadTexture(key, paths...)
}

// 返回已加载纹理的像素尺寸
func (m *ResourceManager) TextureSize(key ResourceKey) (mgl32.Vec2, bool) {
	if m == nil || m.textures == nil {
		return mgl32.Vec2{}, false
	}
	return m.textures.textureSize(key)
}

// 卸载指定纹理资源
func (m *ResourceManager) UnloadTexture(key ResourceKey) {
	if m == nil || m.textures == nil {
		return
	}
	m.textures.unloadTexture(key)
}

// 返回按 key 排序的纹理调试信息
func (m *ResourceManager) TextureDebugInfo() []TextureDebugInfo {
	if m == nil || m.textures == nil {
		return nil
	}
	return m.textures.textureDebugInfo()
}
