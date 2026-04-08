package config

type Config struct {
	GameTitle string
	MaxFrames int
}

func Default() Config {
	return Config{
		GameTitle: "Tiny Farm",
		MaxFrames: 5,
	}
}
