package core

import (
	ectx "tiny_farm/engine/context"
)

type sceneSetupFunc func(*ectx.Context)

type GameApp struct {
	sceneSetup sceneSetupFunc
	isRunning  bool
}

func NewGameApp() *GameApp {
	return &GameApp{}
}

func (a *GameApp) RegisterSceneSetup(fn sceneSetupFunc) {
	a.sceneSetup = fn
}

func (a *GameApp) Run() {
	if err := a.init(); err != nil {
	}
	defer a.close()

	for a.isRunning {
	}
}

func (a *GameApp) init() error {
	return nil
}

func (a *GameApp) close() {
}
