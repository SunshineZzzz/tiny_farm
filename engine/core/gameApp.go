package core

import (
	"errors"
	"log/slog"
	"runtime"

	"tiny_farm/engine/abstract"
	ectx "tiny_farm/engine/context"
	"tiny_farm/engine/input"
	"tiny_farm/engine/render"
	"tiny_farm/engine/resource"
	"tiny_farm/engine/utils/dispatch"
	"tiny_farm/engine/utils/event"

	"github.com/SunshineZzzz/purego-sdl3/sdl"
	"github.com/go-gl/mathgl/mgl32"
)

// 把游戏层初始化逻辑注入到引擎入口
type sceneSetupFunc func(*ectx.Context)

// 当前项目的应用壳
//
// 当前阶段保留一个主循环，输入、逻辑、渲染和事件分发后续都走这一个循环
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
	renderer *render.Renderer
	// 统一管理资源加载、缓存和释放
	resourceManager *resource.ResourceManager
	// 当前帧使用的世界相机
	camera *render.Camera
	// 阶段 4 用于验证贴图绘制的临时纹理
	demoTexture *render.Texture
	// 游戏配置
	config *Config
	// 负责本轮主循环的控帧和 dt 计算
	fpsManager *FPS
	// 事件分发器
	dispatcher *dispatch.Dispatcher
	// 管理 SDL 输入事件和动作状态
	inputManager *input.InputManager
	// 游戏状态
	gameState *GameState

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
		// 每帧节拍（从“玩家输入”到“屏幕呈现”）：
		// 1) 时间推进：计算本帧 deltaTime（用于驱动更新）
		// 2) 输入/事件：轮询 SDL 事件、更新动作状态，并触发必要的引擎事件（例如 Quit/WindowResized）
		// 3) 更新：只更新栈顶场景（以及其系统/逻辑）；场景切换通过事件请求，在 update 末尾统一处理
		// 4) 渲染：清屏 → 渲染场景/调试 UI → present
		// 5) 分发队列事件：处理本帧 enqueue 的事件
		//    TinyFarm 把 dispatcher.update 放在 render 之后：队列事件在“本帧画面已呈现”后才会被分发，
		//    这样可以避免递归触发导致的时序混乱，并把大多数数据类事件自然变成“下一帧的输入”。
		a.fpsManager.Update()
		deltaTime := a.fpsManager.GetDeltaTime()

		a.handleInputEvents()
		a.update(deltaTime)
		a.render()

		// 分发 enqueue 的事件
		// 在 update() 里 enqueue 了一个事件，它在"本帧画面画完之后"才会被处理。 换句话说，对于游戏逻辑而言，本帧 enqueue 的事件，是给下一帧用的输入。
		// 因为这样可以保证：整个 update 阶段，所有系统都在同一个"数据快照"上运行，不会有人因为中途收到事件而看到不一致的状态。渲染完成后再结算，时序最清晰。
		a.dispatcher.Update()

		// 帧率统计
		a.frameStats(deltaTime)
	}
}

// 处理并分发输入事件
func (a *GameApp) handleInputEvents() {
	a.inputManager.Update()
}

// 更新游戏状态
func (a *GameApp) update(deltaTime float64) {
}

// 渲染
func (a *GameApp) render() {
	if a.renderer == nil {
		return
	}

	a.renderer.BeginFrame(a.camera)
	a.renderer.Clear()
	a.renderer.DrawWorldRect(mgl32.Vec4{32.0, 32.0, 96.0, 64.0}, mgl32.Vec4{0.9, 0.72, 0.32, 1.0})
	a.renderer.DrawWorldRect(mgl32.Vec4{144.0, 48.0, 48.0, 96}, mgl32.Vec4{0.38, 0.72, 0.92, 1.0})
	a.renderer.DrawWorldRect(mgl32.Vec4{216.0, 80.0, 72.0, 40.0}, mgl32.Vec4{0.78, 0.42, 0.88, 1.0})
	if a.demoTexture != nil {
		a.renderer.DrawWorldRect(mgl32.Vec4{32.0, 4.0, 96.0, 32.0}, mgl32.Vec4{0.78, 0.42, 0.88, 1.0})
		if err := a.renderer.DrawWorldTexture(a.demoTexture, mgl32.Vec4{32.0, 4.0, 96.0, 32.0}, mgl32.Vec4{0.0, 0.0, 1.0, 1.0}); err != nil {
			slog.Error("draw demo texture failed", slog.Any("err", err))
		}
		a.renderer.DrawWorldRect(mgl32.Vec4{32.0, 92.0, 48.0, 32.0}, mgl32.Vec4{0.78, 0.42, 0.88, 1.0})
		if err := a.renderer.DrawWorldTextureSourceRect(a.demoTexture, mgl32.Vec4{32.0, 92.0, 48.0, 32.0}, mgl32.Vec4{0.0, 0.0, 24.0, 16.0}); err != nil {
			slog.Error("draw demo texture source rect failed", slog.Any("err", err))
		}
	}
	if err := a.renderer.DrawUIRect(mgl32.Vec4{8.0, 8.0, 72.0, 16.0}, mgl32.Vec4{0.08, 0.08, 0.1, 1.0}); err != nil {
		slog.Error("draw demo ui rect failed", slog.Any("err", err))
	}
	if a.demoTexture != nil {
		if err := a.renderer.DrawUITexture(a.demoTexture, mgl32.Vec4{80.0, 10.0, 24.0, 12.0}, mgl32.Vec4{0.0, 0.0, 1.0, 1.0}); err != nil {
			slog.Error("draw demo ui texture failed", slog.Any("err", err))
		}
	}

	// 临时黄昏方向光参数，用于验证屏幕空间明暗渐变效果
	// 方向让光从左上铺入画面，暖橙色负责黄昏色调，少量 MiddayBlend 避免无环境光时暗部完全死黑
	a.renderer.AddDirectionalLight(mgl32.Vec2{-0.7, -1.0}, &render.DirectionalLightOptions{
		Color:       mgl32.Vec4{1.0, 0.56, 0.30, 1.0},
		Intensity:   0.9,
		Offset:      0.20,
		Softness:    0.44,
		MiddayBlend: 0.08,
	})
	// 点光源测试矩形，配合点光源检查光源中心和半径覆盖范围
	a.renderer.DrawWorldRect(mgl32.Vec4{266.0, 134.0, 36.0, 36.0}, mgl32.Vec4{1.0, 1.0, 1.0, 1.0})
	// 临时点光源参数，对准右下角测试矩形中心，radius 接近矩形半对角线，用于验证 position 和 radius 是否生效
	a.renderer.AddWorldPointLight(mgl32.Vec2{284.0, 152.0}, 18.0, &render.PointLightOptions{
		Color:     mgl32.Vec4{1.0, 0.72, 0.34, 1.0},
		Intensity: 1.0,
	})
	// 聚光灯测试矩形，配合聚光灯检查光源中心和半径覆盖范围
	a.renderer.DrawWorldRect(
		mgl32.Vec4{82.0, 72.0, 64.0, 64.0},
		mgl32.Vec4{1.0, 1.0, 1.0, 1.0},
	)
	// 临时聚光灯参数，中心对准测试矩形中心，方向水平向右
	a.renderer.AddWorldSpotLight(mgl32.Vec2{114.0, 104.0}, 32.0, mgl32.Vec2{1.0, 0.0}, &render.SpotLightOptions{
		Color:         mgl32.Vec4{1.0, 0.82, 0.48, 1.0},
		Intensity:     0.75,
		InnerAngleDeg: 16.0,
		OuterAngleDeg: 34.0,
	})
	// 临时自发光矩形，独立于光照乘法合成，用于验证 EmissivePass 和 CompositePass 输入
	if err := a.renderer.DrawWorldEmissiveRect(mgl32.Vec4{232.0, 132.0, 24.0, 16.0}, mgl32.Vec4{0.0, 0.75, 1.0, 1.0}); err != nil {
		slog.Error("draw demo emissive rect failed", slog.Any("err", err))
	}

	a.renderer.DrawWorldRectOutline(mgl32.Vec4{32.0, 32.0, 96.0, 64.0}, 2.0, mgl32.Vec4{1.0, 0.95, 0.45, 1.0})
	a.renderer.DrawWorldLine(mgl32.Vec2{24.0, 148.0}, mgl32.Vec2{164.0, 116.0}, 1.0, mgl32.Vec4{0.15, 0.95, 0.55, 1.0})
	a.renderer.DrawWorldCircleOutline(mgl32.Vec2{184.0, 132.0}, 18.0, 3.0, mgl32.Vec4{0.0, 1.0, 0.2, 1.0})

	a.renderer.Present()
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
	if a.renderer != nil {
		slog.Debug("render stats", slog.Any("stats", a.renderer.RenderStats()))
	}

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

	if err := a.initGameState(); err != nil {
		return err
	}

	if err := a.initCamera(); err != nil {
		return err
	}

	if err := a.initGLRenderer(); err != nil {
		return err
	}

	if err := a.initResourceManager(); err != nil {
		return err
	}

	if err := a.initInputManager(); err != nil {
		return err
	}

	// 注册退出事件
	dispatch.SinkOf[event.QuitEvent](a.dispatcher).Connect(a.onQuitEvent)
	// 注册窗口大小变化事件：更新 OpenGL 渲染器视口
	dispatch.SinkOf[event.WindowResizedEvent](a.dispatcher).Connect(a.onWindowResizedEvent)

	a.isRunning = true

	slog.Debug(
		"game app init",
		slog.Bool("isRunning", a.isRunning),
	)
	return nil
}

// 处理退出事件
func (a *GameApp) onQuitEvent(event.QuitEvent) {
	slog.Info("quit event received, game app exit")
	a.isRunning = false
}

// 处理窗口大小变化事件
func (a *GameApp) onWindowResizedEvent(event event.WindowResizedEvent) {
	// 使用像素大小更新 OpenGL 视口（高 DPI 下，window 坐标尺寸 ≠ drawable 像素尺寸）
	w := event.Width
	h := event.Height
	if !event.PixelSizeChanged {
		// 将窗口坐标转换为像素坐标（高DPI）
		sdl.GetWindowSizeInPixels(a.window, &w, &h)
	}
	if a.renderer != nil {
		a.renderer.Resize(w, h)
	}
}

// 初始化游戏状态
func (a *GameApp) initGameState() error {
	a.gameState = NewGameState(a.window, abstract.GameStateTitle)
	if a.gameState == nil {
		return errors.New("create game state failed")
	}
	if a.config != nil {
		a.gameState.SetLogicalSize(a.logicalRenderSize())
	}
	slog.Debug("game state init success")
	return nil
}

// 初始化当前阶段使用的世界相机
func (a *GameApp) initCamera() error {
	logicalSize := a.logicalRenderSize()
	a.camera = render.NewCamera(logicalSize)
	// 当前阶段还没有地图中心、出生点或跟随目标来决定初始视角
	// 先把相机中心放到逻辑画布中心附近，使默认可视区域从世界原点附近开始
	// 这只是骨架阶段的临时默认值，后续应由场景或地图装配决定相机初始位置
	a.camera.SetPosition(logicalSize.Mul(0.5))
	slog.Debug("camera init success", slog.Any("logicalSize", logicalSize))
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
	a.dispatcher = dispatch.NewDispatcher()
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
	// 窗口缩放只影响窗口初始大小，逻辑缩放决定渲染质量
	logicalSize := a.logicalRenderSize()
	renderer, err := render.NewRenderer(a.window, logicalSize, "config/render.json")
	if err != nil {
		return err
	}
	a.renderer = renderer
	a.renderer.SetVSyncEnabled(a.config.Graphics.Vsync)
	a.renderer.SetViewportClippingEnabled(true)
	a.renderer.SetPixelSnapEnabled(true)
	a.renderer.SetPointLightEnabled(true)
	a.renderer.SetSpotLightEnabled(true)
	a.renderer.SetDirectionalLightEnabled(true)
	a.renderer.SetBloomEnabled(true)
	if err := a.renderer.SetBloomLevelCount(4); err != nil {
		return err
	}
	a.renderer.SetBloomSigma(2.0)
	// a.renderer.SetAmbientLightColor(mgl32.Vec4{0.0, 0.0, 0.0, 1.0})
	// 低亮度暖灰环境光，适合作为黄昏默认底光，只抬暗部，不明显改变整体色调
	a.renderer.SetAmbientLightColor(mgl32.Vec4{0.10, 0.085, 0.075, 1.0})
	slog.Debug("open gl renderer success", slog.Any("logicalSize", logicalSize))
	slog.Debug("render debug textures", slog.Any("textures", a.renderer.DebugTextures()))
	return nil
}

// 初始化资源管理器并加载启动阶段资源
func (a *GameApp) initResourceManager() error {
	manager, err := resource.NewResourceManager(a.renderer)
	if err != nil {
		return err
	}
	if err := manager.LoadResources("assets/data/resource_mapping.json"); err != nil {
		manager.Clear()
		return err
	}

	demoTexture, err := manager.LoadTexture(resource.ResourceKey("Button Normal.png"), "assets/tests/Button Normal.png")
	if err != nil {
		manager.Clear()
		return err
	}

	a.resourceManager = manager
	a.demoTexture = demoTexture
	slog.Debug("resource manager init success")
	slog.Debug("resource texture debug info", slog.Any("textures", manager.TextureDebugInfo()))
	slog.Debug("resource sound debug info", slog.Any("sounds", manager.SoundDebugInfo()))
	slog.Debug("resource music debug info", slog.Any("music", manager.MusicDebugInfo()))
	return nil
}

func (a *GameApp) logicalRenderSize() mgl32.Vec2 {
	if a == nil || a.config == nil {
		return mgl32.Vec2{}
	}
	return mgl32.Vec2{
		float32(a.config.Window.Width) * a.config.Window.LogicalScale,
		float32(a.config.Window.Height) * a.config.Window.LogicalScale,
	}
}

// 初始化输入管理器
func (a *GameApp) initInputManager() error {
	a.inputManager = input.NewInputManager(a.dispatcher, a.window, a.gameState)
	slog.Debug("input manager init success")
	return nil
}

// 释放应用持有的运行时资源
func (a *GameApp) close() {
	if a.isRunning {
		slog.Warn("game app is running, close")
	}

	slog.Debug("game app closed", slog.Bool("isRunning", a.isRunning))

	a.inputManager = nil

	a.demoTexture = nil

	if a.resourceManager != nil {
		a.resourceManager.Clear()
		a.resourceManager = nil
	}

	if a.renderer != nil {
		a.renderer.Close()
		a.renderer = nil
	}

	a.camera = nil

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
