package main

import (
	"log"
	"runtime"
)

func init() {
	runtime.LockOSThread()
	log.SetFlags(log.Lshortfile)
}

func main() {
	config := parseConfig()

	if err := runApplication(config); err != nil {
		log.Fatalf("Application failed: %v", err)
	}
}
