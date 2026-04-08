package scene

type Scene interface {
	Name() string
	Init()
	Update(delta float64)
	Render()
}
