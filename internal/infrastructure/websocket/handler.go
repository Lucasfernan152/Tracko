package websocket

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"

	"tracko/internal/domain"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WSHandler struct {
	hub *Hub
}

func NewWSHandler(hub *Hub) *WSHandler {
	return &WSHandler{hub: hub}
}

func (h *WSHandler) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WebSocket] Error upgrading connection: %v", err)
		return
	}

	clientChan := make(chan *domain.Location, 256)
	h.hub.register <- clientChan

	defer func() {
		h.hub.unregister <- clientChan
		conn.Close()
	}()

	for location := range clientChan {
		err := conn.WriteJSON(location)
		if err != nil {
			log.Printf("[WebSocket] Error sending message to client: %v", err)
			break
		}
	}
}