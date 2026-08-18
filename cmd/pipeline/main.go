package main

import (
	"fmt"
	"image/jpeg"
	"log"
	"os"
	"parking-monitor/internal/camera"
	"parking-monitor/internal/detector"
	"parking-monitor/internal/parking"
	"parking-monitor/internal/storage"
	"time"

	onnx "github.com/yalue/onnxruntime_go"
)

func main() {
	// ========== 1. Загружаем конфиг ==========
	store, err := storage.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	cfg := store.GetConfig()

	// ========== 2. Создаём менеджер парковки ==========
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

	// ========== 3. Инициализируем ONNX ==========
	onnx.SetSharedLibraryPath("C:/onnxruntime/onnxruntime-win-x64-1.28.0/lib/onnxruntime.dll")
	err = onnx.InitializeEnvironment()
	if err != nil {
		log.Fatalf("ONNX init failed: %v", err)
	}
	defer onnx.DestroyEnvironment()

	// ========== 4. Создаём детектор с низким порогом ==========
	confThreshold := float32(0.3)
	det, err := detector.NewYOLODetector("yolov8n.onnx", confThreshold, false)
	if err != nil {
		log.Fatalf("Failed to create detector: %v", err)
	}
	defer det.Close()

	// ========== 5. Открываем видеофайл ==========
	videoPath := "parking_video.mp4"
	cam, err := camera.NewFileCamera(videoPath)
	if err != nil {
		log.Fatalf("Failed to open video file: %v", err)
	}
	defer cam.Close()

	// ========== 6. Основной цикл ==========
	camID := cfg.Cameras[0].ID
	ticker := time.NewTicker(time.Duration(cfg.DetectionIntervalMs) * time.Millisecond)
	defer ticker.Stop()

	statusTicker := time.NewTicker(5 * time.Second)
	defer statusTicker.Stop()

	var lastFrameTime time.Time
	var frameCount int

	fmt.Printf("Система запущена. Обработка видеофайла '%s'\n", videoPath)
	fmt.Printf("Порог уверенности: %.2f\n", confThreshold)

	for {
		select {
		case <-ticker.C:
			img := cam.ReadFrame()
			if img == nil {
				log.Println("Видео закончилось, выходим")
				return
			}
			frameCount++
			bounds := img.Bounds()
			imgWidth := bounds.Dx()
			imgHeight := bounds.Dy()

			// Сохраняем каждый 10-й кадр для визуальной проверки
			if frameCount%10 == 0 {
				outFile, err := os.Create(fmt.Sprintf("frame_%d.jpg", frameCount))
				if err == nil {
					jpeg.Encode(outFile, img, &jpeg.Options{Quality: 85})
					outFile.Close()
				}
			}

			// Детекция
			start := time.Now()
			detections, err := det.Detect(img)
			elapsed := time.Since(start)
			if err != nil {
				log.Printf("Detection error: %v", err)
				continue
			}

			// Отладочный вывод
			if len(detections) > 0 {
				fmt.Printf("Frame %d: found %d objects (in %v)\n", frameCount, len(detections), elapsed)
				for i, d := range detections {
					fmt.Printf("  %d: Bbox=%v, score=%.2f\n", i+1, d.Bbox, d.Score)
				}
			} else if frameCount%5 == 0 {
				fmt.Printf("Frame %d: no objects (in %v)\n", frameCount, elapsed)
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
			lastFrameTime = time.Now()

		case <-statusTicker.C:
			statuses := manager.GetStatuses(camID)
			fmt.Printf("\n=== Statuses for %s (last frame: %v ago) ===\n", camID, time.Since(lastFrameTime))
			for id, status := range statuses {
				fmt.Printf("  Spot %d: %s\n", id, status)
			}
		}
	}
}