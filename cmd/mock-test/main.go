package main

import (
	"fmt"
	"parking-monitor/internal/parking"
	"parking-monitor/internal/storage"
	"time"
)

func main() {
	// Загружаем конфиг
	store, err := storage.LoadConfig("config.json")
	if err != nil {
		panic(err)
	}
	cfg := store.GetConfig()

	// Преобразуем конфиг в формат parking.CameraConfig
	camConfigs := make([]parking.CameraConfig, len(cfg.Cameras))
	for i, cam := range cfg.Cameras {
		spots := make([]struct {
			ID    int
			Label string
			Rect  [4]int
		}, len(cam.Spots))
		for j, s := range cam.Spots {
			spots[j] = struct {
				ID    int
				Label string
				Rect  [4]int
			}{
				ID:    s.ID,
				Label: s.Label,
				Rect:  s.Rect,
			}
		}
		camConfigs[i] = parking.CameraConfig{
			ID:    cam.ID,
			Spots: spots,
		}
	}

	manager := parking.NewManager(camConfigs, cfg.IOUThreshold, cfg.OccupancyTimeoutSec)
	camID := cfg.Cameras[0].ID

	// Создаём мок-детекцию, которая попадает в первую зону
	// Координаты должны перекрывать зону 1 (например, центр зоны)
	// Для примера возьмём зону из конфига – это [x1,y1,x2,y2].
	// Вычислим центры и сделаем бокс чуть меньше.
	spot1Rect := cfg.Cameras[0].Spots[0].Rect
	cx := float32(spot1Rect[0]+spot1Rect[2]) / 2
	cy := float32(spot1Rect[1]+spot1Rect[3]) / 2
	w := float32(spot1Rect[2]-spot1Rect[0]) * 0.8
	h := float32(spot1Rect[3]-spot1Rect[1]) * 0.8
	bbox := [4]float32{
		cx - w/2,
		cy - h/2,
		cx + w/2,
		cy + h/2,
	}
	mockDetections := []parking.Detection{
		{Bbox: bbox, Class: 2, Score: 0.9},
	}

	fmt.Println("Исходные статусы:", manager.GetStatuses(camID))

	// Обновляем с мок-детекцией
	manager.UpdateDetections(camID, mockDetections, time.Now())
	fmt.Println("Сразу после обновления (pending):", manager.GetStatuses(camID))

	// Ждём 12 секунд (больше чем occupancy_timeout)
	time.Sleep(12 * time.Second)
	fmt.Println("Через 12 секунд (occupied):", manager.GetStatuses(camID))

	// Убираем машину – передаём пустой список
	manager.UpdateDetections(camID, []parking.Detection{}, time.Now())
	fmt.Println("После исчезновения (free):", manager.GetStatuses(camID))
}