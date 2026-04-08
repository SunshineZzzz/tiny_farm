package render

import "fmt"

type Renderer interface {
	Clear()
	DrawLine(text string)
	Present()
	Close()
}

type ConsoleRenderer struct {
	title string
	lines []string
}

func NewConsoleRenderer(title string) *ConsoleRenderer {
	fmt.Printf("boot renderer for %s\n", title)
	return &ConsoleRenderer{title: title}
}

func (r *ConsoleRenderer) Clear() {
	r.lines = nil
}

func (r *ConsoleRenderer) DrawLine(text string) {
	r.lines = append(r.lines, text)
}

func (r *ConsoleRenderer) Present() {
	fmt.Println("----- frame -----")
	for _, line := range r.lines {
		fmt.Println(line)
	}
}

func (r *ConsoleRenderer) Close() {
	fmt.Printf("shutdown renderer for %s\n", r.title)
}
