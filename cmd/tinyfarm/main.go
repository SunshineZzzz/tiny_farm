package main

import (
	"log"
	"os"

	"tiny_farm/game"
)

func main() {
	if err := game.Run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}
