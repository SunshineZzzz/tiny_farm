package game

import (
	ectx "tiny_farm/engine/context"
	"tiny_farm/engine/core"
)

func Run() {
	app := core.NewGameApp()
	app.RegisterSceneSetup(setupInitialScene)
	app.Run()
}

func setupInitialScene(ctx *ectx.Context) {
}
