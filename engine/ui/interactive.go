package ui

import (
	"log/slog"

	"tiny_farm/engine/abstract"
	"tiny_farm/engine/utils/defs"

	"github.com/go-gl/mathgl/mgl32"
)

// 表示可交互元素当前所处的交互状态
type UIState int

const (
	// 未发生鼠标交互时的默认状态
	UIStateNormal UIState = iota
	// 鼠标悬停在元素范围内的状态
	UIStateHover
	// 鼠标按键已按下的状态
	UIStatePressed
	// 元素已禁用且不响应交互的状态
	UIStateDisabled
)

const (
	// 悬停音效事件标识
	UISoundEventHover = "hover"
	// 点击音效事件标识
	UISoundEventClick = "click"
)

// 描述单个交互事件对默认音效的覆盖配置
type soundOverride struct {
	// 覆盖使用的音效资源键
	key defs.ResourceKey
	// 加载音效资源时使用的可选路径
	path string
	// 是否关闭该交互事件的音效
	disabled bool
}

// 管理可交互元素的状态、视觉、行为回调和音效反馈
//
// 当前使用状态对象处理交互迁移，并保留枚举状态作为对外查询和视觉映射
type UIInteractive struct {
	// 提供布局、渲染和鼠标事件挂接能力
	*UIElement
	// 当前交互状态
	state UIState
	// 当前状态对象，负责处理状态内的事件规则
	stateObject interactionState
	// 下一次更新时应用的状态对象
	nextState interactionState
	// 是否接受鼠标交互
	enabled bool
	// 图片集合，键通常使用四个 UIImage 常量
	images map[UIState]ImageSpec
	// 音效播放入口
	audio abstract.IAudioPlayer
	// 鼠标位置查询入口，用于拖拽行为
	input abstract.IActionInput
	// 交互状态变化时触发的回调
	onState func(UIState)
	// 按交互事件名称保存的音效覆盖配置
	soundEvents map[string]soundOverride
	// 挂载到元素上的扩展交互行为
	behaviors []IInteractionBehavior
	// 鼠标左键是否已在元素上按下
	pressed bool
	// 按下后是否已经发生位置变化
	dragging bool
	// 上一次处理拖拽时的鼠标位置
	lastMousePosition mgl32.Vec2
}

// 创建一个默认启用的可交互元素并挂接鼠标事件
//
// 可选输入源用于启用拖拽位置跟踪，不传时仍支持点击和悬停
func NewUIInteractive(position, size mgl32.Vec2, audio abstract.IAudioPlayer, inputs ...abstract.IActionInput) *UIInteractive {
	interactive := &UIInteractive{
		UIElement:   NewUIElement(position, size),
		state:       UIStateNormal,
		enabled:     true,
		images:      make(map[UIState]ImageSpec),
		audio:       audio,
		soundEvents: make(map[string]soundOverride),
	}
	interactive.stateObject = newInteractionState(UIStateNormal)
	if len(inputs) > 0 {
		interactive.input = inputs[0]
	}
	interactive.SetInteractive(true)
	interactive.SetMouseHandlers(
		interactive.MouseEnter,
		interactive.MouseExit,
		interactive.MousePressed,
		interactive.MouseReleased,
	)
	interactive.SetUpdateSelf(func(_ *UIElement, deltaTime float64) {
		interactive.updateState(deltaTime)
		interactive.updateDrag()
	})
	interactive.SetRenderUI(interactive.render)
	return interactive
}

// 返回当前交互状态，空对象按禁用状态处理
func (i *UIInteractive) State() UIState {
	if i == nil {
		return UIStateDisabled
	}
	return i.state
}

// 返回元素当前是否启用交互功能
func (i *UIInteractive) Enabled() bool {
	return i != nil && i.enabled
}

// 设置元素是否接受交互，并同步对应的交互状态
func (i *UIInteractive) SetEnabled(enabled bool) {
	if i == nil || i.enabled == enabled {
		return
	}
	i.enabled = enabled
	i.SetInteractive(enabled)
	i.pressed = false
	i.dragging = false
	i.nextState = nil
	if enabled {
		i.SetState(UIStateNormal)
	} else {
		i.SetState(UIStateDisabled)
	}
}

// 设置拖拽行为使用的鼠标位置查询入口
func (i *UIInteractive) SetInput(input abstract.IActionInput) {
	if i != nil {
		i.input = input
	}
}

// 设置交互状态变化时触发的回调
func (i *UIInteractive) SetStateCallback(callback func(UIState)) {
	if i != nil {
		i.onState = callback
	}
}

// 设置当前状态并同步状态图片和回调，同步
func (i *UIInteractive) SetState(state UIState) {
	if i == nil || i.state == state {
		return
	}
	i.nextState = nil
	i.setStateObject(newInteractionState(state))
}

// 设置下一次更新时应用的交互状态吗，延迟切换状态，比如状态机切换状态
func (i *UIInteractive) SetNextState(state UIState) {
	if i == nil {
		return
	}
	i.nextState = newInteractionState(state)
}

// 添加或替换指定标识的状态图片
//
// 元素尺寸为零时使用图片源区域的宽高作为请求尺寸
func (i *UIInteractive) AddImage(state UIState, image ImageSpec) {
	if i == nil {
		return
	}
	i.images[state] = image
	if i.Size() == (mgl32.Vec2{}) && image.SourceRect.Z() > 0 && image.SourceRect.W() > 0 {
		i.SetSize(mgl32.Vec2{image.SourceRect.Z(), image.SourceRect.W()})
	}
}

// 为指定交互事件设置音效资源键和可选加载路径
func (i *UIInteractive) SetSoundEvent(event string, key defs.ResourceKey, path string) {
	if i == nil || event == "" {
		return
	}
	i.soundEvents[event] = soundOverride{key: key, path: path}
}

// 通过资源路径设置指定交互事件的音效，空路径表示禁用
func (i *UIInteractive) SetSoundEventPath(event, path string) {
	if path == "" {
		i.DisableSoundEvent(event)
		return
	}
	i.SetSoundEvent(event, defs.ResourceKey(path), path)
}

// 关闭指定交互事件的音效反馈
func (i *UIInteractive) DisableSoundEvent(event string) {
	if i != nil && event != "" {
		i.soundEvents[event] = soundOverride{disabled: true}
	}
}

// 清除指定交互事件的音效覆盖并恢复默认映射
func (i *UIInteractive) ClearSoundEventOverride(event string) {
	if i != nil {
		delete(i.soundEvents, event)
	}
}

// 清除全部交互事件音效覆盖并恢复默认映射
func (i *UIInteractive) ClearSoundOverrides() {
	if i != nil {
		clear(i.soundEvents)
	}
}

// 设置悬停事件音效
func (i *UIInteractive) SetHoverSound(key defs.ResourceKey, path string) {
	i.SetSoundEvent(UISoundEventHover, key, path)
}

// 设置点击事件音效
func (i *UIInteractive) SetClickSound(key defs.ResourceKey, path string) {
	i.SetSoundEvent(UISoundEventClick, key, path)
}

// 关闭悬停事件音效
func (i *UIInteractive) DisableHoverSound() {
	i.DisableSoundEvent(UISoundEventHover)
}

// 关闭点击事件音效
func (i *UIInteractive) DisableClickSound() {
	i.DisableSoundEvent(UISoundEventClick)
}

// 清除悬停事件音效覆盖
func (i *UIInteractive) ClearHoverSoundOverride() {
	i.ClearSoundEventOverride(UISoundEventHover)
}

// 清除点击事件音效覆盖
func (i *UIInteractive) ClearClickSoundOverride() {
	i.ClearSoundEventOverride(UISoundEventClick)
}

// 播放指定交互事件对应的覆盖音效或默认音效
func (i *UIInteractive) PlaySoundEvent(event string) {
	if i == nil || i.audio == nil || event == "" {
		return
	}
	key := defaultSoundForEvent(event)
	var paths []string
	if override, ok := i.soundEvents[event]; ok {
		if override.disabled {
			return
		}
		key = override.key
		if override.path != "" {
			paths = append(paths, override.path)
		}
	}
	if key == "" {
		return
	}
	if err := i.audio.PlaySound(key, paths...); err != nil {
		slog.Warn("play ui sound failed", slog.String("event", event), slog.Any("err", err))
	}
}

// 挂载扩展交互行为并返回是否成功
func (i *UIInteractive) AddBehavior(behavior IInteractionBehavior) bool {
	if i == nil || behavior == nil {
		return false
	}
	behavior.OnAttach(i)
	i.behaviors = append(i.behaviors, behavior)
	return true
}

// 清除全部扩展交互行为
func (i *UIInteractive) ClearBehaviors() {
	if i != nil {
		i.behaviors = nil
	}
}

// 返回当前是否处于悬停状态
func (i *UIInteractive) IsHovered() bool {
	return i != nil && i.state == UIStateHover
}

// 返回当前是否处于按下状态
func (i *UIInteractive) IsPressed() bool {
	return i != nil && i.pressed
}

// 返回当前是否已发生拖拽移动
func (i *UIInteractive) IsDragging() bool {
	return i != nil && i.dragging
}

// 将屏幕坐标转换为相对父内容区域的局部坐标
func (i *UIInteractive) ScreenToLocal(screenPosition mgl32.Vec2) mgl32.Vec2 {
	if i == nil || i.Parent() == nil {
		return screenPosition
	}
	return screenPosition.Sub(i.Parent().ContentBounds().Position)
}

// 使用屏幕坐标设置元素的局部位置
func (i *UIInteractive) SetPositionByScreen(screenPosition mgl32.Vec2) {
	if i != nil {
		i.SetPosition(i.ScreenToLocal(screenPosition))
	}
}

// 处理鼠标进入事件并切换到悬停状态
func (i *UIInteractive) MouseEnter() {
	if i == nil || !i.enabled || i.stateObject == nil {
		return
	}
	// 因为状态迁移通过 SetNextState() 延迟保存，新的鼠标事件到来时，要先应用之前等待中的状态，否则可能由错误的状态对象处理事件。
	i.applyNextState()
	i.stateObject.onMouseEnter(i)
	for _, behavior := range i.behaviors {
		behavior.OnHoverEnter(i)
	}
}

// 处理鼠标离开事件，未按下时恢复到默认状态
func (i *UIInteractive) MouseExit() {
	if i == nil || !i.enabled || i.stateObject == nil {
		return
	}
	i.applyNextState()
	i.stateObject.onMouseExit(i)
	for _, behavior := range i.behaviors {
		behavior.OnHoverExit(i)
	}
}

// 处理鼠标按下事件并开始跟踪潜在拖拽
func (i *UIInteractive) MousePressed() {
	if i == nil || !i.enabled || i.stateObject == nil {
		return
	}
	i.applyNextState()
	i.pressed = true
	i.dragging = false
	i.lastMousePosition = i.mousePosition()
	i.stateObject.onMousePressed(i)
	for _, behavior := range i.behaviors {
		behavior.OnPressed(i)
		behavior.OnDragBegin(i, i.lastMousePosition)
	}
}

// 处理鼠标释放事件并结束拖拽，仅在元素范围内释放时触发点击
func (i *UIInteractive) MouseReleased(inside bool) {
	if i == nil || !i.enabled || i.stateObject == nil {
		return
	}
	i.applyNextState()
	position := i.mousePosition()
	for _, behavior := range i.behaviors {
		behavior.OnReleased(i, inside)
		behavior.OnDragEnd(i, position, inside)
	}
	i.pressed = false
	i.dragging = false
	i.stateObject.onMouseReleased(i, inside)
	if inside {
		for _, behavior := range i.behaviors {
			behavior.OnClick(i)
		}
	}
}

// 跟踪按下后的鼠标位移并通知拖拽行为
func (i *UIInteractive) updateDrag() {
	if i == nil || !i.enabled || !i.pressed || i.input == nil {
		return
	}
	position := i.input.LogicalMousePosition()
	delta := position.Sub(i.lastMousePosition)
	if delta == (mgl32.Vec2{}) {
		return
	}
	i.dragging = true
	for _, behavior := range i.behaviors {
		behavior.OnDragUpdate(i, position, delta)
	}
	i.lastMousePosition = position
}

// 应用等待中的交互状态并更新当前状态
func (i *UIInteractive) updateState(deltaTime float64) {
	if i == nil {
		return
	}
	i.applyNextState()
	if i.stateObject != nil {
		i.stateObject.update(i, deltaTime)
	}
}

// 应用等待中的交互状态
func (i *UIInteractive) applyNextState() {
	if i == nil {
		return
	}
	if i.nextState != nil {
		state := i.nextState
		i.nextState = nil
		i.setStateObject(state)
	}
}

// 绘制当前状态选择的图片
func (i *UIInteractive) render(uiCtx *uiContext) error {
	return i.drawStateImage(uiCtx)
}

// 返回输入源当前鼠标位置，未配置输入源时返回上一次位置
func (i *UIInteractive) mousePosition() mgl32.Vec2 {
	if i != nil && i.input != nil {
		return i.input.LogicalMousePosition()
	}
	if i == nil {
		return mgl32.Vec2{}
	}
	return i.lastMousePosition
}

// 返回事件对应的默认音效资源键
func defaultSoundForEvent(event string) defs.ResourceKey {
	switch event {
	case UISoundEventHover:
		return defs.ResourceKey("ui_hover")
	case UISoundEventClick:
		return defs.ResourceKey("ui_click")
	default:
		return ""
	}
}

// 绘制当前状态对应的图片，缺失时回退普通状态图片
func (i *UIInteractive) drawStateImage(uiCtx *uiContext) error {
	if i == nil {
		return nil
	}
	image, ok := i.images[i.state]
	if !ok && i.state != UIStateNormal {
		image, ok = i.images[UIStateNormal]
	}
	if !ok {
		return nil
	}
	return drawImageSpec(uiCtx, image, i.Bounds().RectToVec4())
}

// 立即切换到指定状态对象并同步公开状态
func (i *UIInteractive) setStateObject(state interactionState) {
	if i == nil || state == nil {
		return
	}
	stateID := state.id()
	if i.stateObject != nil && i.state == stateID {
		return
	}
	i.stateObject = state
	state.enter(i)
}

// 应用状态枚举并通知视觉更新回调
func (i *UIInteractive) applyState(state UIState) {
	if i == nil || i.state == state {
		return
	}
	i.state = state
	if i.onState != nil {
		i.onState(state)
	}
}
