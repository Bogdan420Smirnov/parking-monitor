package web

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"sync"
)

type StatusUpdate struct {
	CameraID  string         `json:"camera_id"`
	Statuses  map[int]string `json:"statuses"`
	Timestamp int64          `json:"timestamp"`
}

type WebServer struct {
	mu       sync.RWMutex
	clients  map[chan StatusUpdate]bool
	tmpl     *template.Template
	lastStatus StatusUpdate
	Cameras  []string // список ID камер для шаблона
}

func NewWebServer() *WebServer {
	return &WebServer{
		clients: make(map[chan StatusUpdate]bool),
	}
}

func (w *WebServer) LoadTemplates() error {
	tmpl, err := template.ParseFiles("web/templates/index.html")
	if err != nil {
		return err
	}
	w.tmpl = tmpl
	return nil
}

func (w *WebServer) ServeIndex(wr http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(wr, r)
		return
	}
	wr.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := struct {
		Cameras []string
	}{
		Cameras: w.Cameras,
	}
	err := w.tmpl.Execute(wr, data)
	if err != nil {
		log.Printf("Template error: %v", err)
	}
}

func (w *WebServer) ServeSSE(wr http.ResponseWriter, r *http.Request) {
	wr.Header().Set("Content-Type", "text/event-stream")
	wr.Header().Set("Cache-Control", "no-cache")
	wr.Header().Set("Connection", "keep-alive")
	wr.Header().Set("Access-Control-Allow-Origin", "*")

	ch := make(chan StatusUpdate, 10)
	w.mu.Lock()
	w.clients[ch] = true
	w.mu.Unlock()

	// Отправить текущий статус сразу
	w.mu.RLock()
	if w.lastStatus.CameraID != "" {
		data, _ := json.Marshal(w.lastStatus)
		wr.Write([]byte("data: " + string(data) + "\n\n"))
		wr.(http.Flusher).Flush()
	}
	w.mu.RUnlock()

	for {
		select {
		case update := <-ch:
			data, err := json.Marshal(update)
			if err != nil {
				continue
			}
			_, err = wr.Write([]byte("data: " + string(data) + "\n\n"))
			if err != nil {
				goto cleanup
			}
			wr.(http.Flusher).Flush()
		case <-r.Context().Done():
			goto cleanup
		}
	}

cleanup:
	w.mu.Lock()
	delete(w.clients, ch)
	w.mu.Unlock()
	close(ch)
}

func (w *WebServer) BroadcastStatus(update StatusUpdate) {
	w.mu.Lock()
	w.lastStatus = update
	clients := make([]chan StatusUpdate, 0, len(w.clients))
	for ch := range w.clients {
		clients = append(clients, ch)
	}
	w.mu.Unlock()

	for _, ch := range clients {
		select {
		case ch <- update:
		default:
			// клиент медленный, пропускаем
		}
	}
}