package main

import (
	"fmt"
	"parking-monitor/internal/storage"
)

func main() {
	store, _ := storage.LoadConfig("config.json")
	cfg := store.GetConfig()
	for _, cam := range cfg.Cameras {
		fmt.Printf("Camera: %s (%s), spots: %d", cam.Name, cam.ID, len(cam.Spots))
	}
}
