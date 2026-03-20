package main

import (
	"log"

	"github.com/mao360/saga/order/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
