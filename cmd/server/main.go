package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"parking-monitor/internal/camera"
	"parking-monitor/internal/detector"
	"parking-monitor/internal/parking"
	"parking-monitor/internal/storage"
	"parking-monitor/internal/web"
	"time"

	onnx "github.com/yalue/onnxruntime_go"
)

func main() {
	// ===== Загружаем конфиг =====
	store, err := storage.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("Config load: %v", err)
	}
	cfg := store.GetConfig()

	// ===== Парковка =====
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

	// ===== ONNX =====
	onnx.SetSharedLibraryPath("C:/onnxruntime/onnxruntime-win-x64-1.28.0/lib/onnxruntime.dll")
	err = onnx.InitializeEnvironment()
	if err != nil {
		log.Fatalf("ONNX init: %v", err)
	}
	defer onnx.DestroyEnvironment()

	// ===== Веб-сервер =====
	webServer := web.NewWebServer()
	// Передаём список ID камер для отображения
	camIDs := make([]string, len(cfg.Cameras))
	for i, cam := range cfg.Cameras {
		camIDs[i] = cam.ID
	}
	webServer.Cameras = camIDs
	if err := webServer.LoadTemplates(); err != nil {
		log.Fatalf("Templates: %v", err)
	}

	// Создаём MJPEG-потоки для каждой камеры
	streams := make(map[string]*web.MJPEGStream)
	for _, cam := range cfg.Cameras {
		streams[cam.ID] = web.NewMJPEGStream()
	}

	// Роутинг
	http.HandleFunc("/", webServer.ServeIndex)
	http.HandleFunc("/events", webServer.ServeSSE)
	// /stream?camera=cam1
	http.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		camID := r.URL.Query().Get("camera")
		if camID == "" {
			http.Error(w, "Missing camera parameter", http.StatusBadRequest)
			return
		}
		stream, ok := streams[camID]
		if !ok {
			http.Error(w, "Camera not found", http.StatusNotFound)
			return
		}
		stream.ServeHTTP(w, r)
	})

	// Запускаем HTTP-сервер в горутине
	go func() {
		log.Println("HTTP сервер запущен на http://localhost:8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatalf("HTTP server: %v", err)
		}
	}()

	// ===== Обработка камер =====
	interval := time.Duration(cfg.DetectionIntervalMs) * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Запускаем горутину для каждой камеры
	for _, camCfg := range cfg.Cameras {
		camCfg := camCfg // захват переменной
		go processCamera(ctx, camCfg, manager, webServer, streams, interval)
	}

	// Ждём сигнал завершения
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	<-sigCh
	log.Println("Завершение работы...")
	cancel()
	time.Sleep(2 * time.Second)
}

func processCamera(ctx context.Context, camCfg storage.Camera, manager *parking.Manager, webServer *web.WebServer, streams map[string]*web.MJPEGStream, interval time.Duration) {
	camID := camCfg.ID
	log.Printf("Запуск обработки камеры %s (%s)", camID, camCfg.Name)

	cam, err := camera.NewFileCamera(camCfg.RTSPURL)
	if err != nil {
		log.Printf("Ошибка подключения к камере %s: %v", camID, err)
		return
	}
	defer cam.Close()

	// Создаём отдельный детектор для каждой камеры (можно общий, но для простоты отдельный)
	det, err := detector.NewYOLODetector("yolov8n.onnx", 0.3, false)
	if err != nil {
		log.Printf("Ошибка создания детектора для %s: %v", camID, err)
		return
	}
	defer det.Close()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var lastSSETime time.Time

	for {
		select {
		case <-ctx.Done():
			log.Printf("Остановка обработки камеры %s", camID)
			return
		case <-ticker.C:
			img := cam.ReadFrame()
			if img == nil {
				continue
			}
			bounds := img.Bounds()
			imgWidth := bounds.Dx()
			imgHeight := bounds.Dy()

			detections, err := det.Detect(img)
			if err != nil {
				log.Printf("Ошибка детекции на %s: %v", camID, err)
				continue
			}

			// Масштабируем боксы
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

			// Обновляем MJPEG-поток
			if stream, ok := streams[camID]; ok {
				stream.UpdateFrame(img)
			}

			// SSE раз в 500 мс
			if time.Since(lastSSETime) >= 500*time.Millisecond {
				statuses := manager.GetStatuses(camID)
				strStatuses := make(map[int]string)
				for id, st := range statuses {
					strStatuses[id] = string(st)
				}
				update := web.StatusUpdate{
					CameraID:  camID,
					Statuses:  strStatuses,
					Timestamp: time.Now().Unix(),
				}
				webServer.BroadcastStatus(update)
				lastSSETime = time.Now()
			}
		}
	}
}