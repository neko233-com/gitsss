// Copyright 2026 gitsss Authors. All rights reserved.

package dashboard

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/neko233-com/gitsss/server/internal/audit"
	"github.com/neko233-com/gitsss/server/internal/state"
)

const previewStateNamespace = "dashboard-previews"

var (
	ErrInvalidPreview = errors.New("invalid dashboard preview")
	ErrState          = errors.New("dashboard state unavailable")
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

type previewDocument struct {
	Viewer string  `json:"viewer"`
	Events []Event `json:"events"`
}

type Service struct {
	mu       sync.RWMutex
	capacity int
	seed     []Event
	previews map[string][]Event
	audit    *audit.Log
	store    *state.Store
	vault    *state.Vault
}

func NewService(capacity int, events []Event, log *audit.Log) *Service {
	service, err := newService(capacity, events, log)
	if err != nil {
		panic(err)
	}
	return service
}

func NewPersistentService(capacity int, events []Event, log *audit.Log, store *state.Store, vault *state.Vault) (*Service, error) {
	if store == nil || vault == nil {
		return nil, errors.New("dashboard state store and vault are required")
	}
	service, err := newService(capacity, events, log)
	if err != nil {
		return nil, err
	}
	service.store = store
	service.vault = vault
	documents, err := store.List(previewStateNamespace)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrState, err)
	}
	for _, document := range documents {
		plaintext, err := vault.Open(previewStateNamespace, document.ID, document.Payload)
		if err != nil {
			return nil, fmt.Errorf("%w: encrypted preview %q could not be opened", ErrState, document.ID)
		}
		var preview previewDocument
		if err := json.Unmarshal(plaintext, &preview); err != nil || document.ID != previewID(preview.Viewer) || !validViewer(preview.Viewer) || !validEvents(preview.Events, capacity) {
			return nil, fmt.Errorf("%w: corrupt preview %q", ErrState, document.ID)
		}
		service.previews[preview.Viewer] = append([]Event(nil), preview.Events...)
	}
	return service, nil
}

func newService(capacity int, events []Event, log *audit.Log) (*Service, error) {
	if capacity < 1 || !validEvents(events, len(events)) {
		return nil, ErrInvalidPreview
	}
	service := &Service{capacity: capacity, previews: make(map[string][]Event), audit: log}
	for index := len(events) - 1; index >= 0; index-- {
		service.seed = touchEvents(service.seed, events[index], capacity)
	}
	return service, nil
}

func (s *Service) Snapshot() Snapshot {
	return s.SnapshotFor("")
}

func (s *Service) SnapshotFor(viewer string) Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	events, found := s.previews[viewer]
	if !found {
		events = s.seed
	}
	cloned := make([]Event, len(events))
	copy(cloned, events)
	return Snapshot{
		Metrics: []Metric{{Label: "待处理", Value: 12}, {Label: "构建失败", Value: 3}, {Label: "待发布", Value: 2}},
		Events:  cloned,
	}
}

// Touch updates the default preview used before a viewer has personalized their stream.
func (s *Service) Touch(event Event) {
	if !validEvent(event) {
		return
	}
	s.mu.Lock()
	s.seed = touchEvents(s.seed, event, s.capacity)
	s.mu.Unlock()
}

func (s *Service) TouchFor(viewer string, event Event) error {
	if !validViewer(viewer) || !validEvent(event) {
		return ErrInvalidPreview
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	events, found := s.previews[viewer]
	if !found {
		events = append([]Event(nil), s.seed...)
	}
	events = touchEvents(events, event, s.capacity)
	if err := s.persist(viewer, events); err != nil {
		return err
	}
	s.previews[viewer] = events
	return nil
}

func (s *Service) ClearPreview(actor, requestID string, now time.Time) error {
	if !validViewer(actor) {
		return ErrInvalidPreview
	}
	if s.audit != nil {
		if err := s.audit.Append(audit.Entry{Actor: actor, Action: "events.preview.clear", Target: "dashboard", RequestID: requestID, At: now}); err != nil {
			return err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.persist(actor, []Event{}); err != nil {
		return err
	}
	s.previews[actor] = []Event{}
	return nil
}

func (s *Service) persist(viewer string, events []Event) error {
	if s.store == nil {
		return nil
	}
	plaintext, err := json.Marshal(previewDocument{Viewer: viewer, Events: events})
	if err != nil {
		return fmt.Errorf("%w: %v", ErrState, err)
	}
	id := previewID(viewer)
	envelope, err := s.vault.Seal(previewStateNamespace, id, plaintext)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrState, err)
	}
	if err := s.store.Put(previewStateNamespace, id, envelope); err != nil {
		return fmt.Errorf("%w: %v", ErrState, err)
	}
	return nil
}

func touchEvents(events []Event, event Event, capacity int) []Event {
	updated := make([]Event, 0, min(capacity, len(events)+1))
	updated = append(updated, event)
	for _, current := range events {
		if current.ID != event.ID && len(updated) < capacity {
			updated = append(updated, current)
		}
	}
	return updated
}

func validEvents(events []Event, capacity int) bool {
	if len(events) > capacity {
		return false
	}
	seen := make(map[string]struct{}, len(events))
	for _, event := range events {
		if !validEvent(event) {
			return false
		}
		if _, exists := seen[event.ID]; exists {
			return false
		}
		seen[event.ID] = struct{}{}
	}
	return true
}

func validEvent(event Event) bool {
	return validText(event.ID, 256, true) && validText(event.Action, 4096, false) &&
		validText(event.Actor, 256, false) && validText(event.At, 64, false) && validText(event.Kind, 64, false)
}

func validViewer(viewer string) bool {
	return viewer == strings.TrimSpace(viewer) && validText(viewer, 128, true)
}

func validText(value string, maximum int, required bool) bool {
	if required && strings.TrimSpace(value) == "" {
		return false
	}
	return len(value) <= maximum && !strings.ContainsAny(value, "\r\n\x00")
}

func previewID(viewer string) string {
	digest := sha256.Sum256([]byte(viewer))
	return "preview-" + hex.EncodeToString(digest[:])
}
