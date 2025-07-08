package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Message represents a WebSocket message
type Message struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// Client represents a WebSocket client
type Client struct {
	ID      string
	Address string // Ethereum address
	Conn    *websocket.Conn
	Send    chan Message
	Manager *Manager
}

// Manager manages WebSocket connections
type Manager struct {
	clients    map[string]*Client // Map of address -> client
	register   chan *Client
	unregister chan *Client
	broadcast  chan Message
	mu         sync.RWMutex
}

// WebSocket upgrader with CORS support
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		allowedOrigins := []string{
			"http://localhost:3000",
			"https://nadmon.kadzu.dev",
			"https://be-nadmon.kadzu.dev",
		}

		for _, allowed := range allowedOrigins {
			if origin == allowed {
				return true
			}
		}
		return false
	},
}

// NewManager creates a new WebSocket manager
func NewManager() *Manager {
	return &Manager{
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan Message),
	}
}

// Start starts the WebSocket manager
func (m *Manager) Start() {
	log.Println("🔌 WebSocket manager started")

	for {
		select {
		case client := <-m.register:
			m.registerClient(client)

		case client := <-m.unregister:
			m.unregisterClient(client)

		case message := <-m.broadcast:
			m.broadcastMessage(message)
		}
	}
}

// registerClient registers a new client
func (m *Manager) registerClient(client *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// If there's already a client for this address, close the old connection
	if existingClient, exists := m.clients[client.Address]; exists {
		close(existingClient.Send)
		existingClient.Conn.Close()
	}

	m.clients[client.Address] = client
	log.Printf("✅ Client connected: %s (Total: %d)", client.Address, len(m.clients))

	// Send welcome message
	welcomeMsg := Message{
		Type:      "connected",
		Data:      map[string]string{"address": client.Address, "status": "connected"},
		Timestamp: time.Now(),
	}

	select {
	case client.Send <- welcomeMsg:
	default:
		close(client.Send)
		delete(m.clients, client.Address)
	}
}

// unregisterClient unregisters a client
func (m *Manager) unregisterClient(client *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.clients[client.Address]; exists {
		delete(m.clients, client.Address)
		close(client.Send)
		client.Conn.Close()
		log.Printf("❌ Client disconnected: %s (Total: %d)", client.Address, len(m.clients))
	}
}

// broadcastMessage broadcasts a message to all clients
func (m *Manager) broadcastMessage(message Message) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for address, client := range m.clients {
		select {
		case client.Send <- message:
		default:
			close(client.Send)
			delete(m.clients, address)
		}
	}
}

// NotifyUser sends a message to a specific user
func (m *Manager) NotifyUser(address string, messageType string, data interface{}) {
	m.mu.RLock()
	client, exists := m.clients[address]
	m.mu.RUnlock()

	if !exists {
		return // User not connected
	}

	message := Message{
		Type:      messageType,
		Data:      data,
		Timestamp: time.Now(),
	}

	select {
	case client.Send <- message:
		log.Printf("📤 Sent %s to %s", messageType, address)
	default:
		// Client's send channel is blocked, remove client
		m.unregister <- client
	}
}

// BroadcastToAll sends a message to all connected clients
func (m *Manager) BroadcastToAll(messageType string, data interface{}) {
	message := Message{
		Type:      messageType,
		Data:      data,
		Timestamp: time.Now(),
	}

	m.broadcast <- message
}

// GetConnectedUsers returns a list of connected user addresses
func (m *Manager) GetConnectedUsers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	users := make([]string, 0, len(m.clients))
	for address := range m.clients {
		users = append(users, address)
	}
	return users
}

// GetStats returns WebSocket manager statistics
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"connected_clients": len(m.clients),
		"connected_users":   m.GetConnectedUsers(),
	}
}

// UpgradeConnection upgrades HTTP connection to WebSocket
func (m *Manager) UpgradeConnection(w http.ResponseWriter, r *http.Request, address string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("❌ WebSocket upgrade failed: %v", err)
		return
	}

	client := &Client{
		ID:      generateClientID(),
		Address: address,
		Conn:    conn,
		Send:    make(chan Message, 256),
		Manager: m,
	}

	// Register the client
	m.register <- client

	// Start client goroutines
	go client.writePump()
	go client.readPump()
}

// readPump handles reading messages from the WebSocket connection
func (c *Client) readPump() {
	defer func() {
		c.Manager.unregister <- c
		c.Conn.Close()
	}()

	// Set read deadline and pong handler for keep-alive
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		// Read message from client
		_, messageBytes, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("❌ WebSocket error: %v", err)
			}
			break
		}

		// Parse client message
		var clientMessage map[string]interface{}
		if err := json.Unmarshal(messageBytes, &clientMessage); err != nil {
			log.Printf("⚠️ Invalid message from client %s: %v", c.Address, err)
			continue
		}

		// Handle client message (ping, subscribe to events, etc.)
		c.handleClientMessage(clientMessage)
	}
}

// writePump handles writing messages to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second) // Send ping every 54 seconds
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// Channel closed
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Send message as JSON
			if err := c.Conn.WriteJSON(message); err != nil {
				log.Printf("❌ Write error for client %s: %v", c.Address, err)
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleClientMessage processes messages received from clients
func (c *Client) handleClientMessage(message map[string]interface{}) {
	messageType, ok := message["type"].(string)
	if !ok {
		return
	}

	switch messageType {
	case "ping":
		// Respond to ping
		pongMsg := Message{
			Type:      "pong",
			Data:      map[string]string{"status": "ok"},
			Timestamp: time.Now(),
		}
		select {
		case c.Send <- pongMsg:
		default:
		}

	case "subscribe":
		// Handle event subscriptions (future feature)
		log.Printf("📝 Client %s subscribed to events", c.Address)

	default:
		log.Printf("⚠️ Unknown message type from client %s: %s", c.Address, messageType)
	}
}

// generateClientID generates a unique client ID
func generateClientID() string {
	return time.Now().Format("20060102150405") + "-" + "client"
}
