package ui

// 定义单个交互状态的行为规则
type interactionState interface {
	// 返回状态对应的公开枚举值
	id() UIState
	// 进入状态时执行逻辑
	enter(*UIInteractive)
	// 每帧更新状态内部逻辑
	update(*UIInteractive, float64)
	// 处理鼠标进入事件
	onMouseEnter(*UIInteractive)
	// 处理鼠标离开事件
	onMouseExit(*UIInteractive)
	// 处理鼠标按下事件
	onMousePressed(*UIInteractive)
	// 处理鼠标释放事件
	onMouseReleased(*UIInteractive, bool)
}

// 提供状态对象的默认空行为
type baseInteractionState struct {
}

// 确保 baseInteractionState 实现 interactionState 接口
var _ interactionState = (*baseInteractionState)(nil)

// 返回状态对应的公开枚举值
func (baseInteractionState) id() UIState {
	panic("unimplemented")
}

// 进入状态时执行逻辑
func (baseInteractionState) enter(owner *UIInteractive) {
	panic("unimplemented")
}

// 每帧更新状态内部逻辑
func (baseInteractionState) update(*UIInteractive, float64) {
}

// 处理鼠标进入事件
func (baseInteractionState) onMouseEnter(*UIInteractive) {
}

// 处理鼠标离开事件
func (baseInteractionState) onMouseExit(*UIInteractive) {
}

// 处理鼠标按下事件
func (baseInteractionState) onMousePressed(*UIInteractive) {
}

// 处理鼠标释放事件
func (baseInteractionState) onMouseReleased(*UIInteractive, bool) {
}

// 非接口函数

// 应用状态到交互元素
func (baseInteractionState) apply(owner *UIInteractive, state UIState) {
	owner.applyState(state)
}

// 设置下一个状态，延迟
func (baseInteractionState) queue(owner *UIInteractive, state UIState) {
	owner.SetNextState(state)
}

// 播放音效事件
func (baseInteractionState) sound(owner *UIInteractive, event string) {
	owner.PlaySoundEvent(event)
}

// 普通状态处理从静止到悬停的迁移
type normalInteractionState struct {
	// 继承基础交互状态
	baseInteractionState
}

// 返回状态对应的公开枚举值
func (normalInteractionState) id() UIState {
	return UIStateNormal
}

// 进入状态时执行逻辑
func (s normalInteractionState) enter(owner *UIInteractive) {
	s.apply(owner, UIStateNormal)
}

// 处理鼠标进入事件
func (s normalInteractionState) onMouseEnter(owner *UIInteractive) {
	// 这里播放声音合理，原因如下：
	// Normal → Hover：鼠标进入，应该播放
	// Pressed → Hover：鼠标在按钮内释放，不应该再次播放
	// 外部 SetState(Hover)：不一定应该播放
	s.sound(owner, UISoundEventHover)
	s.queue(owner, UIStateHover)
}

// 悬停状态处理离开和按下迁移
type hoverInteractionState struct {
	// 继承基础交互状态
	baseInteractionState
}

// 返回状态对应的公开枚举值
func (hoverInteractionState) id() UIState {
	return UIStateHover
}

// 进入状态时执行逻辑
func (s hoverInteractionState) enter(owner *UIInteractive) {
	s.apply(owner, UIStateHover)
}

// 处理鼠标离开事件
func (s hoverInteractionState) onMouseExit(owner *UIInteractive) {
	s.queue(owner, UIStateNormal)
}

// 处理鼠标按下事件
func (s hoverInteractionState) onMousePressed(owner *UIInteractive) {
	s.queue(owner, UIStatePressed)
}

// 按下状态处理释放后的点击成立和取消
type pressedInteractionState struct {
	// 继承基础交互状态
	baseInteractionState
}

// 返回状态对应的公开枚举值
func (pressedInteractionState) id() UIState {
	return UIStatePressed
}

// 进入状态时执行逻辑
func (s pressedInteractionState) enter(owner *UIInteractive) {
	s.apply(owner, UIStatePressed)
	s.sound(owner, UISoundEventClick)
}

// 处理鼠标释放事件
func (s pressedInteractionState) onMouseReleased(owner *UIInteractive, inside bool) {
	if inside {
		s.queue(owner, UIStateHover)
		return
	}
	s.queue(owner, UIStateNormal)
}

// 禁用状态只同步禁用视觉，不响应鼠标事件
type disabledInteractionState struct {
	baseInteractionState
}

// 返回状态对应的公开枚举值
func (disabledInteractionState) id() UIState {
	return UIStateDisabled
}

// 进入状态时执行逻辑
func (s disabledInteractionState) enter(owner *UIInteractive) {
	s.apply(owner, UIStateDisabled)
}

// 创建指定枚举值对应的状态对象
func newInteractionState(state UIState) interactionState {
	switch state {
	case UIStateHover:
		return hoverInteractionState{}
	case UIStatePressed:
		return pressedInteractionState{}
	case UIStateDisabled:
		return disabledInteractionState{}
	default:
		return normalInteractionState{}
	}
}
