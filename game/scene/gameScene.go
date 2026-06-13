package scene

import (
	"errors"
	"log/slog"

	ectx "tiny_farm/engine/context"
	esystem "tiny_farm/engine/ecs/system"
	escene "tiny_farm/engine/scene"
	"tiny_farm/engine/utils/defs"
	gfactory "tiny_farm/game/factory"
	gsystem "tiny_farm/game/system"

	"github.com/yohamta/donburi"
)

// 持有当前游戏场景的 ECS World 和系统执行顺序
type GameScene struct {
	// 提供输入、渲染、资源和场景管理等跨场景服务
	context *ectx.Context
	// 保存当前场景独占的实体和组件数据
	world donburi.World
	// 玩家控制系统
	playerControlSystem *gsystem.PlayerControlSystem
	// 实体移动系统
	movementSystem *esystem.MovementSystem
	// 实体删除系统
	removeEntitySystem *esystem.RemoveEntitySystem
	// 音频事件系统
	audioSystem *esystem.AudioSystem
	// 渲染系统
	renderSystem *esystem.RenderSystem
	// 标记场景资源和运行时状态是否已经初始化
	initialized bool
}

// 确保 GameScene 实现 IScene 接口
var _ escene.IScene = (*GameScene)(nil)

// 创建尚未初始化的游戏场景
func NewGameScene(ctx *ectx.Context) *GameScene {
	return &GameScene{context: ctx}
}

// 返回场景调试名称
func (s *GameScene) Name() string {
	return "GameScene"
}

// 创建本场景独占的 World、系统和初始实体
func (s *GameScene) Init() error {
	if s == nil {
		return errors.New("game scene is nil")
	}
	if s.initialized {
		return nil
	}
	if s.context == nil {
		return errors.New("runtime context is nil")
	}

	s.world = donburi.NewWorld()
	s.playerControlSystem = gsystem.NewPlayerControlSystem(64.0)
	s.movementSystem = esystem.NewMovementSystem()
	s.removeEntitySystem = esystem.NewRemoveEntitySystem()
	audioSystem, err := esystem.NewAudioSystem(s.world, s.context.Dispatcher(), s.context.AudioPlayer(), s.context.Camera())
	if err != nil {
		s.Close()
		return err
	}
	s.audioSystem = audioSystem
	s.renderSystem = esystem.NewRenderSystem()

	if _, err := gfactory.CreatePlayer(s.world); err != nil {
		s.Close()
		return err
	}
	s.playSceneMusic()
	s.initialized = true
	return nil
}

// 按玩家控制、移动、集中删除的顺序更新场景
func (s *GameScene) Update(deltaTime float64) {
	if s == nil || !s.initialized {
		return
	}
	s.playerControlSystem.Update(s.world, s.context.InputManager())
	s.movementSystem.Update(s.world, deltaTime)
	s.removeEntitySystem.Update(s.world)
}

// 提交本场景的正式精灵绘制命令
func (s *GameScene) Render() error {
	if s == nil || !s.initialized {
		return nil
	}
	return s.renderSystem.Render(
		s.world,
		s.context.ResourceManager(),
		s.context.Renderer(),
	)
}

// 释放本场景持有的 World 和系统引用
func (s *GameScene) Close() {
	if s == nil {
		return
	}
	s.world = nil
	s.playerControlSystem = nil
	s.movementSystem = nil
	s.removeEntitySystem = nil
	if s.audioSystem != nil {
		s.audioSystem.Close()
		s.audioSystem = nil
	}
	s.renderSystem = nil
	s.initialized = false
}

// 播放当前场景的临时背景音乐
//
// 当前作为音频系统首版验证入口，后续应由正式场景配置或状态机决定播放哪首音乐
func (s *GameScene) playSceneMusic() {
	if s == nil || s.context == nil || s.context.AudioPlayer() == nil {
		return
	}
	if err := s.context.AudioPlayer().PlayMusic(defs.ResourceKey("scene-bg-music"), true, 0); err != nil {
		slog.Warn("play scene music failed", slog.Any("err", err))
	}
}
