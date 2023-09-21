package main

import (
	"L1/internal/api"
	"log"
)

func main() {
	//t
	log.Println("- App start")
	api.StartServer()
	log.Println("- App terminated")
}
