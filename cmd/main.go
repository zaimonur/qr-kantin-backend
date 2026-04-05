// Copyright (c) 2026 Onur Zaim. Tüm hakları saklıdır.
// Bu kodun ticari amaçla izinsiz kullanımı, kopyalanması veya dağıtılması yasaktır.
// İletişim: zaimonur08@gmail.com

package main

import (
	"log"
	"os"
	"qr-kantin/internal/db"
	"qr-kantin/internal/handlers"
	myMiddleware "qr-kantin/internal/middleware"
	myWS "qr-kantin/internal/websocket"

	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"
)

type CustomValidator struct {
	validator *validator.Validate
}

func (cv *CustomValidator) Validate(i interface{}) error {
	if err := cv.validator.Struct(i); err != nil {
		return err
	}
	return nil
}

func main() {
	// 1. .env Yükle
	if err := godotenv.Load(); err != nil {
		log.Println(".env dosyası bulunamadı, sistem değişkenleri denenecek.")
	}

	// 2. Veritabanına Bağlan
	db.Connect()
	defer db.Instance.Close()

	// 3. Echo Instance Oluştur
	e := echo.New()

	e.Validator = &CustomValidator{validator: validator.New()}

	// 4. Global Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	//Prometheus metrik toplayıcısını başlat
	e.Use(echoprometheus.NewMiddleware("qrkantin"))
	// Grafana'nın verileri çekeceği gizli rota
	e.GET("/metrics", echoprometheus.NewHandler())

	//Her IP için saniyede maksimum 20 isteğe izin verir
	e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(rate.Limit(20))))

	// 5. WebSocket Merkezini (Hub) Arka Planda Başlat
	go myWS.AppHub.Run()

	// ---------------------------------------------------------
	// 6. ROTALAR (ROUTES)
	// ---------------------------------------------------------

	// A. Public Rotalar (Giriş Kapısı)
	auth := e.Group("/auth")
	{
		// Öğrenciler buradan kaydolur (is_approved=false olarak başlar)
		auth.POST("/register", handlers.RegisterStudent)
		// Herkes buradan giriş yapar (is_approved kontrolü Login içinde)
		auth.POST("/login", handlers.Login)
	}

	// B. WebSocket (Sadece Kantinci Paneli İçin)
	// Canlıda buraya da AdminMiddleware ekleyerek güvenliği artırabilirsin
	e.GET("/ws", myWS.ServeWS)

	// C. API Rotaları (Giriş Yapmış Onaylı Kullanıcılar)
	api := e.Group("/api")
	api.Use(myMiddleware.JWTMiddleware)
	{
		api.GET("/menu", handlers.GetMenu)
		api.GET("/orders/me", handlers.GetMyOrders)
		api.POST("/order", handlers.CreateOrder) // Sipariş verme yetkisi

		// Bakiye işlemleri (Görüntüleme)
		api.GET("/wallet/balance", handlers.GetBalance)
		api.POST("/wallet/load", handlers.AddBalance) //Bakiye yükleme
		api.GET("/wallet/history", handlers.GetWalletHistory)
	}

	// D. Admin Rotaları (Sadece Kantinciler - Full Yetki)
	admin := e.Group("/admin")
	admin.Use(myMiddleware.JWTMiddleware)
	admin.Use(myMiddleware.AdminMiddleware)
	{
		// KULLANICI YÖNETİMİ (KAYITLAR)
		admin.POST("/register/student", handlers.RegisterStudentByAdmin)
		admin.POST("/register/teacher", handlers.RegisterTeacherByAdmin)
		admin.POST("/register/admin", handlers.RegisterAdminByAdmin)

		// MALZEME (STOK) YÖNETİMİ
		admin.POST("/materials", handlers.AddMaterial)
		admin.GET("/materials", handlers.GetMaterials)
		admin.DELETE("/materials/:id", handlers.DeleteMaterial)

		// ÜRÜN (MENÜ) YÖNETİMİ
		admin.POST("/products", handlers.CreateProduct)
		admin.GET("/products", handlers.GetProducts)
		admin.PUT("/products/:id", handlers.UpdateProduct)
		admin.DELETE("/products/:id", handlers.DeleteProduct)
		admin.GET("/products/:id/recipe", handlers.GetProductRecipe)

		// SİPARİŞ VE TESLİMAT
		admin.GET("/orders", handlers.GetActiveOrders)
		admin.PUT("/orders/:id/approve", handlers.ApproveOrder)
		admin.PUT("/orders/:id/reject", handlers.RejectOrder)
		admin.PUT("/orders/:id/ready", handlers.MarkOrderReady)
		admin.POST("/orders/complete", handlers.CompleteOrder)

		// RAPORLAMA
		admin.GET("/reports", handlers.GetDashboardReports)
		admin.GET("/history", handlers.GetOrderHistory)
		admin.GET("/reports/sales", handlers.GetSalesReport)
		admin.GET("/reports/detailed-orders", handlers.GetDetailedOrders)

		// KULLANICI ONAY VE BAKİYE YÖNETİMİ
		admin.GET("/users/students", handlers.GetAllCustomers) //Müşterilerin hepsini getir
		admin.GET("/users/pending", handlers.GetPendingStudents)
		admin.PUT("/users/:id/approve", handlers.ApproveAndLoadBalance)
		admin.POST("/users/:id/load-balance", handlers.AdminLoadBalance)
	}

	// 7. Server'ı Başlat
	port := os.Getenv("PORT")
	if port == "" {
		port = "1323"
	}
	e.Logger.Fatal(e.Start(":" + port))
}
