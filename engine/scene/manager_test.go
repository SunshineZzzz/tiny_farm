package scene

import (
	"testing"

	"tiny_farm/engine/event"
	"tiny_farm/engine/utils"
)

type stubScene struct {
	name    string
	initted int
}

func (s *stubScene) Name() string   { return s.name }
func (s *stubScene) Init()          { s.initted++ }
func (s *stubScene) Update(float64) {}
func (s *stubScene) Render()        {}

func TestManagerProcessesQueuedSceneActionsInOrder(t *testing.T) {
	dispatcher := event.NewDispatcher()
	manager := NewManager(dispatcher)

	first := &stubScene{name: "first"}
	second := &stubScene{name: "second"}

	dispatcher.Trigger(utils.PushSceneEventName, utils.PushSceneEvent{Scene: first})
	dispatcher.Trigger(utils.ReplaceSceneEventName, utils.ReplaceSceneEvent{Scene: second})

	manager.Update(0)

	if manager.Current() != second {
		t.Fatalf("expected current scene to be second")
	}
	if first.initted != 1 {
		t.Fatalf("expected first scene init once, got %d", first.initted)
	}
	if second.initted != 1 {
		t.Fatalf("expected second scene init once, got %d", second.initted)
	}
}
