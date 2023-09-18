package main

import (
	"L1/internal/api"
	"log"
)

func main() {
	//test rename
	log.Println("- App start")
	api.StartServer()
	log.Println("- App terminated")
}
