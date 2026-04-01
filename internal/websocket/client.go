package websocket

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

// SvelteKit'ten gelen istekleri engellememek için CORS ayarını esnek tutuyoruz
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Canlıda: os.Getenv("ALLOWED_ORIGINS") ile kontrol edilebilir
		// Şimdilik daha kontrollü bir yapı:
		origin := r.Header.Get("Origin")
		return origin != "" // Boş olmayan isteklere (şimdilik) izin ver
	},
}

// Client, tek bir WebSocket bağlantısını temsil eder
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// writePump: Hub'dan gelen mesajları WebSocket üzerinden frontend'e basar
func (c *Client) writePump() {
	defer c.conn.Close()
	for {
		message, ok := <-c.send
		if !ok {
			// Hub kanalı kapattıysa bağlantıyı sonlandır
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
	}
}

// ServeWS: Echo rotası için handler. HTTP'yi WS'ye yükseltir.
func ServeWS(c echo.Context) error {
	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		log.Println("WebSocket Upgrade Hatası:", err)
		return err
	}

	client := &Client{hub: AppHub, conn: conn, send: make(chan []byte, 256)}
	client.hub.register <- client

	// Bağlantının koptuğunu anlamak için basit bir okuma döngüsü (goroutine)
	go func() {
		defer func() {
			client.hub.unregister <- client
			client.conn.Close()
		}()
		for {
			if _, _, err := client.conn.ReadMessage(); err != nil {
				break
			}
		}
	}()

	// Mesajları yazdıracak asıl döngüyü başlat
	go client.writePump()

	return nil
}
