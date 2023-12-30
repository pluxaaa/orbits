package main

import (
	"L1/internal/pkg/app"
	"context"
	"log"
)

// @title orbits docs
// @version 0.0-0
// @description ORBIT TRANSFER

// @host localhost:8000
// @schemes http
// @BasePath /

func main() {
	log.Println("Application start!")

	a, err := app.New(context.Background())
	if err != nil {
		log.Println(err)

		return
	}

	a.StartServer()

	log.Println("Application terminated!")
}
