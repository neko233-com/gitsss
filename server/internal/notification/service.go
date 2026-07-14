// Copyright 2026 gitsss Authors. All rights reserved.

package notification

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/neko233-com/gitsss/server/internal/audit"
	"github.com/neko233-com/gitsss/server/internal/state"
)

const (
	configurationNamespace = "notification-configurations"
	messageNamespace       = "notification-messages"
	maxIdempotencyBytes    = 256
	maxSubjectBytes        = 512
	maxBodyBytes           = 32 << 10
	deliveryLease          = 15 * time.Minute
)

type Channel string

const (
	ChannelEmail  Channel = "email"
	ChannelFeishu Channel = "feishu"
	ChannelWeCom  Channel = "wecom"
)

type Status string

const (
	StatusPending    Status = "pending"
	StatusDelivering Status = "delivering"
	StatusRetrying   Status = "retrying"
	StatusDelivered  Status = "delivered"
	StatusDead       Status = "dead"
)

var (
	ErrInvalidChannel = errors.New("unsupported notification channel")
	ErrNotConfigured  = errors.New("notification channel is not configured")
	ErrState          = errors.New("notification state unavailable")
)

type Configuration struct {
	Channel    Channel  `json:"channel"`
	WebhookURL string   `json:"webhook_url,omitempty"`
	Secret     string   `json:"secret,omitempty"`
	SMTPHost   string   `json:"smtp_host,omitempty"`
	SMTPPort   int      `json:"smtp_port,omitempty"`
	Username   string   `json:"username,omitempty"`
	Password   string   `json:"password,omitempty"`
	From       string   `json:"from,omitempty"`
	Recipients []string `json:"recipients,omitempty"`
}

type ChannelState struct {
	Channel     Channel `json:"channel"`
	Configured  bool    `json:"configured"`
	Destination string  `json:"destination,omitempty"`
}

type Message struct {
	ID                string    `json:"id"`
	IdempotencyKey    string    `json:"idempotency_key"`
	Channel           Channel   `json:"channel"`
	Subject           string    `json:"subject"`
	Body              string    `json:"body"`
	Status            Status    `json:"status"`
	Attempts          int       `json:"attempts"`
	NextAttemptAt     time.Time `json:"next_attempt_at,omitempty"`
	DeliveryStartedAt time.Time `json:"delivery_started_at,omitempty"`
	LastError         string    `json:"last_error,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	DeliveredAt       time.Time `json:"delivered_at,omitempty"`
}

type Adapter interface {
	Deliver(context.Context, Message) error
}

type Factory interface {
	Build(Configuration) (Adapter, error)
}

type Service struct {
	mu             sync.RWMutex
	configurations map[Channel]Configuration
	messages       []Message
	byKey          map[string]int
	factory        Factory
	audit          *audit.Log
	now            func() time.Time
	maxAttempts    int
	store          *state.Store
	vault          *state.Vault
}

func NewPersistentService(factory Factory, log *audit.Log, store *state.Store, vault *state.Vault) (*Service, error) {
	if store == nil || vault == nil {
		return nil, errors.New("notification state store and vault are required")
	}
	service := NewService(factory, log)
	service.store = store
	service.vault = vault
	if err := service.load(); err != nil {
		return nil, err
	}
	return service, nil
}

func NewService(factory Factory, log *audit.Log) *Service {
	return &Service{
		configurations: make(map[Channel]Configuration),
		byKey:          make(map[string]int),
		factory:        factory,
		audit:          log,
		now:            time.Now,
		maxAttempts:    4,
	}
}

func (s *Service) Configure(actor, requestID string, configuration Configuration) (ChannelState, error) {
	if !validChannel(configuration.Channel) {
		return ChannelState{}, ErrInvalidChannel
	}
	if _, err := s.factory.Build(configuration); err != nil {
		return ChannelState{}, err
	}

	configuration.Recipients = append([]string(nil), configuration.Recipients...)
	s.mu.Lock()
	if err := s.persistConfiguration(configuration); err != nil {
		s.mu.Unlock()
		return ChannelState{}, err
	}
	s.configurations[configuration.Channel] = configuration
	s.mu.Unlock()

	if s.audit != nil {
		if err := s.audit.Append(audit.Entry{Actor: actor, Action: "notifications.configure", Target: "notification:" + string(configuration.Channel), RequestID: requestID, At: s.now().UTC()}); err != nil {
			return ChannelState{}, fmt.Errorf("%w: %v", ErrState, err)
		}
	}
	return stateFor(configuration), nil
}

func (s *Service) Channels() []ChannelState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	states := make([]ChannelState, 0, 3)
	for _, channel := range []Channel{ChannelEmail, ChannelFeishu, ChannelWeCom} {
		configuration, configured := s.configurations[channel]
		if !configured {
			states = append(states, ChannelState{Channel: channel})
			continue
		}
		states = append(states, stateFor(configuration))
	}
	return states
}

func (s *Service) Enqueue(channel Channel, idempotencyKey, subject, body string) (Message, error) {
	if !validChannel(channel) {
		return Message{}, ErrInvalidChannel
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	subject = strings.TrimSpace(subject)
	body = strings.TrimSpace(body)
	if idempotencyKey == "" || len(idempotencyKey) > maxIdempotencyBytes || subject == "" || len(subject) > maxSubjectBytes || body == "" || len(body) > maxBodyBytes {
		return Message{}, errors.New("invalid notification message")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.configurations[channel]; !ok {
		return Message{}, ErrNotConfigured
	}
	key := string(channel) + ":" + idempotencyKey
	if index, ok := s.byKey[key]; ok {
		return s.messages[index], nil
	}
	now := s.now().UTC()
	message := Message{
		ID:             newMessageID(),
		IdempotencyKey: idempotencyKey,
		Channel:        channel,
		Subject:        subject,
		Body:           body,
		Status:         StatusPending,
		NextAttemptAt:  now,
		CreatedAt:      now,
	}
	if err := s.persistMessage(message); err != nil {
		return Message{}, err
	}
	s.messages = append(s.messages, message)
	s.byKey[key] = len(s.messages) - 1
	return message, nil
}

func (s *Service) DispatchDue(ctx context.Context, actor, requestID string, limit int) ([]Message, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	processed := make([]Message, 0, limit)
	for len(processed) < limit {
		index, message, configuration, ok, err := s.claimDue()
		if err != nil {
			return processed, err
		}
		if !ok {
			break
		}
		adapter, err := s.factory.Build(configuration)
		if err == nil {
			err = adapter.Deliver(ctx, message)
		}
		updated, persistErr := s.finish(index, message.ID, err)
		if persistErr != nil {
			return processed, persistErr
		}
		processed = append(processed, updated)
		action := "notifications.deliver"
		if updated.Status != StatusDelivered {
			action = "notifications.delivery_failed"
		}
		if s.audit != nil {
			if err := s.audit.Append(audit.Entry{Actor: actor, Action: action, Target: "notification:" + string(updated.Channel), RequestID: requestID, At: s.now().UTC()}); err != nil {
				return processed, fmt.Errorf("%w: %v", ErrState, err)
			}
		}
	}
	return processed, nil
}

func (s *Service) Messages() []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	messages := append([]Message(nil), s.messages...)
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
	return messages
}

func (s *Service) claimDue() (int, Message, Configuration, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	for index, message := range s.messages {
		if message.Status == StatusDelivering && !message.DeliveryStartedAt.Add(deliveryLease).After(now) {
			message = recoverInterruptedDelivery(message, now, "delivery lease expired before completion", s.maxAttempts)
			if err := s.persistMessage(message); err != nil {
				return 0, Message{}, Configuration{}, false, err
			}
			s.messages[index] = message
		}
		if (message.Status == StatusPending || message.Status == StatusRetrying) && !message.NextAttemptAt.After(now) {
			candidate := message
			candidate.Status = StatusDelivering
			candidate.Attempts++
			candidate.NextAttemptAt = time.Time{}
			candidate.DeliveryStartedAt = now
			candidate.LastError = ""
			if err := s.persistMessage(candidate); err != nil {
				return 0, Message{}, Configuration{}, false, err
			}
			s.messages[index] = candidate
			return index, candidate, s.configurations[message.Channel], true, nil
		}
	}
	return 0, Message{}, Configuration{}, false, nil
}

func (s *Service) finish(index int, messageID string, deliveryError error) (Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index >= len(s.messages) || s.messages[index].ID != messageID || s.messages[index].Status != StatusDelivering {
		return Message{}, errors.New("notification delivery claim changed")
	}
	message := s.messages[index]
	if deliveryError == nil {
		message.Status = StatusDelivered
		message.DeliveredAt = s.now().UTC()
		message.NextAttemptAt = time.Time{}
		message.DeliveryStartedAt = time.Time{}
		message.LastError = ""
		if err := s.persistMessage(message); err != nil {
			return Message{}, err
		}
		s.messages[index] = message
		return message, nil
	}
	message.LastError = sanitizeError(deliveryError)
	if message.Attempts >= s.maxAttempts {
		message.Status = StatusDead
		message.NextAttemptAt = time.Time{}
		message.DeliveryStartedAt = time.Time{}
		if err := s.persistMessage(message); err != nil {
			return Message{}, err
		}
		s.messages[index] = message
		return message, nil
	}
	message.Status = StatusRetrying
	message.NextAttemptAt = s.now().UTC().Add(time.Duration(1<<(message.Attempts-1)) * time.Minute)
	message.DeliveryStartedAt = time.Time{}
	if err := s.persistMessage(message); err != nil {
		return Message{}, err
	}
	s.messages[index] = message
	return message, nil
}

func (s *Service) load() error {
	configurationDocuments, err := s.store.List(configurationNamespace)
	if err != nil {
		return err
	}
	for _, document := range configurationDocuments {
		plaintext, err := s.vault.Open(configurationNamespace, document.ID, document.Payload)
		if err != nil {
			return err
		}
		var configuration Configuration
		if err := json.Unmarshal(plaintext, &configuration); err != nil || document.ID != string(configuration.Channel) || !validChannel(configuration.Channel) {
			return fmt.Errorf("invalid persisted notification configuration %q", document.ID)
		}
		if _, err := s.factory.Build(configuration); err != nil {
			return fmt.Errorf("invalid persisted notification configuration %q: %w", document.ID, err)
		}
		configuration.Recipients = append([]string(nil), configuration.Recipients...)
		s.configurations[configuration.Channel] = configuration
	}
	messageDocuments, err := s.store.List(messageNamespace)
	if err != nil {
		return err
	}
	loaded := make([]Message, 0, len(messageDocuments))
	for _, document := range messageDocuments {
		plaintext, err := s.vault.Open(messageNamespace, document.ID, document.Payload)
		if err != nil {
			return err
		}
		var message Message
		if err := json.Unmarshal(plaintext, &message); err != nil || document.ID != message.ID || !validPersistedMessage(message, s.maxAttempts) {
			return fmt.Errorf("invalid persisted notification message %q", document.ID)
		}
		if _, configured := s.configurations[message.Channel]; !configured {
			return fmt.Errorf("notification message %q has no configured channel", document.ID)
		}
		if message.Status == StatusDelivering {
			message = recoverInterruptedDelivery(message, s.now().UTC(), "delivery outcome unknown after restart", s.maxAttempts)
			if err := s.persistMessage(message); err != nil {
				return err
			}
		}
		loaded = append(loaded, message)
	}
	sort.Slice(loaded, func(left, right int) bool {
		if loaded[left].CreatedAt.Equal(loaded[right].CreatedAt) {
			return loaded[left].ID < loaded[right].ID
		}
		return loaded[left].CreatedAt.Before(loaded[right].CreatedAt)
	})
	for _, message := range loaded {
		key := string(message.Channel) + ":" + message.IdempotencyKey
		if _, exists := s.byKey[key]; exists {
			return fmt.Errorf("duplicate persisted notification idempotency key")
		}
		s.byKey[key] = len(s.messages)
		s.messages = append(s.messages, message)
	}
	return nil
}

func recoverInterruptedDelivery(message Message, now time.Time, finding string, maxAttempts int) Message {
	message.LastError = finding
	message.DeliveryStartedAt = time.Time{}
	if message.Attempts >= maxAttempts {
		message.Status = StatusDead
		message.NextAttemptAt = time.Time{}
		return message
	}
	message.Status = StatusRetrying
	message.NextAttemptAt = now.UTC()
	return message
}

func (s *Service) persistConfiguration(configuration Configuration) error {
	if s.store == nil {
		return nil
	}
	payload, err := json.Marshal(configuration)
	if err != nil {
		return err
	}
	sealed, err := s.vault.Seal(configurationNamespace, string(configuration.Channel), payload)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrState, err)
	}
	if err := s.store.Put(configurationNamespace, string(configuration.Channel), sealed); err != nil {
		return fmt.Errorf("%w: %v", ErrState, err)
	}
	return nil
}

func (s *Service) persistMessage(message Message) error {
	if s.store == nil {
		return nil
	}
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}
	sealed, err := s.vault.Seal(messageNamespace, message.ID, payload)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrState, err)
	}
	if err := s.store.Put(messageNamespace, message.ID, sealed); err != nil {
		return fmt.Errorf("%w: %v", ErrState, err)
	}
	return nil
}

func validPersistedMessage(message Message, maxAttempts int) bool {
	id, err := hex.DecodeString(strings.TrimPrefix(message.ID, "notification-"))
	if !strings.HasPrefix(message.ID, "notification-") || err != nil || len(id) != 12 || !validChannel(message.Channel) || message.CreatedAt.IsZero() {
		return false
	}
	if message.IdempotencyKey == "" || len(message.IdempotencyKey) > maxIdempotencyBytes || message.Subject == "" || len(message.Subject) > maxSubjectBytes || message.Body == "" || len(message.Body) > maxBodyBytes || message.Attempts < 0 || message.Attempts > maxAttempts || len(message.LastError) > 200 {
		return false
	}
	switch message.Status {
	case StatusPending:
		return message.Attempts == 0 && !message.NextAttemptAt.IsZero() && message.DeliveryStartedAt.IsZero() && message.DeliveredAt.IsZero()
	case StatusDelivering:
		return message.Attempts > 0 && message.NextAttemptAt.IsZero() && !message.DeliveryStartedAt.IsZero() && message.DeliveredAt.IsZero()
	case StatusRetrying:
		return message.Attempts > 0 && message.Attempts < maxAttempts && !message.NextAttemptAt.IsZero() && message.DeliveryStartedAt.IsZero() && message.DeliveredAt.IsZero() && message.LastError != ""
	case StatusDelivered:
		return message.Attempts > 0 && message.NextAttemptAt.IsZero() && message.DeliveryStartedAt.IsZero() && !message.DeliveredAt.IsZero() && message.LastError == ""
	case StatusDead:
		return message.Attempts >= maxAttempts && message.NextAttemptAt.IsZero() && message.DeliveryStartedAt.IsZero() && message.DeliveredAt.IsZero() && message.LastError != ""
	default:
		return false
	}
}

func newMessageID() string {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return fmt.Sprintf("notification-%024x", uint64(time.Now().UnixNano()))
	}
	return "notification-" + hex.EncodeToString(value)
}

func validChannel(channel Channel) bool {
	return channel == ChannelEmail || channel == ChannelFeishu || channel == ChannelWeCom
}

func stateFor(configuration Configuration) ChannelState {
	destination := configuration.WebhookURL
	if configuration.Channel == ChannelEmail {
		masked := make([]string, 0, len(configuration.Recipients))
		for _, recipient := range configuration.Recipients {
			masked = append(masked, maskDestination(recipient))
		}
		destination = strings.Join(masked, ", ")
		return ChannelState{Channel: configuration.Channel, Configured: true, Destination: destination}
	}
	return ChannelState{Channel: configuration.Channel, Configured: true, Destination: maskDestination(destination)}
}

func maskDestination(destination string) string {
	destination = strings.TrimSpace(destination)
	if destination == "" {
		return "已配置"
	}
	if at := strings.Index(destination, "@"); at > 1 {
		return destination[:1] + "***" + destination[at:]
	}
	if len(destination) <= 12 {
		return "已配置"
	}
	return destination[:8] + "…" + destination[len(destination)-4:]
}

func sanitizeError(err error) string {
	message := strings.NewReplacer("\r", " ", "\n", " ").Replace(err.Error())
	if len(message) > 200 {
		message = message[:200]
	}
	return message
}
