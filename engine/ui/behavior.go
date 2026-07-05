package ui

import (
	"github.com/go-gl/mathgl/mgl32"
)

// 定义可挂载到交互元素上的扩展输入行为
//
// 由 UIInteractive 在对应鼠标事件和拖拽阶段调用，具体行为可按需响应
type IInteractionBehavior interface {
	// 在行为挂载到交互元素时调用
	OnAttach(*UIInteractive)
	// 在鼠标进入元素范围时调用
	OnHoverEnter(*UIInteractive)
	// 在鼠标离开元素范围时调用
	OnHoverExit(*UIInteractive)
	// 在鼠标按键于元素上按下时调用
	OnPressed(*UIInteractive)
	// 在鼠标按键释放时调用，第二个参数表示是否在原元素内释放
	OnReleased(*UIInteractive, bool)
	// 在鼠标按键于原元素内释放并成立点击时调用
	OnClick(*UIInteractive)
	// 在元素开始跟踪潜在拖拽时调用，位置使用逻辑屏幕坐标
	OnDragBegin(*UIInteractive, mgl32.Vec2)
	// 在拖拽位置变化时调用，参数依次为当前位置和本次位移
	OnDragUpdate(*UIInteractive, mgl32.Vec2, mgl32.Vec2)
	// 在拖拽跟踪结束时调用，最后一个参数表示是否在原元素内释放
	OnDragEnd(*UIInteractive, mgl32.Vec2, bool)
}

// 提供全部交互行为方法的空实现，具体行为可通过嵌入按需覆盖
type BaseInteractionBehavior struct{}

// 默认不处理行为挂载事件
func (BaseInteractionBehavior) OnAttach(*UIInteractive) {}

// 默认不处理鼠标进入事件
func (BaseInteractionBehavior) OnHoverEnter(*UIInteractive) {}

// 默认不处理鼠标离开事件
func (BaseInteractionBehavior) OnHoverExit(*UIInteractive) {}

// 默认不处理鼠标按下事件
func (BaseInteractionBehavior) OnPressed(*UIInteractive) {}

// 默认不处理鼠标释放事件
func (BaseInteractionBehavior) OnReleased(*UIInteractive, bool) {}

// 默认不处理点击成立事件
func (BaseInteractionBehavior) OnClick(*UIInteractive) {}

// 默认不处理拖拽开始事件
func (BaseInteractionBehavior) OnDragBegin(*UIInteractive, mgl32.Vec2) {}

// 默认不处理拖拽更新事件
func (BaseInteractionBehavior) OnDragUpdate(*UIInteractive, mgl32.Vec2, mgl32.Vec2) {}

// 默认不处理拖拽结束事件
func (BaseInteractionBehavior) OnDragEnd(*UIInteractive, mgl32.Vec2, bool) {}

// 通过回调扩展任意交互事件
type CallbackBehavior struct {
	// 提供未关注事件的默认空实现
	BaseInteractionBehavior
	// 行为挂载到元素时触发的回调
	OnAttachFunc func(*UIInteractive)
	// 鼠标进入元素范围时触发的回调
	OnHoverEnterFunc func(*UIInteractive)
	// 鼠标离开元素范围时触发的回调
	OnHoverExitFunc func(*UIInteractive)
	// 鼠标按键于元素上按下时触发的回调
	OnPressedFunc func(*UIInteractive)
	// 鼠标按键释放时触发的回调
	OnReleasedFunc func(*UIInteractive, bool)
	// 鼠标按键于原元素内释放并成立点击时触发的回调
	OnClickFunc func(*UIInteractive)
	// 开始跟踪拖拽时触发的回调
	OnDragBeginFunc func(*UIInteractive, mgl32.Vec2)
	// 拖拽位置变化时触发的回调
	OnDragUpdateFunc func(*UIInteractive, mgl32.Vec2, mgl32.Vec2)
	// 拖拽结束时触发的回调
	OnDragEndFunc func(*UIInteractive, mgl32.Vec2, bool)
}

// 在行为挂载时调用已配置回调
func (b *CallbackBehavior) OnAttach(owner *UIInteractive) {
	if b != nil && b.OnAttachFunc != nil {
		b.OnAttachFunc(owner)
	}
}

// 在鼠标进入时调用已配置回调
func (b *CallbackBehavior) OnHoverEnter(owner *UIInteractive) {
	if b != nil && b.OnHoverEnterFunc != nil {
		b.OnHoverEnterFunc(owner)
	}
}

// 在鼠标离开时调用已配置回调
func (b *CallbackBehavior) OnHoverExit(owner *UIInteractive) {
	if b != nil && b.OnHoverExitFunc != nil {
		b.OnHoverExitFunc(owner)
	}
}

// 在鼠标按下时调用已配置回调
func (b *CallbackBehavior) OnPressed(owner *UIInteractive) {
	if b != nil && b.OnPressedFunc != nil {
		b.OnPressedFunc(owner)
	}
}

// 在鼠标释放时调用已配置回调
func (b *CallbackBehavior) OnReleased(owner *UIInteractive, inside bool) {
	if b != nil && b.OnReleasedFunc != nil {
		b.OnReleasedFunc(owner, inside)
	}
}

// 在点击成立时调用已配置回调
func (b *CallbackBehavior) OnClick(owner *UIInteractive) {
	if b != nil && b.OnClickFunc != nil {
		b.OnClickFunc(owner)
	}
}

// 在拖拽开始时调用已配置回调
func (b *CallbackBehavior) OnDragBegin(owner *UIInteractive, position mgl32.Vec2) {
	if b != nil && b.OnDragBeginFunc != nil {
		b.OnDragBeginFunc(owner, position)
	}
}

// 在拖拽更新时调用已配置回调
func (b *CallbackBehavior) OnDragUpdate(owner *UIInteractive, position, delta mgl32.Vec2) {
	if b != nil && b.OnDragUpdateFunc != nil {
		b.OnDragUpdateFunc(owner, position, delta)
	}
}

// 在拖拽结束时调用已配置回调
func (b *CallbackBehavior) OnDragEnd(owner *UIInteractive, position mgl32.Vec2, accepted bool) {
	if b != nil && b.OnDragEndFunc != nil {
		b.OnDragEndFunc(owner, position, accepted)
	}
}

// 通过回调扩展鼠标进入和离开行为
type HoverBehavior struct {
	// 提供未关注事件的默认空实现
	BaseInteractionBehavior
	// 鼠标进入元素范围时触发的回调
	OnEnter func(*UIInteractive)
	// 鼠标离开元素范围时触发的回调
	OnExit func(*UIInteractive)
}

// 确保 HoverBehavior 实现 IInteractionBehavior 接口
var _ IInteractionBehavior = (*HoverBehavior)(nil)

// 在鼠标进入时调用已配置回调
func (b *HoverBehavior) OnHoverEnter(owner *UIInteractive) {
	if b != nil && b.OnEnter != nil {
		b.OnEnter(owner)
	}
}

// 在鼠标离开时调用已配置回调
func (b *HoverBehavior) OnHoverExit(owner *UIInteractive) {
	if b != nil && b.OnExit != nil {
		b.OnExit(owner)
	}
}

// 通过回调扩展拖拽开始、更新和结束行为
type DragBehavior struct {
	// 提供未关注事件的默认空实现
	BaseInteractionBehavior
	// 开始跟踪拖拽时触发，位置使用逻辑屏幕坐标
	OnBegin func(*UIInteractive, mgl32.Vec2)
	// 拖拽位置变化时触发，参数依次为当前位置和本次位移
	OnUpdate func(*UIInteractive, mgl32.Vec2, mgl32.Vec2)
	// 拖拽结束时触发，最后一个参数表示是否在原元素内释放
	OnEnd func(*UIInteractive, mgl32.Vec2, bool)
}

// 在鼠标按下并开始跟踪拖拽时调用已配置回调
func (b *DragBehavior) OnDragBegin(owner *UIInteractive, position mgl32.Vec2) {
	if b != nil && b.OnBegin != nil {
		b.OnBegin(owner, position)
	}
}

// 在按下期间鼠标位置变化时调用已配置回调
func (b *DragBehavior) OnDragUpdate(owner *UIInteractive, position, delta mgl32.Vec2) {
	if b != nil && b.OnUpdate != nil {
		b.OnUpdate(owner, position, delta)
	}
}

// 在鼠标释放并结束拖拽时调用已配置回调
func (b *DragBehavior) OnDragEnd(owner *UIInteractive, position mgl32.Vec2, inside bool) {
	if b != nil && b.OnEnd != nil {
		b.OnEnd(owner, position, inside)
	}
}
