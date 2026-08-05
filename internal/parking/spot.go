package parking

import (
	"sync"
	"time"
)

type Status string

const (
	Free     Status = "free"
	Occupied Status = "occupied"
	Pending  Status = "pending"
)

type Spot struct {
	ID          int
	Label       string
	Rect        [4]int
	mu          sync.RWMutex
	status      Status
	lastSeen    time.Time
	timerActive bool
	timeoutSec  int
}

func NewSpot(id int, label string, rect [4]int, timeoutSec int) *Spot {

	return &Spot{
		ID:         id,
		Label:      label,
		Rect:       rect,
		status:     Free,
		timeoutSec: timeoutSec,
	}
}

func (s *Spot) GetStatus() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

func (s *Spot) UpdateOccupation(box [4]float32, iouThreshold float64, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	iou := calculateIOU(s.Rect, box)

	if iou >= iouThreshold {

		s.lastSeen = now
		if !s.timerActive {

			s.timerActive = true
			go s.startTimer(now)
		}

		if s.status != Occupied {
			s.status = Pending
		}
	} else {
		s.timerActive = false
		s.status = Free
	}

}

func (s *Spot) startTimer(startTime time.Time) {

	time.Sleep(time.Duration(s.timeoutSec) * time.Second)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.timerActive && s.status == Pending {

		if s.lastSeen.After(startTime) || s.lastSeen.Equal(startTime) {
			s.status = Occupied
		} else {
			s.timerActive = false
			s.status = Free
		}

	}
}

func (s *Spot) ResetTimer() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.timerActive = false
	s.status = Free
}
