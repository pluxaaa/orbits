package main

import (
	"L1/internal/pkg/app"
	"log"
)

// @title orbits
// @version 0.0-0
// @description orbit transfer

// @host 127.0.0.1:8000
// @schemes http
// @BasePath /

func main() {
	log.Println("Application start!")

	a := app.New()
	a.StartServer()

	log.Println("Application terminated!")
}
