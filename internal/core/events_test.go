package core

import (
	"testing"
	"time"
)

func TestNewEventPublisher(t *testing.T) {
	publisher := NewEventPublisher(100)

	if publisher == nil {
		t.Fatal("Expected non-nil publisher")
	}

	if publisher.subscribers == nil {
		t.Error("Expected subscribers map to be initialized")
	}

	if publisher.bufferSize != 100 {
		t.Errorf("Expected bufferSize 100, got %d", publisher.bufferSize)
	}
}

func TestNewEventPublisherZeroBuffer(t *testing.T) {
	publisher := NewEventPublisher(0)

	if publisher == nil {
		t.Fatal("Expected non-nil publisher")
	}

	// Should default to 100
	if publisher.bufferSize != 100 {
		t.Errorf("Expected default bufferSize 100, got %d", publisher.bufferSize)
	}
}

func TestNewEventPublisherNegativeBuffer(t *testing.T) {
	publisher := NewEventPublisher(-50)

	if publisher == nil {
		t.Fatal("Expected non-nil publisher")
	}

	// Should default to 100
	if publisher.bufferSize != 100 {
		t.Errorf("Expected default bufferSize 100, got %d", publisher.bufferSize)
	}
}

func TestSubscribe(t *testing.T) {
	publisher := NewEventPublisher(100)

	sub := publisher.Subscribe("test-subscriber", nil)

	if sub == nil {
		t.Fatal("Expected non-nil subscriber")
	}

	if sub.ID != "test-subscriber" {
		t.Errorf("Expected ID 'test-subscriber', got '%s'", sub.ID)
	}

	if sub.Channel == nil {
		t.Error("Expected Channel to be initialized")
	}

	if sub.Filter != nil {
		t.Error("Expected Filter to be nil")
	}

	// Check that subscriber was added
	count := publisher.SubscriberCount()
	if count != 1 {
		t.Errorf("Expected 1 subscriber, got %d", count)
	}
}

func TestSubscribeWithFilter(t *testing.T) {
	publisher := NewEventPublisher(100)

	filter := func(event *ConnectionEvent) bool {
		return event.Type == EventConnected
	}

	sub := publisher.Subscribe("filtered-subscriber", filter)

	if sub.Filter == nil {
		t.Error("Expected Filter to be set")
	}
}

func TestSubscribeMultiple(t *testing.T) {
	publisher := NewEventPublisher(100)

	sub1 := publisher.Subscribe("subscriber-1", nil)
	sub2 := publisher.Subscribe("subscriber-2", nil)
	sub3 := publisher.Subscribe("subscriber-3", nil)

	if sub1 == nil || sub2 == nil || sub3 == nil {
		t.Fatal("Expected all subscribers to be non-nil")
	}

	count := publisher.SubscriberCount()
	if count != 3 {
		t.Errorf("Expected 3 subscribers, got %d", count)
	}
}

func TestUnsubscribe(t *testing.T) {
	publisher := NewEventPublisher(100)

	sub := publisher.Subscribe("test-subscriber", nil)

	// Verify subscriber exists
	count := publisher.SubscriberCount()
	if count != 1 {
		t.Errorf("Expected 1 subscriber, got %d", count)
	}

	// Unsubscribe
	publisher.Unsubscribe("test-subscriber")

	// Verify subscriber removed
	count = publisher.SubscriberCount()
	if count != 0 {
		t.Errorf("Expected 0 subscribers after unsubscribe, got %d", count)
	}

	// Verify channel is closed
	select {
	case _, ok := <-sub.Channel:
		if ok {
			t.Error("Expected channel to be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected channel to be closed immediately")
	}
}

func TestUnsubscribeNonExistent(t *testing.T) {
	publisher := NewEventPublisher(100)

	// Should not panic
	publisher.Unsubscribe("non-existent")

	count := publisher.SubscriberCount()
	if count != 0 {
		t.Errorf("Expected 0 subscribers, got %d", count)
	}
}

func TestPublish(t *testing.T) {
	publisher := NewEventPublisher(100)

	sub := publisher.Subscribe("test-subscriber", nil)

	event := NewEvent(EventConnected, "conn-1", nil, "Test connection")

	// Publish event
	publisher.Publish(event)

	// Receive event
	select {
	case receivedEvent := <-sub.Channel:
		if receivedEvent.Type != EventConnected {
			t.Errorf("Expected EventConnected, got %s", receivedEvent.Type)
		}
		if receivedEvent.ConnID != "conn-1" {
			t.Errorf("Expected ConnID 'conn-1', got '%s'", receivedEvent.ConnID)
		}
		if receivedEvent.Message != "Test connection" {
			t.Errorf("Expected Message 'Test connection', got '%s'", receivedEvent.Message)
		}
	case <-time.After(1 * time.Second):
		t.Error("Expected to receive event")
	}
}

func TestPublishToMultipleSubscribers(t *testing.T) {
	publisher := NewEventPublisher(100)

	sub1 := publisher.Subscribe("subscriber-1", nil)
	sub2 := publisher.Subscribe("subscriber-2", nil)
	sub3 := publisher.Subscribe("subscriber-3", nil)

	event := NewEvent(EventConnected, "conn-1", nil, "Test connection")

	// Publish event
	publisher.Publish(event)

	// All subscribers should receive the event
	received := make(chan bool, 3)

	go func() {
		select {
		case <-sub1.Channel:
			received <- true
		case <-time.After(1 * time.Second):
			received <- false
		}
	}()

	go func() {
		select {
		case <-sub2.Channel:
			received <- true
		case <-time.After(1 * time.Second):
			received <- false
		}
	}()

	go func() {
		select {
		case <-sub3.Channel:
			received <- true
		case <-time.After(1 * time.Second):
			received <- false
		}
	}()

	// Check all received
	for i := 0; i < 3; i++ {
		if !<-received {
			t.Errorf("Subscriber %d did not receive event", i+1)
		}
	}
}

func TestPublishWithFilter(t *testing.T) {
	publisher := NewEventPublisher(100)

	// Subscriber that only receives Connected events
	connectedFilter := func(event *ConnectionEvent) bool {
		return event.Type == EventConnected
	}
	sub1 := publisher.Subscribe("connected-only", connectedFilter)

	// Subscriber that receives all events
	sub2 := publisher.Subscribe("all-events", nil)

	// Publish Connected event
	connectedEvent := NewEvent(EventConnected, "conn-1", nil, "Connected")
	publisher.Publish(connectedEvent)

	// Both should receive
	select {
	case <-sub1.Channel:
		// Expected
	case <-time.After(500 * time.Millisecond):
		t.Error("Filtered subscriber should receive Connected event")
	}

	select {
	case <-sub2.Channel:
		// Expected
	case <-time.After(500 * time.Millisecond):
		t.Error("Unfiltered subscriber should receive Connected event")
	}

	// Publish Disconnected event
	disconnectedEvent := NewEvent(EventDisconnected, "conn-1", nil, "Disconnected")
	publisher.Publish(disconnectedEvent)

	// Only sub2 should receive
	select {
	case <-sub1.Channel:
		t.Error("Filtered subscriber should not receive Disconnected event")
	case <-time.After(200 * time.Millisecond):
		// Expected
	}

	select {
	case <-sub2.Channel:
		// Expected
	case <-time.After(500 * time.Millisecond):
		t.Error("Unfiltered subscriber should receive Disconnected event")
	}
}

func TestPublishNonBlocking(t *testing.T) {
	publisher := NewEventPublisher(1) // Very small buffer

	sub := publisher.Subscribe("test-subscriber", nil)

	// Fill the buffer
	publisher.Publish(NewEvent(EventConnected, "conn-1", nil, "Event 1"))

	// Don't read from channel, buffer is full

	// Publishing more events should not block
	done := make(chan bool, 1)
	go func() {
		publisher.Publish(NewEvent(EventConnected, "conn-2", nil, "Event 2"))
		publisher.Publish(NewEvent(EventConnected, "conn-3", nil, "Event 3"))
		done <- true
	}()

	select {
	case <-done:
		// Expected - publish should not block
	case <-time.After(1 * time.Second):
		t.Error("Publish should not block even when subscriber buffer is full")
	}

	// Drain the channel
	<-sub.Channel
}

func TestClose(t *testing.T) {
	publisher := NewEventPublisher(100)

	sub1 := publisher.Subscribe("subscriber-1", nil)
	sub2 := publisher.Subscribe("subscriber-2", nil)
	sub3 := publisher.Subscribe("subscriber-3", nil)

	count := publisher.SubscriberCount()
	if count != 3 {
		t.Errorf("Expected 3 subscribers, got %d", count)
	}

	// Close publisher
	publisher.Close()

	// All subscriber channels should be closed
	checkClosed := func(ch <-chan *ConnectionEvent, name string) {
		select {
		case _, ok := <-ch:
			if ok {
				t.Errorf("Expected %s channel to be closed", name)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Expected %s channel to be closed", name)
		}
	}

	checkClosed(sub1.Channel, "subscriber-1")
	checkClosed(sub2.Channel, "subscriber-2")
	checkClosed(sub3.Channel, "subscriber-3")

	// Subscriber count should be 0
	count = publisher.SubscriberCount()
	if count != 0 {
		t.Errorf("Expected 0 subscribers after close, got %d", count)
	}
}

func TestNewEvent(t *testing.T) {
	before := time.Now()
	event := NewEvent(EventConnected, "conn-1", "test-data", "Test message")
	after := time.Now()

	if event == nil {
		t.Fatal("Expected non-nil event")
	}

	if event.Type != EventConnected {
		t.Errorf("Expected Type EventConnected, got %s", event.Type)
	}

	if event.ConnID != "conn-1" {
		t.Errorf("Expected ConnID 'conn-1', got '%s'", event.ConnID)
	}

	if event.Data != "test-data" {
		t.Errorf("Expected Data 'test-data', got '%v'", event.Data)
	}

	if event.Message != "Test message" {
		t.Errorf("Expected Message 'Test message', got '%s'", event.Message)
	}

	if event.Timestamp.Before(before) || event.Timestamp.After(after) {
		t.Error("Expected Timestamp to be set to current time")
	}
}

func TestEventTypeString(t *testing.T) {
	tests := []struct {
		eventType EventType
		expected  string
	}{
		{EventConnected, "Connected"},
		{EventDisconnected, "Disconnected"},
		{EventReconnecting, "Reconnecting"},
		{EventFailover, "Failover"},
		{EventMetricsUpdate, "MetricsUpdate"},
		{EventError, "Error"},
		{EventStateChange, "StateChange"},
		{EventPrimaryChange, "PrimaryChange"},
	}

	for _, test := range tests {
		str := test.eventType.String()
		if str != test.expected {
			t.Errorf("Expected %s, got %s", test.expected, str)
		}
	}
}

func TestSubscriberCount(t *testing.T) {
	publisher := NewEventPublisher(100)

	if publisher.SubscriberCount() != 0 {
		t.Error("Expected 0 subscribers initially")
	}

	publisher.Subscribe("sub-1", nil)
	if publisher.SubscriberCount() != 1 {
		t.Errorf("Expected 1 subscriber, got %d", publisher.SubscriberCount())
	}

	publisher.Subscribe("sub-2", nil)
	if publisher.SubscriberCount() != 2 {
		t.Errorf("Expected 2 subscribers, got %d", publisher.SubscriberCount())
	}

	publisher.Subscribe("sub-3", nil)
	if publisher.SubscriberCount() != 3 {
		t.Errorf("Expected 3 subscribers, got %d", publisher.SubscriberCount())
	}

	publisher.Unsubscribe("sub-2")
	if publisher.SubscriberCount() != 2 {
		t.Errorf("Expected 2 subscribers, got %d", publisher.SubscriberCount())
	}
}

func TestNewEventLogger(t *testing.T) {
	logger := NewEventLogger(100)

	if logger == nil {
		t.Fatal("Expected non-nil logger")
	}

	if logger.events == nil {
		t.Error("Expected events slice to be initialized")
	}

	if logger.maxLog != 100 {
		t.Errorf("Expected maxLog 100, got %d", logger.maxLog)
	}
}

func TestNewEventLoggerZeroMax(t *testing.T) {
	logger := NewEventLogger(0)

	// Should default to 1000
	if logger.maxLog != 1000 {
		t.Errorf("Expected default maxLog 1000, got %d", logger.maxLog)
	}
}

func TestEventLoggerLog(t *testing.T) {
	logger := NewEventLogger(100)

	event := NewEvent(EventConnected, "conn-1", nil, "Test")
	logger.Log(event)

	logger.mu.RLock()
	defer logger.mu.RUnlock()

	if len(logger.events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(logger.events))
	}

	if logger.events[0].Type != EventConnected {
		t.Error("Event not logged correctly")
	}
}

func TestEventLoggerLogTrimming(t *testing.T) {
	logger := NewEventLogger(5) // Small max

	// Log more than max
	for i := 0; i < 10; i++ {
		event := NewEvent(EventConnected, "conn-1", i, "Test")
		logger.Log(event)
	}

	logger.mu.RLock()
	defer logger.mu.RUnlock()

	if len(logger.events) != 5 {
		t.Errorf("Expected 5 events (trimmed), got %d", len(logger.events))
	}

	// Should keep the most recent 5
	if logger.events[0].Data.(int) != 5 {
		t.Errorf("Expected oldest event to have data 5, got %d", logger.events[0].Data.(int))
	}

	if logger.events[4].Data.(int) != 9 {
		t.Errorf("Expected newest event to have data 9, got %d", logger.events[4].Data.(int))
	}
}

func TestEventLoggerGetRecent(t *testing.T) {
	logger := NewEventLogger(100)

	// Log some events
	for i := 0; i < 5; i++ {
		event := NewEvent(EventConnected, "conn-1", i, "Test")
		logger.Log(event)
	}

	// Get recent 3
	recent := logger.GetRecent(3)

	if len(recent) != 3 {
		t.Errorf("Expected 3 recent events, got %d", len(recent))
	}

	// Should be the most recent
	if recent[0].Data.(int) != 2 {
		t.Errorf("Expected first event data 2, got %d", recent[0].Data.(int))
	}

	if recent[2].Data.(int) != 4 {
		t.Errorf("Expected last event data 4, got %d", recent[2].Data.(int))
	}
}

func TestEventLoggerGetRecentAll(t *testing.T) {
	logger := NewEventLogger(100)

	// Log 3 events
	for i := 0; i < 3; i++ {
		event := NewEvent(EventConnected, "conn-1", i, "Test")
		logger.Log(event)
	}

	// Request more than available
	recent := logger.GetRecent(10)

	if len(recent) != 3 {
		t.Errorf("Expected 3 events, got %d", len(recent))
	}
}

func TestEventLoggerGetByType(t *testing.T) {
	logger := NewEventLogger(100)

	// Log various event types
	logger.Log(NewEvent(EventConnected, "conn-1", nil, "Connected"))
	logger.Log(NewEvent(EventDisconnected, "conn-1", nil, "Disconnected"))
	logger.Log(NewEvent(EventConnected, "conn-2", nil, "Connected"))
	logger.Log(NewEvent(EventError, "conn-1", nil, "Error"))
	logger.Log(NewEvent(EventConnected, "conn-3", nil, "Connected"))

	// Get only Connected events
	connected := logger.GetByType(EventConnected)

	if len(connected) != 3 {
		t.Errorf("Expected 3 Connected events, got %d", len(connected))
	}

	for _, event := range connected {
		if event.Type != EventConnected {
			t.Error("Expected all events to be EventConnected")
		}
	}

	// Get only Error events
	errors := logger.GetByType(EventError)

	if len(errors) != 1 {
		t.Errorf("Expected 1 Error event, got %d", len(errors))
	}
}

func TestEventLoggerClear(t *testing.T) {
	logger := NewEventLogger(100)

	// Log some events
	for i := 0; i < 5; i++ {
		logger.Log(NewEvent(EventConnected, "conn-1", nil, "Test"))
	}

	logger.mu.RLock()
	count := len(logger.events)
	logger.mu.RUnlock()

	if count != 5 {
		t.Errorf("Expected 5 events, got %d", count)
	}

	// Clear
	logger.Clear()

	logger.mu.RLock()
	count = len(logger.events)
	logger.mu.RUnlock()

	if count != 0 {
		t.Errorf("Expected 0 events after clear, got %d", count)
	}
}

func TestEventPublisherConcurrency(t *testing.T) {
	publisher := NewEventPublisher(1000)

	// Create multiple subscribers
	subs := make([]*EventSubscriber, 10)
	for i := 0; i < 10; i++ {
		subs[i] = publisher.Subscribe("sub-"+string(rune(i)), nil)
	}

	// Publish many events concurrently
	done := make(chan bool)
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 20; j++ {
				event := NewEvent(EventConnected, "conn-1", nil, "Test")
				publisher.Publish(event)
			}
			done <- true
		}(i)
	}

	// Wait for publishers
	for i := 0; i < 5; i++ {
		<-done
	}

	// No race conditions should occur
	count := publisher.SubscriberCount()
	if count != 10 {
		t.Errorf("Expected 10 subscribers, got %d", count)
	}
}
