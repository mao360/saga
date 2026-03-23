package main

import "log"
import "github.com/mao360/saga/inventory/internal/app"

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
