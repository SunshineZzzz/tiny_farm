package core

import (
	"tiny_farm/engine/config"
	"tiny_farm/engine/event"
	"tiny_farm/engine/input"
	"tiny_farm/engine/render"
	"tiny_farm/engine/timeutil"
)

type Context struct {
	Config     config.Config
	Dispatcher *event.Dispatcher
	Clock      *timeutil.Clock
	Renderer   render.Renderer
	Input      *input.Manager
}
