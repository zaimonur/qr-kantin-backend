package websocket

import (
	"log"
	"net/http"
	"os"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

// WebSocket ve CORS ayarları
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")

		// 1. İstek mobilden (Flutter) veya sunucu içi araçlardan geliyorsa Origin boş olur. Kapıyı aç.
		if origin == "" {
			return true
		}

		// 2. İstek bir web tarayıcısından geliyorsa, sadece kendi adreslerimize izin ver.
		allowedOrigins := map[string]bool{
			"http://188.132.165.48:3000": true, // Canlı SvelteKit arayüzün
			"http://localhost:3000":      true, // Geliştirme ortamın
		}

		// Gelen origin bizim listemizde varsa true, yoksa false döner (Yabancı siteleri engeller)
		return allowedOrigins[origin]
	},
}

// ServeWS: Echo rotası için handler. HTTP'yi WS'ye yükseltir.
func ServeWS(c echo.Context) error {
	// 1. Token'ı URL (Query Param) üzerinden alıyoruz
	tokenString := c.QueryParam("token")

	if tokenString == "" {
		log.Println("WS Reddedildi: Token eksik")
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Bağlantı reddedildi: Token eksik"})
	}

	// 2. Token'ı Tıpkı Middleware'deki gibi Doğrula
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})

	if err != nil || !token.Valid {
		log.Println("WS Reddedildi: Geçersiz token")
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Bağlantı reddedildi: Geçersiz token"})
	}

	// Token geçerliyse, bağlantıyı paşalar gibi WebSocket'e yükselt (Upgrade)
	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		log.Println("WebSocket Upgrade Hatası:", err)
		return err
	}

	client := &Client{hub: AppHub, conn: conn, send: make(chan []byte, 256)}
	client.hub.register <- client

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

	go client.writePump()

	return nil
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
