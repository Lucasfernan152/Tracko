package websocket

import (
	"sync"

	"tracko/internal/domain"
)

type Hub struct {
	clients    map[chan *domain.Location]bool
	broadcast  chan *domain.Location
	register   chan chan *domain.Location
	unregister chan chan *domain.Location
	mu         sync.Mutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[chan *domain.Location]bool),
		broadcast:  make(chan *domain.Location),
		register:   make(chan chan *domain.Location),
		unregister: make(chan chan *domain.Location),
	}
}

func (h *Hub) Run() {
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
				close(client)
			}
			h.mu.Unlock()

		case location := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				select {
				case client <- location:
				default:
					close(client)
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}

func (h *Hub) BroadcastLocation(loc *domain.Location) {
	h.broadcast <- loc
}