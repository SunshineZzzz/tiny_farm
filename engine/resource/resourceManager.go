package resource

import (
	"errors"
	"fmt"

	"tiny_farm/engine/render"

	"github.com/go-gl/mathgl/mgl32"
)

// 对外提供统一资源访问入口
//
// 当前阶段接入纹理和音频缓存，字体和地图等类型后续继续挂在这一层下面
type ResourceManager struct {
	// 负责纹理加载、查询和释放
	textures *textureManager
	// 负责音效和音乐解码缓存
	audio *audioManager
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
		audio:    newAudioManager(),
	}, nil
}

// 按资源映射文件预加载当前阶段支持的资源
//
// 当前会同步加载 texture、sound 和 music，失败时返回带映射文件路径的错误
func (m *ResourceManager) LoadResources(path string) error {
	if m == nil {
		return errors.New("resource manager is nil")
	}
	mapping, err := loadResourceMapping(path)
	if err != nil {
		return err
	}

	for key, texturePath := range mapping.Texture {
		if _, err := m.LoadTexture(key, texturePath); err != nil {
			return fmt.Errorf("load resources %q: %w", path, err)
		}
	}
	for key, soundPath := range mapping.Sound {
		if _, err := m.LoadSound(key, soundPath); err != nil {
			return fmt.Errorf("load resources %q: %w", path, err)
		}
	}
	for key, musicPath := range mapping.Music {
		if _, err := m.LoadMusic(key, musicPath); err != nil {
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
	if m.audio != nil {
		m.audio.clear()
	}
	if m.textures != nil {
		m.textures.clearTextures()
	}
}

// 加载纹理资源，如果命中缓存则直接返回已有纹理
func (m *ResourceManager) LoadTexture(key ResourceKey, paths ...string) (*render.Texture, error) {
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

// 返回按 key 排序的音效调试信息
func (m *ResourceManager) SoundDebugInfo() []AudioDebugInfo {
	if m == nil || m.audio == nil {
		return nil
	}
	return m.audio.soundDebugInfo()
}

// 返回按 key 排序的音乐调试信息
func (m *ResourceManager) MusicDebugInfo() []AudioDebugInfo {
	if m == nil || m.audio == nil {
		return nil
	}
	return m.audio.musicDebugInfo()
}

// 加载音效资源，如果命中缓存则直接返回已有 buffer
func (m *ResourceManager) LoadSound(key ResourceKey, paths ...string) (AudioBufferHandle, error) {
	if m == nil || m.audio == nil {
		return AudioBufferHandle{}, errors.New("resource manager audio manager is nil")
	}
	return m.audio.loadSound(key, paths...)
}

// 加载音乐资源，如果命中缓存则直接返回已有 buffer
func (m *ResourceManager) LoadMusic(key ResourceKey, paths ...string) (AudioBufferHandle, error) {
	if m == nil || m.audio == nil {
		return AudioBufferHandle{}, errors.New("resource manager audio manager is nil")
	}
	return m.audio.loadMusic(key, paths...)
}

// 卸载指定音效资源
func (m *ResourceManager) UnloadSound(key ResourceKey) {
	if m == nil || m.audio == nil {
		return
	}
	m.audio.unloadSound(key)
}

// 卸载指定音乐资源
func (m *ResourceManager) UnloadMusic(key ResourceKey) {
	if m == nil || m.audio == nil {
		return
	}
	m.audio.unloadMusic(key)
}
