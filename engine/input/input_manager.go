package input

import (
	"encoding/json"
	"log/slog"
	"maps"
	"os"
	"path/filepath"

	"tiny_farm/engine/abstract"
	"tiny_farm/engine/utils/defs"
	"tiny_farm/engine/utils/dispatch"
	"tiny_farm/engine/utils/event"
	emath "tiny_farm/engine/utils/math"

	"github.com/SunshineZzzz/purego-sdl3/sdl"
	"github.com/go-gl/mathgl/mgl32"
)

const (
	// 动作状态数量
	callbackStateCount = int(defs.Inactive)
	// 默认输入映射配置路径
	defaultConfigPath = "config/input.json"
)

// 管理 SDL 输入事件到游戏动作状态的转换
//
// 每帧先推进 Pressed/Held/Released 状态，再轮询 SDL 事件
// 最后按动作状态触发回调。鼠标逻辑坐标基于窗口尺寸和固定逻辑尺寸做 letterbox 映射
type InputManager struct {
	// 用于把 quit 和 resize 这类系统事件转发给引擎其他部分
	dispatcher *dispatch.Dispatcher
	// 用于查询当前窗口尺寸，计算鼠标逻辑坐标
	window *sdl.Window
	// 游戏状态
	gameState abstract.IGameState
	// 保存动作在 Pressed、Held、Released 三个阶段的回调信号
	actionsToFunc map[defs.ActionID][callbackStateCount]*dispatch.Signal[bool]
	// 保存每个动作当前所处的状态
	actionStates map[defs.ActionID]defs.ActionState
	// 保存本帧已经被回调消费的动作
	consumedActions map[defs.ActionID]bool
	// 保存键盘或鼠标输入到动作列表的映射
	inputToActions map[uint32][]defs.ActionID
	// 窗口坐标系中的鼠标位置
	mousePosition mgl32.Vec2
	// 映射到游戏逻辑坐标系后的鼠标位置
	logicalMousePosition mgl32.Vec2
	// 当前帧累计的鼠标滚轮变化
	mouseWheelDelta mgl32.Vec2
	// 预留给调试 UI 转发 SDL 事件
	eventForwarder func(*sdl.Event)
}

// 确保 InputManager 实现 IActionInput 接口
var _ abstract.IActionInput = (*InputManager)(nil)

// 解析输入映射配置文件
type inputConfig struct {
	InputMappings map[string][]string `json:"input_mappings"`
}

// 创建输入管理器并加载输入配置
//
// 如果配置文件缺失或解析失败，会回退到内置默认映射，保证客户端仍能接收基础输入
func NewInputManager(dispatcher *dispatch.Dispatcher, window *sdl.Window, gameState abstract.IGameState, configPath ...string) *InputManager {
	manager := &InputManager{
		dispatcher:      dispatcher,
		window:          window,
		gameState:       gameState,
		actionsToFunc:   make(map[defs.ActionID][callbackStateCount]*dispatch.Signal[bool]),
		actionStates:    make(map[defs.ActionID]defs.ActionState),
		consumedActions: make(map[defs.ActionID]bool),
		inputToActions:  make(map[uint32][]defs.ActionID),
		mouseWheelDelta: mgl32.Vec2{},
	}

	path := defaultConfigPath
	if len(configPath) > 0 {
		path = configPath[0]
	}
	if err := manager.loadConfig(path); err != nil {
		slog.Warn("input config load failed, use default mappings", slog.String("path", path), slog.Any("err", err))
		manager.initializeMappings(defaultMappings())
	}

	if window != nil {
		var x, y float32
		sdl.GetMouseState(&x, &y)
		manager.mousePosition = mgl32.Vec2{x, y}
		manager.recalculateLogicalMousePosition()
	}

	slog.Debug("input manager created", slog.String("path", path))
	return manager
}

// 返回指定动作状态的回调注册入口
//
// Inactive 不是可触发阶段；传入 Inactive 时会回退到 Pressed，避免调用方绑定到无效列表
func (m *InputManager) OnAction(actionID defs.ActionID, state defs.ActionState) *defs.ActionSink {
	if m == nil {
		return nil
	}
	if state == defs.Inactive || state < defs.Pressed || state >= defs.Inactive {
		slog.Warn("invalid action callback state, fallback to Pressed", slog.Int("state", int(state)))
		state = defs.Pressed
	}

	signals := m.actionsToFunc[actionID]
	if signals[state] == nil {
		signals[state] = &dispatch.Signal[bool]{}
		m.actionsToFunc[actionID] = signals
	}

	return signals[state].Sink()
}

// 更新输入状态并处理当前帧的全部 SDL 事件
//
// 每帧应在游戏逻辑前调用，这样 Pressed 和 Released 这类瞬时状态只持续一帧
func (m *InputManager) Update() {
	if m == nil {
		return
	}

	m.advanceActionStates()
	clear(m.consumedActions)
	m.mouseWheelDelta = mgl32.Vec2{}

	var event sdl.Event
	for sdl.PollEvent(&event) {
		if m.eventForwarder != nil {
			m.eventForwarder(&event)
		}
		m.processEvent(&event)
	}

	m.dispatchActionCallbacks()
}

// 触发应用退出事件
func (m *InputManager) Quit() {
	if m == nil || m.dispatcher == nil {
		return
	}

	m.dispatcher.Trigger(event.QuitEvent{})
}

// 返回动作当前是否处于按下或持续按下状态
func (m *InputManager) IsActionDown(actionID defs.ActionID) bool {
	if m == nil {
		return false
	}

	state := m.actionStates[actionID]
	return (state == defs.Pressed || state == defs.Held) && !m.consumedActions[actionID]
}

// 返回动作是否在本帧刚按下
func (m *InputManager) IsActionPressed(actionID defs.ActionID) bool {
	if m == nil {
		return false
	}

	return m.actionStates[actionID] == defs.Pressed && !m.consumedActions[actionID]
}

// 返回动作是否在本帧刚释放
func (m *InputManager) IsActionReleased(actionID defs.ActionID) bool {
	if m == nil {
		return false
	}

	return m.actionStates[actionID] == defs.Released && !m.consumedActions[actionID]
}

// 返回窗口坐标系中的鼠标位置
func (m *InputManager) MousePosition() mgl32.Vec2 {
	if m == nil {
		return mgl32.Vec2{}
	}

	return m.mousePosition
}

// 返回游戏逻辑坐标系中的鼠标位置
func (m *InputManager) LogicalMousePosition() mgl32.Vec2 {
	if m == nil {
		return mgl32.Vec2{}
	}

	return m.logicalMousePosition
}

// 返回当前帧鼠标滚轮变化量
func (m *InputManager) MouseWheelDelta() mgl32.Vec2 {
	if m == nil {
		return mgl32.Vec2{}
	}

	return m.mouseWheelDelta
}

// 设置 SDL 事件转发回调
//
// 当前用于给后续调试 UI 预留事件注入点
func (m *InputManager) SetEventForwarder(callback func(*sdl.Event)) {
	if m == nil {
		return
	}

	m.eventForwarder = callback
}

// 返回动作状态快照，供调试面板读取
func (m *InputManager) ActionStatesDebug() map[defs.ActionID]defs.ActionState {
	if m == nil {
		return nil
	}

	states := make(map[defs.ActionID]defs.ActionState, len(m.actionStates))
	maps.Copy(states, m.actionStates)
	// for actionID, state := range m.actionStates {
	// 	states[actionID] = state
	// }
	return states
}

// 手动设置动作状态
//
// 该方法只用于调试入口，允许调试面板临时触发某个动作状态
func (m *InputManager) SetActionStateDebug(actionID defs.ActionID, state defs.ActionState) {
	if m == nil {
		return
	}
	if _, ok := m.actionStates[actionID]; !ok {
		slog.Warn("set unregistered action state ignored", slog.String("actionID", string(actionID)))
		return
	}

	m.actionStates[actionID] = state
}

// 更新所有动作状态，将 Pressed 转换为 Held，Released 转换为 Inactive
func (m *InputManager) advanceActionStates() {
	for actionID, state := range m.actionStates {
		switch state {
		case defs.Pressed:
			m.actionStates[actionID] = defs.Held
		case defs.Released:
			m.actionStates[actionID] = defs.Inactive
		}
	}
}

// 触发所有动作状态回调
func (m *InputManager) dispatchActionCallbacks() {
	for actionID, state := range m.actionStates {
		if state == defs.Inactive {
			continue
		}

		if state < defs.Pressed || state >= defs.Inactive {
			continue
		}

		signals := m.actionsToFunc[actionID]
		if signals[state] == nil {
			continue
		}
		signals[state].Collect(func(result bool) bool {
			if result {
				m.consumedActions[actionID] = true
			}
			return result
		})
	}
}

// 处理 SDL 事件，更新动作状态
func (m *InputManager) processEvent(event *sdl.Event) {
	switch event.Type() {
	case sdl.EventKeyDown, sdl.EventKeyUp:
		scancode := event.Key().Scancode
		isDown := event.Key().Down
		isRepeat := event.Key().Repeat
		m.updateActionsForInput(uint32(scancode), isDown, isRepeat)
	case sdl.EventMouseButtonDown, sdl.EventMouseButtonUp:
		button := event.Button().Button
		isDown := event.Button().Down
		// 鼠标事件不考虑repeat, 所以第三个参数传false
		m.updateActionsForInput(uint32(button), isDown, false)
		// 在点击时更新鼠标位置，同时更新逻辑位置
		m.mousePosition = mgl32.Vec2{event.Button().X, event.Button().Y}
		m.recalculateLogicalMousePosition()
	case sdl.EventMouseMotion:
		// 处理鼠标运动
		motion := event.Motion()
		m.mousePosition = mgl32.Vec2{motion.X, motion.Y}
		m.recalculateLogicalMousePosition()
	case sdl.EventMouseWheel:
		// 处理鼠标滚轮
		wheel := event.Wheel()
		m.mouseWheelDelta = mgl32.Vec2{wheel.X, wheel.Y}
	case sdl.EventWindowResized:
		// SDL3: 窗口大小改变(逻辑坐标)
		window := event.Window()
		m.triggerWindowResized(window.Data1, window.Data2, false)
		m.recalculateLogicalMousePosition()
	case sdl.EventWindowPixelSizeChanged:
		// // 高DPI：像素大小改变(逻辑坐标)
		window := event.Window()
		m.triggerWindowResized(window.Data1, window.Data2, true)
		m.recalculateLogicalMousePosition()
	case sdl.EventQuit, sdl.EventWindowCloseRequested:
		m.Quit()
	}
}

// 根据输入更新动作状态
func (m *InputManager) updateActionsForInput(input uint32, isActive bool, isRepeat bool) {
	actionIDs := m.inputToActions[input]
	for _, actionID := range actionIDs {
		m.updateActionState(actionID, isActive, isRepeat)
	}
}

// 更新动作状态
func (m *InputManager) updateActionState(actionID defs.ActionID, isActive bool, isRepeat bool) {
	state, ok := m.actionStates[actionID]
	if !ok {
		slog.Warn("update unregistered action ignored", slog.String("actionID", string(actionID)))
		return
	}

	if isActive {
		if isRepeat {
			m.actionStates[actionID] = defs.Held
			return
		}
		m.actionStates[actionID] = defs.Pressed
		return
	}

	// 如果从未按下却收到松开事件（比如 ImGui 捕获了按下事件但没捕获松开事件，或者窗口焦点切换），
	// 就跳过 RELEASED 状态，避免产生一次"幽灵释放"
	if state != defs.Inactive {
		m.actionStates[actionID] = defs.Released
	}
}

// 触发窗口大小改变事件，逻辑坐标
func (m *InputManager) triggerWindowResized(width int32, height int32, pixelSizeChanged bool) {
	if m.dispatcher == nil {
		return
	}

	m.dispatcher.Trigger(event.WindowResizedEvent{
		Width:            width,
		Height:           height,
		PixelSizeChanged: pixelSizeChanged,
	})
}

// 加载输入配置文件
func (m *InputManager) loadConfig(configPath string) error {
	if configPath == "" {
		return os.ErrNotExist
	}

	data, err := os.ReadFile(filepath.Clean(configPath))
	if err != nil {
		return err
	}

	var withRoot inputConfig
	if err := json.Unmarshal(data, &withRoot); err != nil {
		return err
	}
	if len(withRoot.InputMappings) > 0 {
		m.initializeMappings(withRoot.InputMappings)
		return nil
	}

	var root map[string][]string
	if err := json.Unmarshal(data, &root); err != nil {
		return err
	}
	if len(root) == 0 {
		return os.ErrInvalid
	}

	m.initializeMappings(root)
	return nil
}

// 初始化按键映射
func (m *InputManager) initializeMappings(actionsToKeyName map[string][]string) {
	if _, ok := actionsToKeyName["mouse_left"]; !ok {
		actionsToKeyName["mouse_left"] = []string{"MouseLeft"}
	}
	if _, ok := actionsToKeyName["mouse_right"]; !ok {
		actionsToKeyName["mouse_right"] = []string{"MouseRight"}
	}

	clear(m.inputToActions)
	clear(m.actionStates)

	for actionName, keyNames := range actionsToKeyName {
		actionID := defs.ActionID(actionName)
		m.actionStates[actionID] = defs.Inactive

		for _, keyName := range keyNames {
			scancode := sdl.GetScancodeFromName(keyName)
			mouseButton := mouseButtonFromString(keyName)
			if scancode != sdl.ScancodeUnknown {
				m.inputToActions[uint32(scancode)] = append(m.inputToActions[uint32(scancode)], actionID)
				slog.Debug("map action to keyboard scancode", slog.String("action", actionName), slog.String("keyName", keyName),
					slog.Any("scancode", scancode))
				continue
			}
			if mouseButton != 0 {
				m.inputToActions[uint32(mouseButton)] = append(m.inputToActions[uint32(mouseButton)], actionID)
				slog.Debug("map action to mouse button", slog.String("action", actionName), slog.String("keyName", keyName),
					slog.Any("mouseButton", mouseButton))
				continue
			}
			slog.Warn("unknown input mapping ignored", slog.String("keyName", keyName), slog.String("action", actionName))
		}
	}
}

// 计算逻辑鼠标位置
func (m *InputManager) recalculateLogicalMousePosition() {
	if m.gameState == nil {
		m.logicalMousePosition = m.mousePosition
		return
	}

	// SDL 的鼠标坐标（event.motion.x/y、SDL_GetMouseState）以“window coordinates”为单位，
	// 对应 SDL_GetWindowSize 返回的窗口坐标尺寸；高 DPI 下该尺寸可能不同于实际像素尺寸。
	// 渲染侧（OpenGL viewport）使用 SDL_GetWindowSizeInPixels，因此这里必须用 window size
	// 来做 letterbox 映射，才能和鼠标输入坐标保持同一坐标系。
	windowSize := m.gameState.GetWindowSize()
	logicalSize := m.gameState.GetLogicalSize()
	metrics := emath.ComputeLetterboxMetrics(windowSize, logicalSize)
	if metrics.Scale <= 0.0 {
		m.logicalMousePosition = m.mousePosition
		return
	}

	// 输入(鼠标)最终要映射到Logical
	local := m.mousePosition.Sub(metrics.Viewport.Position)
	logical := local.Mul(1 / metrics.Scale)

	logical[0] = emath.Clamp(logical.X(), 0.0, logicalSize.X())
	logical[1] = emath.Clamp(logical.Y(), 0.0, logicalSize.Y())
	m.logicalMousePosition = logical
}

// 根据按键名称获取鼠标按钮值
func mouseButtonFromString(buttonName string) sdl.MouseButtonFlags {
	switch buttonName {
	case "MouseLeft":
		return sdl.ButtonLeft
	case "MouseMiddle":
		return sdl.ButtonMiddle
	case "MouseRight":
		return sdl.ButtonRight
	case "MouseX1":
		return sdl.ButtonX1
	case "MouseX2":
		return sdl.ButtonX2
	default:
		return 0
	}
}

// 默认按键映射
func defaultMappings() map[string][]string {
	return map[string][]string{
		"move_left":  {"A", "Left"},
		"move_right": {"D", "Right"},
		"move_up":    {"W", "Up"},
		"move_down":  {"S", "Down"},
		"jump":       {"J", "Space"},
		"attack":     {"K", "MouseLeft"},
		"pause":      {"P", "Escape"},
	}
}
