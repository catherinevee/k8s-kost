package api

import (
	"net/http"

	"k8s-cost-optimizer/internal/websocket"
)

// WebSocketHandler handles WebSocket connections
type WebSocketHandler struct {
	hub *websocket.Hub
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(hub *websocket.Hub) *WebSocketHandler {
	return &WebSocketHandler{
		hub: hub,
	}
}

// ServeWebSocket handles WebSocket upgrade and client management
func (h *WebSocketHandler) ServeWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := websocket.NewClient(h.hub, conn)
	h.hub.register <- client

	go client.WritePump()
	go client.ReadPump()
} 