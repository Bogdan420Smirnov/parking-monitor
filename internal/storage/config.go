package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type Spot struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
	Rect  [4]int `json:"rect"`
}

type Camera struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	RTSPURL string `json:"rtsp_url"`
	Spots   []Spot `json:"spots"`
}

type Config struct {
	Cameras             []Camera `json:"cameras"`
	DetectionIntervalMs int      `json:"detection_interval_ms"`
	OccupancyTimeoutSec int      `json:"occupancy_timeout_sec"`
	IOUThreshold        float64  `json:"iou_threshold"`
}

type ConfigStore struct {
	mu     sync.RWMutex
	config *Config
}

// Конструктор ConfigStore
func LoadConfig(path string) (*ConfigStore, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config json: %w", err)
	}

	// базовая валидация
	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}

	store := &ConfigStore{
		config: &cfg,
	}
	return store, nil
}

// validateConfig проверяет корректность данных
func validateConfig(cfg *Config) error {
	if len(cfg.Cameras) == 0 || len(cfg.Cameras) > 3 {
		return fmt.Errorf("cameras count must be between 1 and 3, got %d", len(cfg.Cameras))
	}
	for i, cam := range cfg.Cameras {
		if cam.ID == "" {
			return fmt.Errorf("camera %d has empty id", i)
		}
		if cam.RTSPURL == "" {
			return fmt.Errorf("camera %s has empty rtsp_url", cam.ID)
		}
		if len(cam.Spots) == 0 || len(cam.Spots) > 10 {
			return fmt.Errorf("camera %s: spots count must be 1..10, got %d", cam.ID, len(cam.Spots))
		}
		for j, spot := range cam.Spots {
			if spot.ID < 1 {
				return fmt.Errorf("camera %s spot %d: id must be >0", cam.ID, j)
			}
			// проверка, что rect корректен
			if len(spot.Rect) != 4 {
				return fmt.Errorf("camera %s spot %d: rect must have 4 values", cam.ID, j)
			}
		}
	}
	if cfg.DetectionIntervalMs <= 0 {
		cfg.DetectionIntervalMs = 500
	}
	if cfg.OccupancyTimeoutSec <= 0 {
		cfg.OccupancyTimeoutSec = 10
	}
	if cfg.IOUThreshold <= 0 || cfg.IOUThreshold > 1 {
		cfg.IOUThreshold = 0.5
	}
	return nil
}

// гетер с мьютексом возвращаеет копию
func (s *ConfigStore) GetConfig() *Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfgCopy := *s.config
	return &cfgCopy
}

// перезагрузка без остановки
func (s *ConfigStore) ReloadConfig(path string) error {
	newStore, err := LoadConfig(path)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = newStore.config
	return nil
}
