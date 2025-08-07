package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client represents a WebSocket client
type Client struct {
	hub                   *Hub
	conn                  *websocket.Conn
	send                  chan []byte
	subscribedNamespaces  map[string]bool
	mutex                 sync.RWMutex
}

// Message represents a WebSocket message
type Message struct {
	Type      string      `json:"type"`
	Namespace string      `json:"namespace,omitempty"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// Upgrader for WebSocket connections
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

// NewClient creates a new WebSocket client
func NewClient(hub *Hub, conn *websocket.Conn) *Client {
	return &Client{
		hub:                  hub,
		conn:                 conn,
		send:                 make(chan []byte, 256),
		subscribedNamespaces: make(map[string]bool),
	}
}

// ReadPump handles reading messages from the WebSocket
func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}

		c.handleMessage(message)
	}
}

// WritePump handles writing messages to the WebSocket
func (c *Client) WritePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage processes incoming WebSocket messages
func (c *Client) handleMessage(message []byte) {
	var msg Message
	if err := json.Unmarshal(message, &msg); err != nil {
		log.Printf("Error unmarshaling message: %v", err)
		return
	}

	switch msg.Type {
	case "subscribe":
		c.subscribeToNamespace(msg.Namespace)
	case "unsubscribe":
		c.unsubscribeFromNamespace(msg.Namespace)
	case "ping":
		c.sendPong()
	}
}

// subscribeToNamespace subscribes the client to a namespace
func (c *Client) subscribeToNamespace(namespace string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.subscribedNamespaces[namespace] = true

	response := Message{
		Type:      "subscribed",
		Namespace: namespace,
		Data:      "Successfully subscribed to " + namespace,
		Timestamp: time.Now(),
	}

	data, _ := json.Marshal(response)
	c.send <- data
}

// unsubscribeFromNamespace unsubscribes the client from a namespace
func (c *Client) unsubscribeFromNamespace(namespace string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.subscribedNamespaces, namespace)

	response := Message{
		Type:      "unsubscribed",
		Namespace: namespace,
		Data:      "Successfully unsubscribed from " + namespace,
		Timestamp: time.Now(),
	}

	data, _ := json.Marshal(response)
	c.send <- data
}

// sendPong sends a pong response
func (c *Client) sendPong() {
	response := Message{
		Type:      "pong",
		Data:      "pong",
		Timestamp: time.Now(),
	}

	data, _ := json.Marshal(response)
	c.send <- data
}

// IsSubscribedTo checks if the client is subscribed to a namespace
func (c *Client) IsSubscribedTo(namespace string) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.subscribedNamespaces[namespace]
} 