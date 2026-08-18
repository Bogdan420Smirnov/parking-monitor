package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
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

	// ========== 2. Загружаем изображение ==========
	imgFile := "test_car.jpg"
	file, err := os.Open(imgFile)
	if err != nil {
		log.Fatalf("Cannot open image: %v", err)
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		log.Fatalf("Cannot decode image: %v", err)
	}
	bounds := img.Bounds()
	imgWidth := bounds.Dx()
	imgHeight := bounds.Dy()
	fmt.Printf("Image size: %dx%d\n", imgWidth, imgHeight)

	// ========== 3. Подготавливаем конфигурацию для parking ==========
	// Предположим, что зоны заданы для разрешения, указанного в конфиге (например, 1920x1080).
	// Если вы хотите масштабировать зоны, сделайте это здесь.
	// Для простоты мы будем использовать зоны как есть, если изображение совпадает.
	// Если нет – нужно масштабировать.
	// Я покажу, как масштабировать, если нужно.

	camConfigs := make([]parking.CameraConfig, len(cfg.Cameras))
	for i, cam := range cfg.Cameras {
    // Создаём срез анонимных структур, как в определении CameraConfig
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

	// Если нужно масштабировать зоны к размеру изображения, делаем это здесь.
	// Например, если эталонное разрешение – 1920x1080:
	// refW, refH := 1920, 1080
	// scaleX := float32(imgWidth) / float32(refW)
	// scaleY := float32(imgHeight) / float32(refH)
	// и пересчитываем Rect для каждого места.
	// Но для простоты оставим как есть, если изображение соответствует.

	// Создаём менеджер парковки
	manager := parking.NewManager(camConfigs, cfg.IOUThreshold, cfg.OccupancyTimeoutSec)

	// ========== 4. Инициализируем ONNX ==========
	onnx.SetSharedLibraryPath("C:/onnxruntime/onnxruntime-win-x64-1.28.0/lib/onnxruntime.dll")
	err = onnx.InitializeEnvironment()
	if err != nil {
		log.Fatalf("ONNX init failed: %v", err)
	}
	defer onnx.DestroyEnvironment()

	// ========== 5. Создаём детектор ==========
	det, err := detector.NewYOLODetector("yolov8n.onnx", 0.5, false)
	if err != nil {
		log.Fatalf("Failed to create detector: %v", err)
	}
	defer det.Close()

	// ========== 6. Детектируем ==========
	detections, err := det.Detect(img)
	if err != nil {
		log.Fatalf("Detection failed: %v", err)
	}
	fmt.Printf("Found %d cars\n", len(detections))

	// ========== 7. Масштабируем боксы к реальному разрешению ==========
	// Боксы приходят нормализованные [0..1] относительно 640x640.
	// Масштабируем к размеру изображения.
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

	// ========== 8. Передаём детекции в менеджер ==========
	camID := cfg.Cameras[0].ID
	manager.UpdateDetections(camID, scaledDetections, time.Now())

	// ========== 9. Получаем статусы ==========
	statuses := manager.GetStatuses(camID)
	fmt.Printf("\nStatuses for camera %s:\n", camID)
	for id, status := range statuses {
		fmt.Printf("  Spot %d: %s\n", id, status)
	}
}