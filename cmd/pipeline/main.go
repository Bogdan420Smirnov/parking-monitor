package main

import (
	"fmt"
	"log"
	"parking-monitor/internal/camera"
	"parking-monitor/internal/detector"
	"parking-monitor/internal/parking"
	"parking-monitor/internal/storage"
	"time"

	onnx "github.com/yalue/onnxruntime_go"
)

func main() {
	// Загружаем конфиг
	store, err := storage.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	cfg := store.GetConfig()

	// Создаём менеджер парковки
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

	// Инициализируем ONNX
	onnx.SetSharedLibraryPath("C:/onnxruntime/onnxruntime-win-x64-1.28.0/lib/onnxruntime.dll")
	err = onnx.InitializeEnvironment()
	if err != nil {
		log.Fatalf("ONNX init failed: %v", err)
	}
	defer onnx.DestroyEnvironment()

	// Создаём детектор
	det, err := detector.NewYOLODetector("yolov8n.onnx", 0.3, false)
	if err != nil {
		log.Fatalf("Failed to create detector: %v", err)
	}
	defer det.Close()

	// Открываем видеофайл (зациклен)
	cam, err := camera.NewFileCamera("parking_video.mp4")
	if err != nil {
		log.Fatalf("Failed to open video: %v", err)
	}
	defer cam.Close()

	camID := cfg.Cameras[0].ID
	interval := time.Duration(cfg.DetectionIntervalMs) * time.Millisecond

	fmt.Println("Система запущена. Обработка видео...")

	var lastStatusPrint time.Time

	for {
		// Читаем кадр
		img := cam.ReadFrame()
		if img == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		bounds := img.Bounds()
		imgWidth := bounds.Dx()
		imgHeight := bounds.Dy()

		// Детекция
		detections, err := det.Detect(img)
		if err != nil {
			log.Printf("Detection error: %v", err)
			continue
		}

		// Масштабируем боксы к реальному разрешению
		scaledDetections := make([]parking.Detection, len(detections))
		for i, d := range detections {
			scaledDetections[i] = parking.Detection{
				Bbox: [4]float32{
					d.Bbox[0] * float32(imgWidth),
					d.Bbox[1] * float32(imgHeight),
					d.Bbox[2] * float32(imgWidth),
					d.Bbox[3] * float32(imgHeight),
				},
				Class: d.Class,
				Score: d.Score,
			}
		}

		// Обновляем статусы
		manager.UpdateDetections(camID, scaledDetections, time.Now())

		// Выводим статусы раз в 5 секунд
		if time.Since(lastStatusPrint) > 5*time.Second {
			statuses := manager.GetStatuses(camID)
			fmt.Printf("\n=== Statuses for %s ===\n", camID)
			for id, status := range statuses {
				fmt.Printf("  Spot %d: %s\n", id, status)
			}
			lastStatusPrint = time.Now()
		}

		// Пауза между кадрами согласно detection_interval_ms
		time.Sleep(interval)
	}
}