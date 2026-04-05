package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"qr-kantin/internal/db"
	"qr-kantin/internal/models"
	ws "qr-kantin/internal/websocket"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type CreateOrderRequest struct {
	Items []struct {
		ProductID uuid.UUID `json:"product_id"`
		Quantity  int       `json:"quantity"`
	} `json:"items"`
	Note string `json:"note"`
}

type AdminOrderResponse struct {
	ID         string  `db:"id" json:"id"`
	FullName   string  `db:"full_name" json:"full_name"`
	TotalPrice float64 `db:"total_price" json:"total_price"`
	Status     string  `db:"status" json:"status"`
	Note       string  `db:"note" json:"note"`
	CreatedAt  string  `db:"created_at" json:"created_at"`
	QRToken    string  `db:"qr_code_token" json:"qr_token"`
}

type CompleteOrderRequest struct {
	QRToken string `json:"qr_token"`
}

type OrderItemResponse struct {
	ProductName string  `json:"product_name"`
	Quantity    int     `json:"quantity"`
	Price       float64 `json:"price"`
}

type DetailedOrderResponse struct {
	ID          string              `json:"id"`
	TotalPrice  float64             `json:"total_price"`
	Status      string              `json:"status"`
	Note        string              `json:"note"`
	QRCodeToken string              `json:"qr_code_token"`
	CreatedAt   string              `json:"created_at"`
	Items       []OrderItemResponse `json:"items"`
}

func CreateOrder(c echo.Context) error {
	userID := c.Get("user_id").(string)
	req := new(CreateOrderRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Veri formatı hatalı"})
	}

	tx, err := db.Instance.Beginx()
	if err != nil {
		return err
	}

	var totalOrderPrice float64

	for _, item := range req.Items {
		var productPrice float64
		err = tx.Get(&productPrice, `SELECT price FROM products WHERE id = $1`, item.ProductID)
		if err != nil {
			tx.Rollback()
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Menüde böyle bir ürün bulunamadı"})
		}
		totalOrderPrice += productPrice * float64(item.Quantity)
	}

	res, err := tx.Exec(`UPDATE users SET balance = balance - $1 WHERE id = $2 AND balance >= $1`, totalOrderPrice, userID)
	if err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Bakiye işlemi sırasında bir hata oluştu"})
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Yetersiz bakiye! Lütfen cüzdanınıza para yükleyin."})
	}

	_, err = tx.Exec(`INSERT INTO wallet_transactions (user_id, amount, transaction_type, source) VALUES ($1, $2, 'spend', 'order_system')`, userID, totalOrderPrice)
	if err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "İşlem logu oluşturulamadı"})
	}

	var newBalance float64
	err = tx.Get(&newBalance, `SELECT balance FROM users WHERE id = $1`, userID)
	if err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Sipariş işlendi ama güncel bakiye okunamadı"})
	}

	var orderID uuid.UUID
	var createdAt string
	qrToken := uuid.New().String()

	err = tx.QueryRow(`INSERT INTO orders (user_id, total_price, status, qr_code_token, note) 
                       VALUES ($1, $2, $3, $4, $5) 
                       RETURNING id, TO_CHAR(created_at AT TIME ZONE 'Europe/Istanbul', 'YYYY-MM-DD HH24:MI:SS')`,
		userID, totalOrderPrice, "pending", qrToken, req.Note).Scan(&orderID, &createdAt)
	if err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Sipariş başlatılamadı"})
	}

	for _, item := range req.Items {
		var productPrice float64
		tx.Get(&productPrice, `SELECT price FROM products WHERE id = $1`, item.ProductID)
		_, err = tx.Exec(`INSERT INTO order_items (order_id, product_id, quantity, unit_price) VALUES ($1, $2, $3, $4)`,
			orderID, item.ProductID, item.Quantity, productPrice)
		if err != nil {
			tx.Rollback()
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Detay eklenemedi"})
		}
	}

	var fullName string
	err = tx.Get(&fullName, `SELECT full_name FROM users WHERE id = $1`, userID)
	if err != nil {
		fullName = "Bilinmeyen Öğrenci"
	}

	tx.Commit()

	// WebSocket bildirimini her şeyi içerecek şekilde güncelliyoruz
	notification := map[string]interface{}{
		"type":       "NEW_ORDER",
		"order_id":   orderID,
		"total":      totalOrderPrice,
		"qr_token":   qrToken,
		"note":       req.Note,
		"full_name":  fullName,
		"role":       "Öğrenci",
		"created_at": createdAt,
		"message":    "Yeni sipariş onayınızı bekliyor!",
	}
	if payload, err := json.Marshal(notification); err == nil {
		ws.AppHub.Broadcast <- payload
	}

	// MOBİLİN BEKLEDİĞİ TAM CEVAP (Hata alan kısım burasıydı)
	return c.JSON(http.StatusCreated, map[string]interface{}{
		"message":     "Siparişiniz alındı, kantinci onayı bekleniyor!",
		"order_id":    orderID,
		"total":       totalOrderPrice,
		"new_balance": newBalance,
	})
}

func ApproveOrder(c echo.Context) error {
	orderID := c.Param("id")

	tx, err := db.Instance.Beginx()
	if err != nil {
		return err
	}

	var orderStatus string
	err = tx.Get(&orderStatus, `SELECT status FROM orders WHERE id = $1`, orderID)
	if err != nil || orderStatus != "pending" {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Sipariş bulunamadı veya zaten işlenmiş"})
	}

	var items []models.OrderItem
	err = tx.Select(&items, `SELECT product_id, quantity FROM order_items WHERE order_id = $1`, orderID)
	for _, item := range items {
		var recipe []models.ProductMaterial
		err = tx.Select(&recipe, `SELECT material_id, quantity_needed FROM product_materials WHERE product_id = $1`, item.ProductID)
		for _, rm := range recipe {
			totalNeeded := rm.QuantityNeeded * float64(item.Quantity)
			res, err := tx.Exec(`UPDATE materials SET stock_quantity = stock_quantity - $1 WHERE id = $2 AND stock_quantity >= $1`, totalNeeded, rm.MaterialID)

			rowsAffected, _ := res.RowsAffected()
			if rowsAffected == 0 || err != nil {
				tx.Rollback()
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "Yetersiz stok! Lütfen depoyu kontrol edin."})
			}
		}
	}

	_, err = tx.Exec(`UPDATE orders SET status = 'approved' WHERE id = $1`, orderID)
	if err != nil {
		tx.Rollback()
		return err
	}

	tx.Commit()
	updatePayload, _ := json.Marshal(map[string]string{"type": "STATUS_UPDATE"})
	ws.AppHub.Broadcast <- updatePayload
	return c.JSON(http.StatusOK, map[string]string{"message": "Sipariş onaylandı ve stok düşüldü! Hazırlamaya başlayabilirsiniz."})
}

func RejectOrder(c echo.Context) error {
	orderID := c.Param("id")

	tx, err := db.Instance.Beginx()
	if err != nil {
		return err
	}

	var order models.Order
	err = tx.Get(&order, `SELECT user_id, total_price, status FROM orders WHERE id = $1 FOR UPDATE`, orderID)
	if err != nil || order.Status != "pending" {
		tx.Rollback()
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Bu sipariş iptal edilemez (Sadece bekleyen siparişler iptal edilebilir)"})
	}

	_, err = tx.Exec(`UPDATE orders SET status = 'cancelled' WHERE id = $1`, orderID)
	if err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Sipariş iptal edilemedi"})
	}

	_, err = tx.Exec(`UPDATE users SET balance = balance + $1 WHERE id = $2`, order.TotalPrice, order.UserID)
	if err != nil {
		tx.Rollback()
		log.Println("Para İade Hatası:", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Sipariş iptal edildi ancak para iadesi yapılamadı!"})
	}

	// İade (Refund) işlemini log tablosuna at
	_, err = tx.Exec(`INSERT INTO wallet_transactions (user_id, amount, transaction_type, source) VALUES ($1, $2, 'refund', 'admin_panel')`, order.UserID, order.TotalPrice)
	if err != nil {
		tx.Rollback()
		log.Println("İade Log Hatası:", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "İade yapıldı ancak loglanamadı!"})
	}

	tx.Commit()
	updatePayload, _ := json.Marshal(map[string]string{"type": "STATUS_UPDATE"})
	ws.AppHub.Broadcast <- updatePayload
	return c.JSON(http.StatusOK, map[string]string{"message": "Sipariş reddedildi ve öğrencinin parası cüzdanına iade edildi."})
}

func MarkOrderReady(c echo.Context) error {
	orderID := c.Param("id")
	var qrToken, userID string

	err := db.Instance.QueryRow(`
		UPDATE orders SET status = 'ready' 
		WHERE id = $1 AND status = 'approved' RETURNING qr_code_token, user_id`, orderID).Scan(&qrToken, &userID)

	if err != nil {
		log.Println("Sipariş Hazır İşaretleme Hatası:", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Sipariş güncellenemedi (Sadece 'Onaylanan' siparişler hazır edilebilir)"})
	}

	updatePayload, _ := json.Marshal(map[string]string{"type": "STATUS_UPDATE"})
	ws.AppHub.Broadcast <- updatePayload
	return c.JSON(http.StatusOK, map[string]string{"message": "Sipariş hazırlandı ve öğrenciye bildirildi!"})
}

func CompleteOrder(c echo.Context) error {
	req := new(CompleteOrderRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Geçersiz istek"})
	}

	res, err := db.Instance.Exec(`UPDATE orders SET status = 'completed' WHERE qr_code_token = $1 AND status = 'ready'`, req.QRToken)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Teslimat hatası"})
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Geçerli bir hazır sipariş bulunamadı!"})
	}

	updatePayload, _ := json.Marshal(map[string]string{"type": "STATUS_UPDATE"})
	ws.AppHub.Broadcast <- updatePayload
	return c.JSON(http.StatusOK, map[string]string{"message": "Sipariş teslim edildi!"})
}

func GetActiveOrders(c echo.Context) error {
	var orders []AdminOrderResponse

	query := `
		SELECT 
			o.id, 
			COALESCE(u.full_name, 'Bilinmeyen Öğrenci') as full_name, 
			o.total_price, 
			o.status, 
			COALESCE(o.note, '') as note,
			CAST(o.created_at AS VARCHAR) as created_at, 
			o.qr_code_token 
		FROM orders o 
		LEFT JOIN users u ON o.user_id = u.id 
		WHERE o.status IN ('pending', 'approved', 'ready') 
		ORDER BY o.created_at ASC`

	err := db.Instance.Select(&orders, query)
	if err != nil {
		log.Println("Siparişleri Çekme Hatası:", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Aktif siparişler alınamadı"})
	}

	if orders == nil {
		orders = []AdminOrderResponse{}
	}
	return c.JSON(http.StatusOK, orders)
}

func GetMyOrders(c echo.Context) error {
	userID := c.Get("user_id").(string)

	type FlatOrderItem struct {
		OrderID     string  `db:"order_id"`
		TotalPrice  float64 `db:"total_price"`
		Status      string  `db:"status"`
		Note        string  `db:"note"`
		Token       string  `db:"qr_code_token"`
		CreatedAt   string  `db:"created_at"`
		Quantity    int     `db:"quantity"`
		Price       float64 `db:"price"`
		ProductName string  `db:"product_name"`
	}

	var flatResults []FlatOrderItem
	query := `
		SELECT 
			o.id as order_id, o.total_price, o.status, COALESCE(o.note, '') as note, o.qr_code_token, TO_CHAR(o.created_at, 'YYYY-MM-DD HH24:MI') as created_at,
			oi.quantity, oi.unit_price as price, 
			p.name as product_name
		FROM orders o
		JOIN order_items oi ON o.id = oi.order_id
		JOIN products p ON oi.product_id = p.id
		WHERE o.user_id = $1
		ORDER BY o.created_at DESC`

	err := db.Instance.Select(&flatResults, query, userID)
	if err != nil {
		log.Println("Mobil Detaylı Sipariş Hatası:", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Sipariş geçmişi alınamadı"})
	}

	ordersMap := make(map[string]*DetailedOrderResponse)
	for _, res := range flatResults {
		if _, ok := ordersMap[res.OrderID]; !ok {
			ordersMap[res.OrderID] = &DetailedOrderResponse{
				ID:          res.OrderID,
				TotalPrice:  res.TotalPrice,
				Status:      res.Status,
				Note:        res.Note,
				QRCodeToken: res.Token,
				CreatedAt:   res.CreatedAt,
				Items:       []OrderItemResponse{},
			}
		}
		item := OrderItemResponse{
			ProductName: res.ProductName,
			Quantity:    res.Quantity,
			Price:       res.Price,
		}
		ordersMap[res.OrderID].Items = append(ordersMap[res.OrderID].Items, item)
	}

	var finalOrders []DetailedOrderResponse
	for _, o := range ordersMap {
		finalOrders = append(finalOrders, *o)
	}

	if finalOrders == nil {
		finalOrders = []DetailedOrderResponse{}
	}

	return c.JSON(http.StatusOK, finalOrders)
}
