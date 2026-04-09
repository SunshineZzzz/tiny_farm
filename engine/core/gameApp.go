package core

import (
	"errors"
	"log/slog"
	"runtime"

	ectx "tiny_farm/engine/context"
)

// sceneSetupFunc 用于把游戏层初始化逻辑注入到引擎入口。
type sceneSetupFunc func(*ectx.Context)

// GameApp 是当前项目的应用壳。
// 当前阶段只保留一个主循环，输入、逻辑、渲染、事件分发后续都走这一个循环。
type GameApp struct {
	// sceneSetup 预留给游戏层做启动时装配。
	sceneSetup sceneSetupFunc
	// isRunning 控制主循环是否继续执行。
	isRunning bool
	// fpsManager 负责本轮主循环的控帧和 dt 计算。
	fpsManager *FPS
	// frameCount 用于统计一段时间内累计跑了多少帧。
	frameCount int
	// deltaTimeSum 用于累计这一段时间内的 dt 总和。
	deltaTimeSum float64
	// statInterval 用于控制多久输出一次统计信息。
	statInterval float64
}

// NewGameApp 创建应用实例，并初始化帧率控制器。
func NewGameApp() *GameApp {
	return &GameApp{
		fpsManager:   NewFPS(),
		statInterval: 1.0,
	}
}

// RegisterSceneSetup 注册游戏层提供的初始化逻辑。
func (a *GameApp) RegisterSceneSetup(fn sceneSetupFunc) {
	a.sceneSetup = fn
}

// Run 启动唯一主循环。
//
// 当前版本继续使用相对帧率方案，但不再逐帧打印 dt。
// 为了观察运行效果，统计信息改成低频输出，尽量减少对主循环的干扰。
func (a *GameApp) Run() {
	if err := a.init(); err != nil {
		return
	}
	defer a.close()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	for a.isRunning {
		a.fpsManager.Update()
		deltaTime := a.fpsManager.GetDeltaTime()
		a.tick(deltaTime)
	}
}

// tick 是单帧逻辑入口。
// 当前先只做帧统计，后续输入、逻辑、渲染、事件分发都应接在这里。
func (a *GameApp) tick(deltaTime float64) {
	a.frameCount++
	a.deltaTimeSum += deltaTime

	if a.deltaTimeSum < a.statInterval {
		return
	}

	avgDeltaTime := a.deltaTimeSum / float64(a.frameCount)
	avgFps := float64(a.frameCount) / a.deltaTimeSum

	slog.Info(
		"frame stats",
		slog.Int("frames", a.frameCount),
		slog.Float64("avgDeltaTime", avgDeltaTime),
		slog.Float64("avgFps", avgFps),
	)

	a.frameCount = 0
	a.deltaTimeSum = 0.0
}

// init 完成运行前的最小初始化。
func (a *GameApp) init() error {
	if !a.initTimer() {
		return errors.New("init timer failed")
	}

	a.isRunning = true
	return nil
}

// initTimer 初始化当前版本使用的帧率参数。
func (a *GameApp) initTimer() bool {
	a.fpsManager.SetTargetFps(60)
	return true
}

// close 预留给后续资源释放逻辑。
func (a *GameApp) close() {
}
