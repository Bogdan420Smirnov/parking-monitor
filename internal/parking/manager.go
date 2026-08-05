package parking

import (
	"sync"
	"time"
)

// Manager управляет всеми местами для всех камер
type Manager struct {
	mu           sync.RWMutex
	cameras      map[string][]*Spot // key: cameraID
	iouThreshold float64
	timeoutSec   int
}

func NewManager(cameras []CameraConfig, iouThreshold float64, timeoutSec int) *Manager {
	m := &Manager{
		cameras:      make(map[string][]*Spot),
		iouThreshold: iouThreshold,
		timeoutSec:   timeoutSec,
	}
	for _, cam := range cameras {
		spots := make([]*Spot, len(cam.Spots))
		for i, spotCfg := range cam.Spots {
			spots[i] = NewSpot(spotCfg.ID, spotCfg.Label, spotCfg.Rect, timeoutSec)
		}
		m.cameras[cam.ID] = spots
	}
	return m
}

// UpdateDetections принимает детекции для конкретной камеры и обновляет статусы мест
func (m *Manager) UpdateDetections(cameraID string, detections []Detection, now time.Time) {

	m.mu.RLock()
	spots, ok := m.cameras[cameraID]
	m.mu.RUnlock()
	if !ok {
		return
	}

	for _, spot := range spots {
		found := false
		for _, det := range detections {
			if calculateIOU(spot.Rect, det.Bbox) >= m.iouThreshold {
				spot.UpdateOccupation(det.Bbox, m.iouThreshold, now)
				found = true
				break
			}
		}
		if !found {

			spot.mu.Lock()
			spot.status = Free
			spot.timerActive = false
			spot.mu.Unlock()
		}
	}
}

// GetStatuses возвращает карту статусов для всех мест камеры (для веб-интерфейса)
func (m *Manager) GetStatuses(cameraID string) map[int]Status {
	m.mu.RLock()
	spots, ok := m.cameras[cameraID]
	m.mu.RUnlock()
	if !ok {
		return nil
	}
	result := make(map[int]Status, len(spots))
	for _, spot := range spots {
		result[spot.ID] = spot.GetStatus()
	}
	return result
}

// GetAllStatuses возвращает статусы для всех камер
func (m *Manager) GetAllStatuses() map[string]map[int]Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	all := make(map[string]map[int]Status)
	for camID, spots := range m.cameras {
		statuses := make(map[int]Status, len(spots))
		for _, spot := range spots {
			statuses[spot.ID] = spot.GetStatus()
		}
		all[camID] = statuses
	}
	return all
}

type CameraConfig struct {
	ID    string
	Spots []struct {
		ID    int
		Label string
		Rect  [4]int
	}
}

type Detection struct {
	Bbox  [4]float32
	Class int
	Score float32
}
