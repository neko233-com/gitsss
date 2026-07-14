// Copyright 2026 gitsss Authors. All rights reserved.

package dashboard

import (
	"sync"
	"time"

	"github.com/neko233-com/gitsss/server/internal/audit"
)

type Metric struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}

type Event struct {
	ID     string `json:"id"`
	Action string `json:"action"`
	Actor  string `json:"actor"`
	At     string `json:"at"`
	Kind   string `json:"kind"`
}

type Snapshot struct {
	Metrics []Metric `json:"metrics"`
	Events  []Event  `json:"events"`
}

type Service struct {
	mu       sync.RWMutex
	capacity int
	events   []Event
	audit    *audit.Log
}

func NewService(capacity int, events []Event, log *audit.Log) *Service {
	if capacity < 1 {
		panic("dashboard event preview capacity must be positive")
	}
	service := &Service{capacity: capacity, audit: log}
	for index := len(events) - 1; index >= 0; index-- {
		service.Touch(events[index])
	}
	return service
}

func (s *Service) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return Snapshot{
		Metrics: []Metric{{Label: "待处理", Value: 12}, {Label: "构建失败", Value: 3}, {Label: "待发布", Value: 2}},
		Events:  append([]Event(nil), s.events...),
	}
}

func (s *Service) Touch(event Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filtered := s.events[:0]
	for _, current := range s.events {
		if current.ID != event.ID {
			filtered = append(filtered, current)
		}
	}
	s.events = append([]Event{event}, filtered...)
	if len(s.events) > s.capacity {
		s.events = s.events[:s.capacity]
	}
}

func (s *Service) ClearPreview(actor, requestID string, now time.Time) error {
	if s.audit != nil {
		if err := s.audit.Append(audit.Entry{Actor: actor, Action: "events.preview.clear", Target: "dashboard", RequestID: requestID, At: now}); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.events = nil
	s.mu.Unlock()
	return nil
}
