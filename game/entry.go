package game

import (
	"log/slog"
	"os"

	ectx "tiny_farm/engine/context"
	"tiny_farm/engine/core"
)

// initEnvironment 初始化当前游戏运行环境。
// 当前主要是把日志系统准备好，便于观察启动流程。
func initEnvironment() {
	minLevel := slog.LevelDebug
	// minLevel := slog.LevelInfo

	options := &slog.HandlerOptions{
		Level: minLevel,
	}
	handler := slog.NewTextHandler(os.Stdout, options)
	slog.SetDefault(slog.New(handler))
}

// Run 是游戏层统一入口。
func Run() {
	initEnvironment()

	app := core.NewGameApp()
	app.RegisterSceneSetup(setupInitialScene)
	app.Run()
}

// setupInitialScene 预留给后续初始场景装配逻辑。
func setupInitialScene(ctx *ectx.Context) {
	_ = ctx
}
