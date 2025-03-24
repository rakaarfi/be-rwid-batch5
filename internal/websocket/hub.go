package websocket

import (
	"github.com/gofiber/websocket/v2"
	"sync"
)

// Client merepresentasikan klien WebSocket.
type Client struct {
	Conn *websocket.Conn
	Mu   sync.Mutex
}

// Hub mengelola koneksi WebSocket.
type Hub struct {
	Clients    map[*Client]bool
	Broadcast  chan []byte
	Register   chan *Client
	Unregister chan *Client
}

// NewHub membuat instance Hub baru.
func NewHub() *Hub {
	return &Hub{
		Clients:    make(map[*Client]bool),
		Broadcast:  make(chan []byte),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
	}
}

// Run menjalankan loop Hub untuk mengelola register, unregister, dan Broadcast.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.Clients[client] = true
		case client := <-h.Unregister:
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				client.Conn.Close()
			}
		case message := <-h.Broadcast:
			for client := range h.Clients {
				client.Mu.Lock()
				err := client.Conn.WriteMessage(websocket.TextMessage, message)
				client.Mu.Unlock()
				if err != nil {
					h.Unregister <- client
				}
			}
		}
	}
}
