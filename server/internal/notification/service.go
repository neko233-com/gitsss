// Copyright 2026 gitsss Authors. All rights reserved.

package notification

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/neko233-com/gitsss/server/internal/audit"
)

type Channel string

const (
	ChannelEmail  Channel = "email"
	ChannelFeishu Channel = "feishu"
	ChannelWeCom  Channel = "wecom"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusRetrying  Status = "retrying"
	StatusDelivered Status = "delivered"
	StatusDead      Status = "dead"
)

var (
	ErrInvalidChannel = errors.New("unsupported notification channel")
	ErrNotConfigured  = errors.New("notification channel is not configured")
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
	ID             string    `json:"id"`
	IdempotencyKey string    `json:"idempotency_key"`
	Channel        Channel   `json:"channel"`
	Subject        string    `json:"subject"`
	Body           string    `json:"body"`
	Status         Status    `json:"status"`
	Attempts       int       `json:"attempts"`
	NextAttemptAt  time.Time `json:"next_attempt_at,omitempty"`
	LastError      string    `json:"last_error,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	DeliveredAt    time.Time `json:"delivered_at,omitempty"`
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
	s.configurations[configuration.Channel] = configuration
	s.mu.Unlock()

	s.audit.Append(audit.Entry{Actor: actor, Action: "notifications.configure", Target: "notification:" + string(configuration.Channel), RequestID: requestID, At: s.now().UTC()})
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.configurations[channel]; !ok {
		return Message{}, ErrNotConfigured
	}
	key := string(channel) + ":" + strings.TrimSpace(idempotencyKey)
	if index, ok := s.byKey[key]; ok {
		return s.messages[index], nil
	}
	now := s.now().UTC()
	message := Message{
		ID:             fmt.Sprintf("notification-%d", now.UnixNano()),
		IdempotencyKey: strings.TrimSpace(idempotencyKey),
		Channel:        channel,
		Subject:        strings.TrimSpace(subject),
		Body:           strings.TrimSpace(body),
		Status:         StatusPending,
		NextAttemptAt:  now,
		CreatedAt:      now,
	}
	s.messages = append(s.messages, message)
	s.byKey[key] = len(s.messages) - 1
	return message, nil
}

func (s *Service) DispatchDue(ctx context.Context, actor, requestID string, limit int) []Message {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	processed := make([]Message, 0, limit)
	for len(processed) < limit {
		index, message, configuration, ok := s.nextDue()
		if !ok {
			break
		}
		adapter, err := s.factory.Build(configuration)
		if err == nil {
			err = adapter.Deliver(ctx, message)
		}
		updated := s.finish(index, err)
		processed = append(processed, updated)
		action := "notifications.deliver"
		if updated.Status != StatusDelivered {
			action = "notifications.delivery_failed"
		}
		s.audit.Append(audit.Entry{Actor: actor, Action: action, Target: "notification:" + string(updated.Channel), RequestID: requestID, At: s.now().UTC()})
	}
	return processed
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

func (s *Service) nextDue() (int, Message, Configuration, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := s.now().UTC()
	for index, message := range s.messages {
		if (message.Status == StatusPending || message.Status == StatusRetrying) && !message.NextAttemptAt.After(now) {
			return index, message, s.configurations[message.Channel], true
		}
	}
	return 0, Message{}, Configuration{}, false
}

func (s *Service) finish(index int, deliveryError error) Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	message := &s.messages[index]
	message.Attempts++
	if deliveryError == nil {
		message.Status = StatusDelivered
		message.DeliveredAt = s.now().UTC()
		message.NextAttemptAt = time.Time{}
		message.LastError = ""
		return *message
	}
	message.LastError = sanitizeError(deliveryError)
	if message.Attempts >= s.maxAttempts {
		message.Status = StatusDead
		message.NextAttemptAt = time.Time{}
		return *message
	}
	message.Status = StatusRetrying
	message.NextAttemptAt = s.now().UTC().Add(time.Duration(1<<(message.Attempts-1)) * time.Minute)
	return *message
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
