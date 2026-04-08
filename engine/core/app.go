package core

import (
	"fmt"

	"tiny_farm/engine/config"
	"tiny_farm/engine/event"
	"tiny_farm/engine/input"
	"tiny_farm/engine/render"
	"tiny_farm/engine/scene"
	"tiny_farm/engine/timeutil"
	"tiny_farm/engine/utils"
)

type SceneSetupFunc func(*Context)

type App struct {
	config      config.Config
	dispatcher  *event.Dispatcher
	clock       *timeutil.Clock
	renderer    render.Renderer
	input       *input.Manager
	context     *Context
	scenes      *scene.Manager
	sceneSetup  SceneSetupFunc
	isRunning   bool
	frameNumber int
}

func NewApp(cfg config.Config) *App {
	return &App{config: cfg}
}

func (a *App) RegisterSceneSetup(fn SceneSetupFunc) {
	a.sceneSetup = fn
}

func (a *App) Run() error {
	if err := a.init(); err != nil {
		return err
	}
	defer a.close()

	for a.isRunning {
		delta := a.clock.Update()
		a.input.HandleEvents(a.frameNumber)
		a.scenes.Update(delta)
		a.renderer.Clear()
		a.scenes.Render()
		a.renderer.Present()
		a.dispatcher.Update()
		a.frameNumber++
	}

	return nil
}

func (a *App) init() error {
	a.dispatcher = event.NewDispatcher()
	a.clock = timeutil.NewClock()
	a.renderer = render.NewConsoleRenderer(a.config.GameTitle)
	a.context = &Context{
		Config:     a.config,
		Dispatcher: a.dispatcher,
		Clock:      a.clock,
		Renderer:   a.renderer,
	}
	a.input = input.NewManager(a.dispatcher, a.config.MaxFrames)
	a.context.Input = a.input
	a.scenes = scene.NewManager(a.dispatcher)

	a.dispatcher.SubscribeTrigger(utils.QuitEventName, func(_ any) {
		a.isRunning = false
	})

	if a.sceneSetup == nil {
		return fmt.Errorf("scene setup callback is required")
	}

	a.sceneSetup(a.context)
	a.isRunning = true
	return nil
}

func (a *App) close() {
	a.renderer.Close()
}
