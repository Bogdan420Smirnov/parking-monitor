package detector

import "image"

// Detection представляет собой один обнаруженный объект
type Detection struct {
	Bbox  [4]float32 // Координаты: [x1, y1, x2, y2]
	Class int        // ID класса (например, 2 для 'car' в COCO)
	Score float32    // Уверенность модели
}

// Detector — интерфейс для всех детекторов, которые мы можем реализовать.
// Это позволяет нам легко переключаться между разными моделями (YOLO v5, v8, v12).
type Detector interface {
	Detect(img image.Image) ([]Detection, error) // Основной метод для детекции
	Close() error                                // Освобождение ресурсов (модели)
}

// YOLODetector — структура, которая будет реализовывать интерфейс Detector.
type YOLODetector struct {
	// Здесь мы позже добавим сессию ONNX Runtime и параметры
}

// NewYOLODetector — конструктор для нашего детектора.
// Принимает путь к ONNX-модели и другие параметры.
func NewYOLODetector(modelPath string, useGPU bool) (*YOLODetector, error) {
	// TODO: Инициализация ONNX-сессии
	return nil, nil
}

// Detect реализует метод интерфейса Detector
func (d *YOLODetector) Detect(img image.Image) ([]Detection, error) {
	// TODO: Преобразование изображения, инференс и постобработка
	return nil, nil
}

// Close освобождает ресурсы ONNX-сессии
func (d *YOLODetector) Close() error {
	// TODO: Уничтожение ONNX-сессии
	return nil
}
