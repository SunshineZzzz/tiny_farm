package game

import (
	"errors"
	"log/slog"
	"os"

	ectx "tiny_farm/engine/context"
	"tiny_farm/engine/core"
	gameworld "tiny_farm/game/world"

	"github.com/yohamta/donburi"
)

// 初始化当前游戏运行环境
func initEnvironment() {
	// 初始化日志系统
	{
		minLevel := slog.LevelDebug
		// 日志级别可改为 slog LevelInfo

		options := &slog.HandlerOptions{
			Level: minLevel,
		}
		handler := slog.NewTextHandler(os.Stdout, options)
		slog.SetDefault(slog.New(handler))
	}
}

// 游戏层统一入口
func Run() {
	initEnvironment()

	app := core.NewGameApp()
	app.RegisterSceneSetup(setupInitialScene)
	app.Run()
}

// 装配当前过渡阶段的初始 ECS 世界
func setupInitialScene(ctx *ectx.Context, world donburi.World) error {
	if ctx == nil {
		return errors.New("runtime context is nil")
	}
	if world == nil {
		return errors.New("ecs world is nil")
	}

	entity, err := gameworld.CreateDemoEntity(world)
	if err != nil {
		return err
	}

	slog.Debug("demo ecs entity created", slog.Any("entity", entity))
	return nil
}
