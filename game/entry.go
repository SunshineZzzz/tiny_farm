package game

import (
	"errors"
	"log/slog"
	"os"

	ectx "tiny_farm/engine/context"
	"tiny_farm/engine/core"
	escene "tiny_farm/engine/scene"
	gscene "tiny_farm/game/scene"
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

// 创建应用启动后的初始游戏场景
func setupInitialScene(ctx *ectx.Context) (escene.IScene, error) {
	if ctx == nil {
		return nil, errors.New("runtime context is nil")
	}
	return gscene.NewGameScene(ctx), nil
}
