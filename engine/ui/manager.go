package ui

import (
	"tiny_farm/engine/abstract"
	"tiny_farm/engine/utils/defs"

	"github.com/go-gl/mathgl/mgl32"
)

// 管理单个场景持有的 UI 元素树
//
// 当前负责 root panel 生命周期、hover/pressed 目标记录、递归更新和递归渲染
type UIManager struct {
	// 根面板，作为所有场景 UI 元素的父容器
	root *Panel
	// 输入查询入口
	input abstract.IActionInput
	// 当前鼠标所在的命中元素
	hoveredElement *UIElement
	// 当前鼠标按下时命中的元素
	pressedElement *UIElement
	// 鼠标按下事件连接
	pressedConnection *defs.ActionConnection
	// 鼠标持续按住事件连接
	heldConnection *defs.ActionConnection
	// 鼠标释放事件连接
	releasedConnection *defs.ActionConnection
	// 是否已注册输入回调，未注册时由每帧更新主动查询输入状态
	usesCallbacks bool
}

// 创建 UI 管理器
func NewUIManager(rootSize mgl32.Vec2, input abstract.IActionInput) *UIManager {
	root := NewPanel(mgl32.Vec2{}, rootSize)
	manager := &UIManager{
		root:  root,
		input: input,
	}
	manager.registerMouseEvents()
	return manager
}

// 在输入源支持动作事件时注册鼠标状态回调
func (m *UIManager) registerMouseEvents() {
	m.pressedConnection = m.input.OnAction(defs.MouseLeftAction, defs.Pressed).Connect(m.onMousePressed)
	m.heldConnection = m.input.OnAction(defs.MouseLeftAction, defs.Held).Connect(m.onMouseHeld)
	m.releasedConnection = m.input.OnAction(defs.MouseLeftAction, defs.Released).Connect(m.onMouseReleased)
	m.usesCallbacks = true
}

// 返回根面板
func (m *UIManager) RootElement() *Panel {
	if m == nil {
		return nil
	}
	return m.root
}

// 返回当前 hover 命中的元素
func (m *UIManager) HoveredElement() *UIElement {
	if m == nil {
		return nil
	}
	return m.hoveredElement
}

// 返回当前 pressed 记录的元素
func (m *UIManager) PressedElement() *UIElement {
	if m == nil {
		return nil
	}
	return m.pressedElement
}

// 添加元素到根面板
func (m *UIManager) AddElement(element *UIElement, orderIndex ...int) bool {
	if m == nil || m.root == nil {
		return false
	}
	return m.root.AddChild(element, orderIndex...)
}

// 清空根面板下的全部元素并重置鼠标状态
func (m *UIManager) Clear() {
	if m == nil || m.root == nil {
		return
	}
	m.root.RemoveAllChildren()
	m.clearMouseState()
}

// 更新 UI 树和当前鼠标目标
func (m *UIManager) Update(deltaTime float64) {
	if m == nil || m.root == nil {
		return
	}
	m.processMouseState()
	m.root.Update(deltaTime)
}

// 渲染 UI 树
func (m *UIManager) Render(uiCtx *uiContext) error {
	if m == nil || m.root == nil {
		return nil
	}
	return m.root.RenderUI(uiCtx)
}

// 释放 manager 持有的场景 UI 状态
func (m *UIManager) Close() {
	if m == nil {
		return
	}
	if m.root != nil {
		m.root.RemoveAllChildren()
	}
	m.clearMouseState()
	m.pressedConnection.Release()
	m.heldConnection.Release()
	m.releasedConnection.Release()
	m.input = nil
}

// 根据鼠标位置更新悬停目标，未注册动作回调时再轮询左键按下和释放事件
func (m *UIManager) processMouseState() {
	if m.input == nil || !m.root.Visible() {
		m.clearMouseState()
		return
	}
	m.updateHovered(m.root.FindInteractiveAt(m.input.LogicalMousePosition()))
	if !m.usesCallbacks {
		if m.input.IsActionPressed(defs.MouseLeftAction) {
			m.onMousePressed()
		}
		if m.input.IsActionReleased(defs.MouseLeftAction) {
			m.onMouseReleased()
		}
	}
}

// 清除鼠标目标，并向原目标发送离开和取消释放事件
func (m *UIManager) clearMouseState() {
	if m.hoveredElement != nil {
		m.hoveredElement.handleMouseExit()
	}
	if m.pressedElement != nil {
		m.pressedElement.handleMouseReleased(false)
	}
	m.hoveredElement = nil
	m.pressedElement = nil
}

// 切换鼠标悬停目标并发送对应的进入和离开事件
func (m *UIManager) updateHovered(target *UIElement) {
	if target == m.hoveredElement {
		return
	}
	if m.hoveredElement != nil {
		m.hoveredElement.handleMouseExit()
	}
	m.hoveredElement = target
	if m.hoveredElement != nil {
		m.hoveredElement.handleMouseEnter()
	}
}

// 记录鼠标按下时命中的交互元素并触发按下事件
func (m *UIManager) onMousePressed() bool {
	if m == nil || m.root == nil || !m.root.Visible() {
		return false
	}
	target := m.root.FindInteractiveAt(m.input.LogicalMousePosition())
	if target == nil {
		m.pressedElement = nil
		return false
	}
	m.updateHovered(target)
	m.pressedElement = target
	target.handleMousePressed()
	return true
}

// 向按下时记录的元素发送释放结果
func (m *UIManager) onMouseReleased() bool {
	if m == nil || m.root == nil || !m.root.Visible() || m.pressedElement == nil {
		return false
	}
	target := m.root.FindInteractiveAt(m.input.LogicalMousePosition())
	pressed := m.pressedElement
	m.pressedElement = nil
	pressed.handleMouseReleased(pressed == target)
	return true
}

// 返回当前是否存在持续按住的交互元素
func (m *UIManager) onMouseHeld() bool {
	return m != nil && m.pressedElement != nil
}
