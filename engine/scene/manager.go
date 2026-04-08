package scene

import (
	"tiny_farm/engine/event"
	"tiny_farm/engine/utils"
)

type pendingActionType int

const (
	actionPush pendingActionType = iota + 1
	actionPop
	actionReplace
)

type pendingAction struct {
	kind  pendingActionType
	scene Scene
}

type Manager struct {
	stack   []Scene
	pending []pendingAction
}

func NewManager(dispatcher *event.Dispatcher) *Manager {
	m := &Manager{}
	dispatcher.SubscribeTrigger(utils.PushSceneEventName, func(payload any) {
		event := payload.(utils.PushSceneEvent)
		m.pending = append(m.pending, pendingAction{kind: actionPush, scene: event.Scene.(Scene)})
	})
	dispatcher.SubscribeTrigger(utils.PopSceneEventName, func(_ any) {
		m.pending = append(m.pending, pendingAction{kind: actionPop})
	})
	dispatcher.SubscribeTrigger(utils.ReplaceSceneEventName, func(payload any) {
		event := payload.(utils.ReplaceSceneEvent)
		m.pending = append(m.pending, pendingAction{kind: actionReplace, scene: event.Scene.(Scene)})
	})
	return m
}

func (m *Manager) Update(delta float64) {
	if current := m.Current(); current != nil {
		current.Update(delta)
	}
	m.processPending()
}

func (m *Manager) Render() {
	for _, current := range m.stack {
		current.Render()
	}
}

func (m *Manager) Current() Scene {
	if len(m.stack) == 0 {
		return nil
	}
	return m.stack[len(m.stack)-1]
}

func (m *Manager) processPending() {
	actions := m.pending
	m.pending = nil
	for _, action := range actions {
		switch action.kind {
		case actionPush:
			m.stack = append(m.stack, action.scene)
			action.scene.Init()
		case actionPop:
			if len(m.stack) > 0 {
				m.stack = m.stack[:len(m.stack)-1]
			}
		case actionReplace:
			if len(m.stack) > 0 {
				m.stack = m.stack[:len(m.stack)-1]
			}
			m.stack = append(m.stack, action.scene)
			action.scene.Init()
		}
	}
}
