package utils

const (
	PushSceneEventName    = "scene.push"
	PopSceneEventName     = "scene.pop"
	ReplaceSceneEventName = "scene.replace"
	QuitEventName         = "app.quit"
)

type PushSceneEvent struct {
	Scene any
}

type ReplaceSceneEvent struct {
	Scene any
}

type QuitEvent struct {
	Reason string
}
