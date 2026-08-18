package main

import (
	"log"
	"parking-monitor/internal/camera"
	"time"
)

func main() {
	rtspURL := "rtsp://wowzaec2demo.streamlock.net/vod/mp4:BigBuckBunny_115k.mp4" // замените на реальный URL
	cam, err := camera.NewCamera(rtspURL)
	if err != nil {
		log.Fatalf("Failed to create camera: %v", err)
	}
	defer cam.Close()

	for i := 0; i < 100; i++ {
		img := cam.ReadFrame()
		if img != nil {
			bounds := img.Bounds()
			log.Printf("Frame %d: %dx%d", i, bounds.Dx(), bounds.Dy())
		} else {
			log.Println("No frame yet")
		}
		time.Sleep(1 * time.Second)
	}
}