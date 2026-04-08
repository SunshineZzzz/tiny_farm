package scenes

import (
	"fmt"

	"tiny_farm/engine/core"
)

type TitleScene struct {
	name        string
	context     *core.Context
	initialized bool
	updates     int
	lastDelta   float64
}

func NewTitleScene(ctx *core.Context) *TitleScene {
	return &TitleScene{
		name:    "TitleScene",
		context: ctx,
	}
}

func (s *TitleScene) Name() string {
	return s.name
}

func (s *TitleScene) Init() {
	s.initialized = true
}

func (s *TitleScene) Update(delta float64) {
	s.updates++
	s.lastDelta = delta
}

func (s *TitleScene) Render() {
	s.context.Renderer.DrawLine("render TitleScene")
	s.context.Renderer.DrawLine(fmt.Sprintf("initialized=%t updates=%d delta=%.6f", s.initialized, s.updates, s.lastDelta))
}
