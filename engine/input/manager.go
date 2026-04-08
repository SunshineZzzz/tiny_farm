package input

import (
	"tiny_farm/engine/event"
	"tiny_farm/engine/utils"
)

type Manager struct {
	dispatcher *event.Dispatcher
	maxFrames  int
}

func NewManager(dispatcher *event.Dispatcher, maxFrames int) *Manager {
	return &Manager{
		dispatcher: dispatcher,
		maxFrames:  maxFrames,
	}
}

func (m *Manager) HandleEvents(frame int) {
	if frame+1 >= m.maxFrames {
		m.dispatcher.Trigger(utils.QuitEventName, utils.QuitEvent{
			Reason: "frame limit reached",
		})
	}
}
