package api

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/jedarden/tunnel/pkg/tunnel"
)

// WebSocketMessage represents a message sent over WebSocket
type WebSocketMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
	Time    time.Time   `json:"time"`
}

// WSClient represents a connected WebSocket client
type WSClient struct {
	conn *websocket.Conn
	send chan *WebSocketMessage
}

// WebSocketHub manages all WebSocket connections
type WebSocketHub struct {
	clients    map[*WSClient]bool
	broadcast  chan *WebSocketMessage
	register   chan *WSClient
	unregister chan *WSClient
	mu         sync.RWMutex
}

var hub *WebSocketHub

func init() {
	hub = &WebSocketHub{
		clients:    make(map[*WSClient]bool),
		broadcast:  make(chan *WebSocketMessage, 256),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
	}
	go hub.run()
}

// run handles the WebSocket hub operations
func (h *WebSocketHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Client is slow, skip
				}
			}
			h.mu.RUnlock()
		}
	}
}

// handleWebSocket upgrades HTTP to WebSocket and handles the connection
func (s *Server) handleWebSocket(c *fiber.Ctx) error {
	// Check if websocket upgrade is requested
	if websocket.IsWebSocketUpgrade(c) {
		return websocket.New(func(conn *websocket.Conn) {
			s.websocketHandler(conn)
		})(c)
	}
	return fiber.ErrUpgradeRequired
}

// websocketHandler handles individual WebSocket connections
func (s *Server) websocketHandler(conn *websocket.Conn) {
	client := &WSClient{
		conn: conn,
		send: make(chan *WebSocketMessage, 256),
	}

	hub.register <- client
	defer func() {
		hub.unregister <- client
		conn.Close()
	}()

	// Subscribe to connection events
	eventPub := s.manager.GetEventPublisher()
	subscriber := eventPub.Subscribe("ws-client", func(event *tunnel.ConnectionEvent) bool {
		return true // Subscribe to all events
	})
	defer eventPub.Unsubscribe("ws-client")

	// Start goroutine to send messages to client
	go func() {
		for {
			select {
			case msg, ok := <-client.send:
				if !ok {
					return
				}

				if err := conn.WriteJSON(msg); err != nil {
					s.logger.Printf("WebSocket write error: %v", err)
					return
				}
			}
		}
	}()

	// Start goroutine to forward events to client
	go func() {
		for {
			select {
			case event, ok := <-subscriber.Channel:
				if !ok {
					return
				}

				msg := &WebSocketMessage{
					Type:    string(event.Type),
					Time:    event.Timestamp,
					Payload: map[string]interface{}{
						"conn_id": event.ConnID,
						"message": event.Message,
						"data":    event.Data,
					},
				}

				select {
				case client.send <- msg:
				default:
					// Skip if send buffer is full
				}
			}
		}
	}()

	// Handle incoming messages from client
	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				s.logger.Printf("WebSocket read error: %v", err)
			}
			break
		}

		// Handle incoming message
		if err := s.handleWSMessage(conn, msg); err != nil {
			s.logger.Printf("Error handling WebSocket message: %v", err)
		}
	}
}

// handleWSMessage processes incoming WebSocket messages
func (s *Server) handleWSMessage(conn *websocket.Conn, msg map[string]interface{}) error {
	msgType, ok := msg["type"].(string)
	if !ok {
		return conn.WriteJSON(&WebSocketMessage{
			Type:    "error",
			Payload: map[string]string{"message": "Invalid message type"},
			Time:    time.Now(),
		})
	}

	var response *WebSocketMessage

	switch msgType {
	case "ping":
		response = &WebSocketMessage{
			Type:    "pong",
			Payload: map[string]string{"message": "alive"},
			Time:    time.Now(),
		}

	case "subscribe":
		// Handle subscription requests
		response = &WebSocketMessage{
			Type:    "subscribed",
			Payload: map[string]string{"message": "Subscribed to events"},
			Time:    time.Now(),
		}

	case "list_connections":
		connections, err := s.manager.List()
		if err != nil {
			return err
		}

		connList := make([]map[string]interface{}, 0, len(connections))
		for _, conn := range connections {
			connList = append(connList, connectionToMap(conn))
		}

		response = &WebSocketMessage{
			Type:    "connections",
			Payload: connList,
			Time:    time.Now(),
		}

	case "list_providers":
		providers := s.registry.ListProviders()
		providerList := make([]map[string]interface{}, 0, len(providers))
		for _, p := range providers {
			providerList = append(providerList, map[string]interface{}{
				"name":      p.Name(),
				"category":  p.Category(),
				"installed": p.IsInstalled(),
				"connected": p.IsConnected(),
			})
		}

		response = &WebSocketMessage{
			Type:    "providers",
			Payload: providerList,
			Time:    time.Now(),
		}

	default:
		response = &WebSocketMessage{
			Type:    "error",
			Payload: map[string]string{"message": "Unknown message type: " + msgType},
			Time:    time.Now(),
		}
	}

	if response != nil {
		return conn.WriteJSON(response)
	}

	return nil
}

// BroadcastEvent broadcasts an event to all connected WebSocket clients
func BroadcastEvent(eventType string, payload interface{}) {
	msg := &WebSocketMessage{
		Type:    eventType,
		Payload: payload,
		Time:    time.Now(),
	}

	select {
	case hub.broadcast <- msg:
	default:
		// Drop message if broadcast channel is full
	}
}

// MarshalJSON custom JSON marshaling for WebSocketMessage
func (m *WebSocketMessage) MarshalJSON() ([]byte, error) {
	type Alias WebSocketMessage
	return json.Marshal(&struct {
		Time string `json:"time"`
		*Alias
	}{
		Time:  m.Time.Format(time.RFC3339),
		Alias: (*Alias)(m),
	})
}
