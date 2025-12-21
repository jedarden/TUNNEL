package core

import (
	"sync"
	"time"
)

// EventType represents the type of connection event
type EventType int

const (
	EventConnected EventType = iota
	EventDisconnected
	EventReconnecting
	EventFailover
	EventMetricsUpdate
	EventError
	EventStateChange
	EventPrimaryChange
)

// String returns the string representation of EventType
func (e EventType) String() string {
	switch e {
	case EventConnected:
		return "Connected"
	case EventDisconnected:
		return "Disconnected"
	case EventReconnecting:
		return "Reconnecting"
	case EventFailover:
		return "Failover"
	case EventMetricsUpdate:
		return "MetricsUpdate"
	case EventError:
		return "Error"
	case EventStateChange:
		return "StateChange"
	case EventPrimaryChange:
		return "PrimaryChange"
	default:
		return "Unknown"
	}
}

// ConnectionEvent represents an event related to a connection
type ConnectionEvent struct {
	Type      EventType
	ConnID    string
	Timestamp time.Time
	Data      interface{}
	Message   string
}

// NewEvent creates a new connection event
func NewEvent(eventType EventType, connID string, data interface{}, message string) *ConnectionEvent {
	return &ConnectionEvent{
		Type:      eventType,
		ConnID:    connID,
		Timestamp: time.Now(),
		Data:      data,
		Message:   message,
	}
}

// EventSubscriber represents a subscriber to connection events
type EventSubscriber struct {
	ID      string
	Channel chan *ConnectionEvent
	Filter  func(*ConnectionEvent) bool // Optional filter function
}

// EventPublisher manages event publishing and subscription
type EventPublisher struct {
	mu          sync.RWMutex
	subscribers map[string]*EventSubscriber
	bufferSize  int
}

// NewEventPublisher creates a new event publisher
func NewEventPublisher(bufferSize int) *EventPublisher {
	if bufferSize <= 0 {
		bufferSize = 100
	}
	return &EventPublisher{
		subscribers: make(map[string]*EventSubscriber),
		bufferSize:  bufferSize,
	}
}

// Subscribe creates a new subscription to events
func (p *EventPublisher) Subscribe(id string, filter func(*ConnectionEvent) bool) *EventSubscriber {
	p.mu.Lock()
	defer p.mu.Unlock()

	subscriber := &EventSubscriber{
		ID:      id,
		Channel: make(chan *ConnectionEvent, p.bufferSize),
		Filter:  filter,
	}

	p.subscribers[id] = subscriber
	return subscriber
}

// Unsubscribe removes a subscriber
func (p *EventPublisher) Unsubscribe(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if sub, exists := p.subscribers[id]; exists {
		close(sub.Channel)
		delete(p.subscribers, id)
	}
}

// Publish sends an event to all subscribers
func (p *EventPublisher) Publish(event *ConnectionEvent) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, sub := range p.subscribers {
		// Apply filter if present
		if sub.Filter != nil && !sub.Filter(event) {
			continue
		}

		// Non-blocking send
		select {
		case sub.Channel <- event:
		default:
			// Channel full, skip this subscriber to avoid blocking
		}
	}
}

// SubscriberCount returns the number of active subscribers
func (p *EventPublisher) SubscriberCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.subscribers)
}

// Close closes all subscriber channels and clears the subscriber list
func (p *EventPublisher) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, sub := range p.subscribers {
		close(sub.Channel)
	}
	p.subscribers = make(map[string]*EventSubscriber)
}

// EventLogger is a helper to log events
type EventLogger struct {
	events []ConnectionEvent
	mu     sync.RWMutex
	maxLog int
}

// NewEventLogger creates a new event logger
func NewEventLogger(maxLog int) *EventLogger {
	if maxLog <= 0 {
		maxLog = 1000
	}
	return &EventLogger{
		events: make([]ConnectionEvent, 0, maxLog),
		maxLog: maxLog,
	}
}

// Log adds an event to the log
func (l *EventLogger) Log(event *ConnectionEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.events = append(l.events, *event)

	// Trim if exceeds max
	if len(l.events) > l.maxLog {
		l.events = l.events[len(l.events)-l.maxLog:]
	}
}

// GetRecent returns the most recent N events
func (l *EventLogger) GetRecent(n int) []ConnectionEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if n <= 0 || n > len(l.events) {
		n = len(l.events)
	}

	start := len(l.events) - n
	result := make([]ConnectionEvent, n)
	copy(result, l.events[start:])
	return result
}

// GetByType returns events of a specific type
func (l *EventLogger) GetByType(eventType EventType) []ConnectionEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]ConnectionEvent, 0)
	for _, event := range l.events {
		if event.Type == eventType {
			result = append(result, event)
		}
	}
	return result
}

// Clear clears the event log
func (l *EventLogger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = make([]ConnectionEvent, 0, l.maxLog)
}
