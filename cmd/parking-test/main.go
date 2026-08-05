package main

import (
	"fmt"
	"parking-monitor/internal/parking"
	"time"
)

func main() {
	// Эмулируем конфиг (можно загрузить из config.json)
	camCfg := parking.CameraConfig{
		ID: "cam1",
		Spots: []struct {
			ID    int
			Label string
			Rect  [4]int
		}{
			{ID: 1, Label: "Место 1", Rect: [4]int{100, 200, 300, 400}},
			{ID: 2, Label: "Место 2", Rect: [4]int{350, 200, 550, 400}},
		},
	}

	manager := parking.NewManager([]parking.CameraConfig{camCfg}, 0.5, 10)

	// Эмулируем детекцию: машина в первом месте
	detections := []parking.Detection{
		{Bbox: [4]float32{120, 220, 280, 380}, Class: 2, Score: 0.8},
	}

	fmt.Println("До обновления:", manager.GetStatuses("cam1"))
	manager.UpdateDetections("cam1", detections, time.Now())
	fmt.Println("Сразу после обновления (pending):", manager.GetStatuses("cam1"))

	// Ждём 12 секунд
	time.Sleep(12 * time.Second)
	fmt.Println("Через 12 секунд (occupied):", manager.GetStatuses("cam1"))

	// Убираем машину
	manager.UpdateDetections("cam1", []parking.Detection{}, time.Now())
	fmt.Println("После исчезновения (free):", manager.GetStatuses("cam1"))
}
