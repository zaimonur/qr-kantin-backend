package handlers

import (
	"log"
	"net/http"
	"time"

	"qr-kantin/internal/db"

	"github.com/labstack/echo/v4"
)

type DashboardReport struct {
	TotalRevenue    float64 `json:"total_revenue"`
	CompletedOrders int     `json:"completed_orders"`
	CancelledOrders int     `json:"cancelled_orders"`
	PendingOrders   int     `json:"pending_orders"`
}

type DetailedOrder struct {
	ID          string  `db:"id" json:"id"`
	FullName    string  `db:"full_name" json:"full_name"`
	Email       string  `db:"email" json:"email"`
	Role        string  `db:"role" json:"role"`
	TotalPrice  float64 `db:"total_price" json:"total_price"`
	Status      string  `db:"status" json:"status"`
	CreatedAt   string  `db:"created_at" json:"created_at"`
	ItemsDetail string  `db:"items_detail" json:"items_detail"`
}

// =========================================================================
// GELİŞMİŞ SATIŞ RAPORU SİSTEMİ (Zaman, Kategori ve Ürün Bazlı)
// =========================================================================

type CategoryStat struct {
	Category string  `db:"category" json:"category"`
	Total    float64 `db:"total" json:"total"`
}

type HourlyStat struct {
	Hour  int `db:"hour" json:"hour"`
	Count int `db:"count" json:"count"`
}

type ProductSaleStat struct {
	Category    string  `db:"category" json:"category"`
	ProductName string  `db:"product_name" json:"product_name"`
	Quantity    int     `db:"quantity" json:"quantity"`
	Revenue     float64 `db:"revenue" json:"revenue"`
}

type SalesReportResponse struct {
	TotalRevenue    float64           `json:"total_revenue"`
	TotalOrders     int               `json:"total_orders"`
	CancelledOrders int               `json:"cancelled_orders"`
	CategorySales   []CategoryStat    `json:"category_sales"`
	HourlySales     []HourlyStat      `json:"hourly_sales"`
	ProductSales    []ProductSaleStat `json:"product_sales"`
}

func GetDashboardReports(c echo.Context) error {
	var report DashboardReport

	err := db.Instance.QueryRow(`
		SELECT 
			COALESCE(SUM(total_price), 0), 
			COUNT(id) 
		FROM orders 
		WHERE status = 'completed'`).Scan(&report.TotalRevenue, &report.CompletedOrders)

	if err != nil {
		log.Println("Ciro Çekme Hatası:", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Ciro hesaplanamadı"})
	}

	err = db.Instance.Get(&report.CancelledOrders, `SELECT COUNT(id) FROM orders WHERE status = 'cancelled'`)
	if err != nil {
		log.Println("İptal Sayısı Çekme Hatası:", err)
	}

	err = db.Instance.Get(&report.PendingOrders, `SELECT COUNT(id) FROM orders WHERE status IN ('pending', 'approved', 'ready')`)
	if err != nil {
		log.Println("Aktif Sipariş Çekme Hatası:", err)
	}

	return c.JSON(http.StatusOK, report)
}

func GetOrderHistory(c echo.Context) error {
	var history []AdminOrderResponse
	query := `
		SELECT 
			o.id, 
			COALESCE(u.full_name, 'Bilinmeyen Öğrenci') as full_name, 
			o.total_price, 
			o.status, 
			CAST(o.created_at AS VARCHAR) as created_at, 
			o.qr_code_token 
		FROM orders o 
		LEFT JOIN users u ON o.user_id = u.id 
		WHERE o.status IN ('completed', 'cancelled') 
		ORDER BY o.created_at DESC
		LIMIT 100`

	err := db.Instance.Select(&history, query)
	if err != nil {
		log.Println("Geçmiş Çekme Hatası:", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Geçmiş alınamadı"})
	}

	if history == nil {
		history = []AdminOrderResponse{}
	}
	return c.JSON(http.StatusOK, history)
}

func GetSalesReport(c echo.Context) error {
	startDate := c.QueryParam("start_date")
	endDate := c.QueryParam("end_date")

	if startDate == "" || endDate == "" {
		today := time.Now().Format("2006-01-02")
		startDate = today
		endDate = today
	}

	var report SalesReportResponse

	// Temel Ciro İstatistikleri
	err := db.Instance.QueryRow(`
		SELECT 
			COALESCE(SUM(total_price) FILTER (WHERE status != 'cancelled'), 0) as total_revenue,
			COUNT(id) as total_orders,
			COUNT(id) FILTER (WHERE status = 'cancelled') as cancelled_orders
		FROM orders
		WHERE created_at >= $1::date AND created_at < $2::date + interval '1 day'
	`, startDate, endDate).Scan(&report.TotalRevenue, &report.TotalOrders, &report.CancelledOrders)

	if err != nil {
		log.Println("Rapor temel istatistik hatası:", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Rapor verileri hesaplanamadı"})
	}

	// Kategori Ciro Dağılımı
	err = db.Instance.Select(&report.CategorySales, `
		SELECT 
			CAST(p.category AS VARCHAR) as category,
			COALESCE(SUM(oi.quantity * p.price), 0) as total
		FROM order_items oi
		JOIN orders o ON oi.order_id = o.id
		JOIN products p ON oi.product_id = p.id
		WHERE o.status != 'cancelled' 
		  AND o.created_at >= $1::date AND o.created_at < $2::date + interval '1 day'
		GROUP BY p.category
	`, startDate, endDate)

	if err != nil {
		log.Println("Rapor kategori hatası:", err)
	}
	if report.CategorySales == nil {
		report.CategorySales = []CategoryStat{}
	}

	// Saatlik Yoğunluk Analizi
	err = db.Instance.Select(&report.HourlySales, `
		SELECT 
			CAST(EXTRACT(HOUR FROM created_at) AS INTEGER) as hour,
			CAST(COUNT(id) AS INTEGER) as count
		FROM orders
		WHERE status != 'cancelled'
		  AND created_at >= $1::date AND created_at < $2::date + interval '1 day'
		GROUP BY EXTRACT(HOUR FROM created_at)
		ORDER BY hour ASC
	`, startDate, endDate)

	if err != nil {
		log.Println("Rapor saatlik analiz hatası:", err)
	}
	if report.HourlySales == nil {
		report.HourlySales = []HourlyStat{}
	}

	// KATEGORİ DETAYLARI (Hangi üründen kaç tane satıldı)
	err = db.Instance.Select(&report.ProductSales, `
		SELECT 
			CAST(p.category AS VARCHAR) as category,
			p.name as product_name,
			CAST(COALESCE(SUM(oi.quantity), 0) AS INTEGER) as quantity,
			COALESCE(SUM(oi.quantity * p.price), 0) as revenue
		FROM order_items oi
		JOIN orders o ON oi.order_id = o.id
		JOIN products p ON oi.product_id = p.id
		WHERE o.status != 'cancelled' 
		  AND o.created_at >= $1::date AND o.created_at < $2::date + interval '1 day'
		GROUP BY p.category, p.name
		ORDER BY quantity DESC
	`, startDate, endDate)

	if err != nil {
		log.Println("Rapor ürün detay hatası:", err)
	}
	if report.ProductSales == nil {
		report.ProductSales = []ProductSaleStat{}
	}

	return c.JSON(http.StatusOK, report)
}

func GetDetailedOrders(c echo.Context) error {
	startDate := c.QueryParam("start_date")
	endDate := c.QueryParam("end_date")

	if startDate == "" || endDate == "" {
		today := time.Now().Format("2006-01-02")
		startDate = today
		endDate = today
	}

	var orders []DetailedOrder

	query := `
		SELECT 
			o.id, 
			COALESCE(u.full_name, 'Bilinmeyen Öğrenci') as full_name, 
			COALESCE(u.email, 'Belirtilmemiş') as email,
			COALESCE(CAST(u.role AS VARCHAR), 'Bilinmiyor') as role,
			o.total_price, 
			o.status, 
			TO_CHAR(o.created_at AT TIME ZONE 'Europe/Istanbul', 'YYYY-MM-DD HH24:MI') as created_at,
			COALESCE(STRING_AGG(oi.quantity || 'x ' || p.name, ', '), '') as items_detail
		FROM orders o 
		LEFT JOIN users u ON o.user_id = u.id 
		LEFT JOIN order_items oi ON o.id = oi.order_id
		LEFT JOIN products p ON oi.product_id = p.id
		WHERE o.created_at >= $1::date AND o.created_at < $2::date + interval '1 day'
		GROUP BY o.id, u.full_name, u.email, u.role, o.total_price, o.status, o.created_at
		ORDER BY o.created_at DESC
	`

	err := db.Instance.Select(&orders, query, startDate, endDate)
	if err != nil {
		log.Println("Detaylı Sipariş Çekme Hatası:", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Detaylı siparişler getirilemedi"})
	}

	if orders == nil {
		orders = []DetailedOrder{}
	}

	return c.JSON(http.StatusOK, orders)
}
