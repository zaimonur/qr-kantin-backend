package websocket

import "log"

// Hub, aktif WebSocket bağlantılarını ve mesaj trafiğini yönetir.
type Hub struct {
	clients    map[*Client]bool
	Broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
}

// Global Hub nesnemiz (Her yerden erişebilmek için)
var AppHub = NewHub()

func NewHub() *Hub {
	return &Hub{
		Broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

// Run, arka planda (goroutine olarak) sürekli çalışıp kanalları dinler.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			log.Println("🔌 Yeni bir kantinci paneli bağlandı!")
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				log.Println("❌ Kantinci paneli bağlantısı koptu!")
			}
		case message := <-h.Broadcast:
			// Yeni mesaj (sipariş) geldiğinde tüm bağlı ekranlara fırlat
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}
