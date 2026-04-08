package game

import (
	"fmt"
	"strconv"
	"strings"

	"tiny_farm/engine/config"
	"tiny_farm/engine/core"
	"tiny_farm/engine/utils"
	"tiny_farm/game/scenes"
)

func Run(args []string) error {
	cfg := config.Default()
	for _, arg := range args {
		if strings.HasPrefix(arg, "--frames=") {
			value := strings.TrimPrefix(arg, "--frames=")
			frames, err := strconv.Atoi(value)
			if err != nil || frames <= 0 {
				return fmt.Errorf("invalid --frames value: %q", value)
			}
			cfg.MaxFrames = frames
		}
	}

	app := core.NewApp(cfg)
	app.RegisterSceneSetup(setupInitialScene)
	return app.Run()
}

func setupInitialScene(ctx *core.Context) {
	titleScene := scenes.NewTitleScene(ctx)
	ctx.Dispatcher.Trigger(utils.PushSceneEventName, utils.PushSceneEvent{
		Scene: titleScene,
	})
}
