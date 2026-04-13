package core

import (
	"errors"
	"log/slog"
	"runtime"

	ectx "tiny_farm/engine/context"
	"tiny_farm/engine/render/opengl"
	"tiny_farm/engine/utils/events"

	"github.com/SunshineZzzz/purego-sdl3/sdl"
	"github.com/go-gl/mathgl/mgl32"
)

// 用于把游戏层初始化逻辑注入到引擎入口
type sceneSetupFunc func(*ectx.Context)

// 是当前项目的应用壳
//
// 当前阶段保留一个主循环，输入、逻辑、渲染、事件分发后续都走这个循环
type GameApp struct {
	// 预留给游戏层做启动时装配
	sceneSetup sceneSetupFunc
	// 控制主循环是否继续执行
	isRunning bool
	// 游戏窗口
	window *sdl.Window
	// 标记 SDL 子系统是否已经初始化
	sdlInitialized bool
	// 管理当前 SDL 窗口和 OpenGL 上下文
	glRenderer *opengl.GLRenderer
	// 游戏配置
	config *Config
	// 负责本轮主循环的控帧和 dt 计算
	fpsManager *FPS
	// 事件分发器
	dispatcher *events.Dispatcher

	// 用于统计一段时间内累计跑了多少帧
	frameCount int
	// 用于累计这一段时间内的 dt 总和
	deltaTimeSum float64
	// 用于控制多久输出一次统计信息
	statInterval float64
}

// 创建应用实例，并初始化帧率控制器
func NewGameApp() *GameApp {
	return &GameApp{
		fpsManager:   NewFPS(),
		statInterval: 1.0,
	}
}

// 注册游戏层提供的初始化逻辑
func (a *GameApp) RegisterSceneSetup(fn sceneSetupFunc) {
	a.sceneSetup = fn
}

// 启动唯一主循环
//
// 上下文和函数加载都依赖当前 OS 线程，所以必须先锁线程，再进入 SDL 和 GL 初始化
// 当前版本继续使用相对帧率方案，统计信息保持低频输出
func (a *GameApp) Run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := a.init(); err != nil {
		slog.Error("game app init failed", slog.Any("err", err))
		a.close()
		return
	}
	defer a.close()

	slog.Debug(
		"game app run",
		slog.Int("targetFps", a.fpsManager.GetTargetFps()),
	)

	for a.isRunning {
		a.fpsManager.Update()
		deltaTime := a.fpsManager.GetDeltaTime()

		// 分发 enqueue 的事件，这个放到主循环后段执行
		a.dispatcher.Update()

		// 渲染入口预留

		// 帧率统计
		a.frameStats(deltaTime)
	}
}

// 低频输出当前主循环帧率统计
func (a *GameApp) frameStats(deltaTime float64) {
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

// 完成运行前的最小初始化
func (a *GameApp) init() error {
	if a.sceneSetup == nil {
		return errors.New("scene setup not registered")
	}

	if err := a.initConfig(); err != nil {
		return err
	}

	if err := a.initTimer(); err != nil {
		return err
	}

	if err := a.initDispatcher(); err != nil {
		return err
	}

	if err := a.initSDL(); err != nil {
		return err
	}

	if err := a.initGLRenderer(); err != nil {
		return err
	}

	a.isRunning = true

	slog.Debug(
		"game app init",
		slog.Bool("isRunning", a.isRunning),
	)
	return nil
}

// 初始化配置文件
func (a *GameApp) initConfig() error {
	config, err := NewConfig("config/window.json")
	if err != nil {
		return err
	}
	a.config = config
	slog.Debug("config init success", slog.Any("config", config))
	return nil
}

// 初始化当前版本使用的帧率参数
func (a *GameApp) initTimer() error {
	a.fpsManager.SetTargetFps(a.config.Performance.TargetFPS)
	slog.Debug("fps manager init success", slog.Int("targetFps", a.fpsManager.GetTargetFps()))
	return nil
}

// 初始化事件分发器
func (a *GameApp) initDispatcher() error {
	a.dispatcher = events.NewDispatcher()
	slog.Debug("dispatcher init success")
	return nil
}

// 初始化 SDL 系统
func (a *GameApp) initSDL() error {
	// 通知 SDL 框架不要显示系统默认的输入法候选词窗口
	sdl.SetHint("SDL_HINT_IME_SHOW_UI", "0")
	if !sdl.Init(sdl.InitVideo | sdl.InitAudio) {
		return errors.New("init sdl failed")
	}
	a.sdlInitialized = true

	// 设置窗口大小，窗口大小乘以窗口缩放比例
	windowWidth := int32(float32(a.config.Window.Width) * a.config.Window.WindowScale)
	windowHeight := int32(float32(a.config.Window.Height) * a.config.Window.WindowScale)
	a.window = sdl.CreateWindow(a.config.Window.Title, windowWidth, windowHeight, sdl.WindowOpenGL|sdl.WindowResizable)
	if a.window == nil {
		return errors.New("create window failed")
	}
	slog.Debug("sdl window init success", slog.Int("width", int(windowWidth)), slog.Int("height", int(windowHeight)))
	return nil
}

// 初始化 GL 渲染器
func (a *GameApp) initGLRenderer() error {
	// 获取逻辑分辨率，窗口大小乘以逻辑缩放比例
	// 逻辑尺寸定义离屏渲染 FBO 的固定设计分辨率
	// 窗口变化时逻辑分辨率保持一致，UI 和相机计算也保持一致
	// 窗口缩放只影响窗口初始大小，逻辑缩放决定渲染质量
	logicalSize := mgl32.Vec2{float32(a.config.Window.Width) * a.config.Window.LogicalScale,
		float32(a.config.Window.Height) * a.config.Window.LogicalScale}
	glRenderer, err := opengl.NewGLRenderer(a.window, logicalSize, "config/render.json")
	if err != nil {
		return err
	}
	a.glRenderer = glRenderer
	a.glRenderer.SetVSyncEnabled(a.config.Graphics.Vsync)
	slog.Debug("open gl renderer success", slog.Any("logicalSize", logicalSize))
	return nil
}

// 释放应用持有的运行时资源
func (a *GameApp) close() {
	if a.isRunning {
		slog.Warn("game app is running, close")
	}

	slog.Debug("game app closed", slog.Bool("isRunning", a.isRunning))

	if a.glRenderer != nil {
		a.glRenderer.Close()
		a.glRenderer = nil
	}

	if a.window != nil {
		sdl.DestroyWindow(a.window)
		a.window = nil
	}

	if a.sdlInitialized {
		sdl.Quit()
		a.sdlInitialized = false
	}

	a.isRunning = false
}
