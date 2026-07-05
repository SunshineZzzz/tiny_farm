package context

import (
	"errors"

	"tiny_farm/engine/abstract"
	"tiny_farm/engine/audio"
	"tiny_farm/engine/input"
	"tiny_farm/engine/render"
	"tiny_farm/engine/resource"
	"tiny_farm/engine/scene"
	"tiny_farm/engine/utils/dispatch"
)

// 持有跨游戏场景复用的核心引擎服务
//
// 当前只提供运行时装配所需的基础服务，场景状态和后续 ECS World 不放在这里
type Context struct {
	inputManager    *input.InputManager
	renderer        *render.Renderer
	resourceManager *resource.ResourceManager
	audioPlayer     *audio.AudioPlayer
	textRenderer    *render.TextRenderer
	camera          *render.Camera
	dispatcher      *dispatch.Dispatcher
	gameState       abstract.IGameState
	sceneManager    *scene.SceneManager
}

// 创建运行时上下文，并确保游戏层装配时拿到的核心服务均可用
func NewContext(
	inputManager *input.InputManager,
	renderer *render.Renderer,
	resourceManager *resource.ResourceManager,
	audioPlayer *audio.AudioPlayer,
	textRenderer *render.TextRenderer,
	camera *render.Camera,
	dispatcher *dispatch.Dispatcher,
	gameState abstract.IGameState,
) (*Context, error) {
	switch {
	case inputManager == nil:
		return nil, errors.New("input manager is nil")
	case renderer == nil:
		return nil, errors.New("renderer is nil")
	case resourceManager == nil:
		return nil, errors.New("resource manager is nil")
	case audioPlayer == nil:
		return nil, errors.New("audio player is nil")
	case textRenderer == nil:
		return nil, errors.New("text renderer is nil")
	case camera == nil:
		return nil, errors.New("camera is nil")
	case dispatcher == nil:
		return nil, errors.New("dispatcher is nil")
	case gameState == nil:
		return nil, errors.New("game state is nil")
	}

	return &Context{
		inputManager:    inputManager,
		renderer:        renderer,
		resourceManager: resourceManager,
		audioPlayer:     audioPlayer,
		textRenderer:    textRenderer,
		camera:          camera,
		dispatcher:      dispatcher,
		gameState:       gameState,
	}, nil
}

// 获取输入管理器
func (c *Context) InputManager() *input.InputManager {
	return c.inputManager
}

// 获取渲染器
func (c *Context) Renderer() *render.Renderer {
	return c.renderer
}

// 获取资源管理器
func (c *Context) ResourceManager() *resource.ResourceManager {
	return c.resourceManager
}

// 获取音频播放器
func (c *Context) AudioPlayer() *audio.AudioPlayer {
	return c.audioPlayer
}

// 获取文本渲染器
func (c *Context) TextRenderer() *render.TextRenderer {
	return c.textRenderer
}

// 获取当前世界相机
func (c *Context) Camera() *render.Camera {
	return c.camera
}

// 获取事件分发器
func (c *Context) Dispatcher() *dispatch.Dispatcher {
	return c.dispatcher
}

// 获取游戏状态
func (c *Context) GameState() abstract.IGameState {
	return c.gameState
}

// 注入应用创建的场景管理器
func (c *Context) SetSceneManager(sceneManager *scene.SceneManager) {
	if c == nil {
		return
	}
	c.sceneManager = sceneManager
}

// 获取场景管理器，用于请求安全时点的场景切换
func (c *Context) SceneManager() *scene.SceneManager {
	if c == nil {
		return nil
	}
	return c.sceneManager
}
