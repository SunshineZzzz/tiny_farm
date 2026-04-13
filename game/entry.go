package game

import (
	"log/slog"
	"os"

	ectx "tiny_farm/engine/context"
	"tiny_farm/engine/core"
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

// 预留给后续初始化场景装配逻辑
func setupInitialScene(ctx *ectx.Context) {
	_ = ctx
}
