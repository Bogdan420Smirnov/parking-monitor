package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
	"parking-monitor/internal/detector"
	"time"

	onnx "github.com/yalue/onnxruntime_go"
)

func main() {
	// Указываем путь к библиотеке ONNX Runtime (если не прописано в PATH)
	onnx.SetSharedLibraryPath("C:/onnxruntime/onnxruntime-win-x64-1.28.0/lib/onnxruntime.dll")

	err := onnx.InitializeEnvironment()
	if err != nil {
		log.Fatalf("ONNX init failed: %v", err)
	}
	defer onnx.DestroyEnvironment()

	modelPath := "yolov8n.onnx"
	confThreshold := float32(0.5)
	det, err := detector.NewYOLODetector(modelPath, confThreshold, false)
	if err != nil {
		log.Fatalf("Failed to create detector: %v", err)
	}
	defer det.Close()

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

	start := time.Now()
	detections, err := det.Detect(img)
	if err != nil {
		log.Fatalf("Detection failed: %v", err)
	}
	fmt.Printf("Found %d objects in %v\n", len(detections), time.Since(start))
	for i, d := range detections {
		fmt.Printf("  %d: Bbox=%v, score=%.2f\n", i+1, d.Bbox, d.Score)
	}
}