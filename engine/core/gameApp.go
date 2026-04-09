package core

import (
	"errors"
	"fmt"
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
}

// NewGameApp 创建应用实例，并初始化帧率控制器。
func NewGameApp() *GameApp {
	return &GameApp{
		fpsManager: NewFPS(),
	}
}

// RegisterSceneSetup 注册游戏层提供的初始化逻辑。
func (a *GameApp) RegisterSceneSetup(fn sceneSetupFunc) {
	a.sceneSetup = fn
}

// Run 启动唯一主循环。
//
// 当前版本只验证主循环和相对帧率方案是否工作，
// 所以每轮只更新 FPS 并输出 delta time 观察结果。
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
		fmt.Printf("deltaTime: %f\n", deltaTime)
	}
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
