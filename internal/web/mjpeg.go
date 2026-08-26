package web

import (
	"image"
	"image/jpeg"
	"log"
	"net/http"
	"sync"
	"time"
)

type MJPEGStream struct {
	mu       sync.RWMutex
	frame    image.Image
	hasFrame bool
}

func NewMJPEGStream() *MJPEGStream {
	return &MJPEGStream{}
}

func (m *MJPEGStream) UpdateFrame(img image.Image) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.frame = img
	m.hasFrame = true
}

func (m *MJPEGStream) ServeHTTP(wr http.ResponseWriter, r *http.Request) {
	wr.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=frame")
	wr.Header().Set("Cache-Control", "no-cache")
	wr.Header().Set("Connection", "keep-alive")

	// Проверка поддержки флашера
	flusher, ok := wr.(http.Flusher)
	if !ok {
		http.Error(wr, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	ticker := time.NewTicker(100 * time.Millisecond) // ~10 fps
	defer ticker.Stop()

	for range ticker.C {
		m.mu.RLock()
		if !m.hasFrame || m.frame == nil {
			m.mu.RUnlock()
			continue
		}
		// Копируем изображение (чтобы не было гонок)
		img := m.frame
		m.mu.RUnlock()

		// Кодируем в JPEG
		wr.Write([]byte("--frame\r\nContent-Type: image/jpeg\r\n\r\n"))
		err := jpeg.Encode(wr, img, &jpeg.Options{Quality: 70})
		if err != nil {
			log.Printf("JPEG encode error: %v", err)
			return
		}
		wr.Write([]byte("\r\n"))
		flusher.Flush()
	}
}