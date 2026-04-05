package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"qr-kantin/internal/db"
	"time"

	"github.com/labstack/echo/v4"
)

type CreateProductRequest struct {
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Category  string  `json:"category"`
	Materials []struct {
		MaterialID     string  `json:"material_id"`
		QuantityNeeded float64 `json:"quantity_needed"`
	} `json:"materials"`
}

func CreateProduct(c echo.Context) error {
	req := new(CreateProductRequest)

	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Veri hatası"})
	}

	// Transaction Başlat
	tx, err := db.Instance.Beginx()
	if err != nil {
		return err
	}

	// Ürünü Ekle
	var productID string
	err = tx.QueryRow(`INSERT INTO products (name, price, category) VALUES ($1, $2, $3) RETURNING id`, req.Name, req.Price, req.Category).Scan(&productID)
	if err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Ürün eklenemedi"})
	}

	// Reçeteyi (Malzemeleri) Ekle
	for _, m := range req.Materials {
		_, err = tx.Exec(`INSERT INTO product_materials (product_id, material_id, quantity_needed) VALUES ($1, $2, $3)`,
			productID, m.MaterialID, m.QuantityNeeded)
		if err != nil {
			tx.Rollback()
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Reçete oluşturulamadı"})
		}
	}

	tx.Commit()
	return c.JSON(http.StatusCreated, map[string]string{"message": "Ürün ve reçete başarıyla oluşturuldu", "id": productID})
}

// GetMenu: Sadece aktif ürünleri listeler (Mobil uygulama için kullanılacak)
func GetMenu(c echo.Context) error {
	ctx := c.Request().Context()
	cacheKey := "kantin:menu"

	// 1. Önce Redis'e Bak (Cache Hit kontrolü)
	if db.RedisClient != nil {
		cachedMenu, err := db.RedisClient.Get(ctx, cacheKey).Result()
		if err == nil {
			// Cache'de veri bulundu, direkt JSON olarak geri dönüyoruz
			return c.Blob(http.StatusOK, "application/json", []byte(cachedMenu))
		}
	}

	// 2. Cache'de Yoksa Veritabanından Çek
	type MenuResponse struct {
		ID       string  `db:"id" json:"id"`
		Name     string  `db:"name" json:"name"`
		Price    float64 `db:"price" json:"price"`
		IsActive bool    `db:"is_active" json:"is_active"`
		Category string  `db:"category" json:"category"`
		ImageURL *string `db:"image_url" json:"image_url"`
		InStock  bool    `db:"in_stock" json:"in_stock"`
	}

	var products []MenuResponse

	query := `
		SELECT p.id, p.name, p.price, CAST(p.category AS VARCHAR) as category, p.is_active, p.image_url,
			NOT EXISTS (
				SELECT 1 
				FROM product_materials pm 
				JOIN materials m ON pm.material_id = m.id 
				WHERE pm.product_id = p.id AND m.stock_quantity < pm.quantity_needed
			) as in_stock
		FROM products p 
		WHERE p.is_active = true 
		ORDER BY p.name ASC`

	err := db.Instance.Select(&products, query)
	if err != nil {
		log.Println("Mobil Menü Çekme Hatası:", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Menü getirilemedi"})
	}

	if products == nil {
		products = []MenuResponse{}
	}

	// 3. Veritabanından Çekilen Veriyi Redis'e Kaydet (5 Dakikalık TTL)
	if db.RedisClient != nil {
		menuJSON, _ := json.Marshal(products)
		// 5 dakika (300 saniye) cache'de tutuyoruz
		db.RedisClient.Set(ctx, cacheKey, menuJSON, 5*time.Minute)
	}

	return c.JSON(http.StatusOK, products)
}

// GetProducts: Kantincinin panelinde menüdeki ürünleri listeler
func GetProducts(c echo.Context) error {
	type ProductResponse struct {
		ID       string  `db:"id" json:"id"`
		Name     string  `db:"name" json:"name"`
		Price    float64 `db:"price" json:"price"`
		Category string  `db:"category" json:"category"`
		InStock  bool    `db:"in_stock" json:"in_stock"`
	}

	var products []ProductResponse

	// Kantincinin tablosu için de aynı akıllı sorguyu yazıyoruz
	query := `
		SELECT p.id, p.name, p.price, CAST(p.category AS VARCHAR) as category,
			NOT EXISTS (
				SELECT 1 
				FROM product_materials pm 
				JOIN materials m ON pm.material_id = m.id 
				WHERE pm.product_id = p.id AND m.stock_quantity < pm.quantity_needed
			) as in_stock
		FROM products p 
		ORDER BY p.name ASC`

	err := db.Instance.Select(&products, query)

	if err != nil {
		log.Println("Ürünleri Çekme Hatası:", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Ürünler getirilemedi"})
	}

	if products == nil {
		products = []ProductResponse{}
	}

	return c.JSON(http.StatusOK, products)
}

// DeleteProduct: Menüden ürün siler
func DeleteProduct(c echo.Context) error {
	id := c.Param("id")

	// Önce bu ürüne ait reçeteyi (içindekiler) temizle
	_, err := db.Instance.Exec(`DELETE FROM product_materials WHERE product_id = $1`, id)
	if err != nil {
		log.Println("Reçete Silme Hatası:", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Ürünün reçetesi silinemedi"})
	}

	// Şimdi ürünün kendisini sil
	res, err := db.Instance.Exec(`DELETE FROM products WHERE id = $1`, id)
	if err != nil {
		log.Println("Ürün Silme Hatası:", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Ürün silinemedi"})
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Ürün bulunamadı"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Ürün menüden kaldırıldı!"})
}

// GetProductRecipe: Bir ürünün mevcut reçetesini (malzemelerini) getirir
func GetProductRecipe(c echo.Context) error {
	id := c.Param("id")
	type RecipeItem struct {
		MaterialID     string  `db:"material_id" json:"material_id"`
		QuantityNeeded float64 `db:"quantity_needed" json:"quantity_needed"`
	}
	var items []RecipeItem
	err := db.Instance.Select(&items, `SELECT material_id, quantity_needed FROM product_materials WHERE product_id = $1`, id)
	if err != nil {
		log.Println("Reçete Çekme Hatası:", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Reçete getirilemedi"})
	}
	if items == nil {
		items = []RecipeItem{}
	}
	return c.JSON(http.StatusOK, items)
}

// UpdateProduct, mevcut bir ürünün adını, fiyatını, kategorisini ve reçetesini (malzemelerini) günceller.
// İşlem atomik bir yapıda (Transaction) yürütülür; ana bilgi veya reçete güncellemesinden biri başarısız olursa tüm işlem geri alınır.
func UpdateProduct(c echo.Context) error {
	id := c.Param("id")
	req := new(CreateProductRequest)

	// 1. Veri Bağlama (Binding)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Geçersiz veri formatı"})
	}

	// 2. Veri Doğrulama (Validation)
	// main.go'da kurduğumuz validator'ı kullanarak gelen verileri kontrol ediyoruz.
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Doğrulama hatası: " + err.Error()})
	}

	// 3. Transaction Başlatma
	tx, err := db.Instance.Beginx()
	if err != nil {
		log.Println("Transaction Başlatılamadı:", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Sistem hatası oluştu"})
	}

	// 4. Ürünün Ana Bilgilerini Güncelleme
	// updated_at = NOW() ekleyerek verinin değişim zamanını veritabanında izliyoruz.
	query := `UPDATE products SET name = $1, price = $2, category = $3, updated_at = NOW() WHERE id = $4`
	_, err = tx.Exec(query, req.Name, req.Price, req.Category, id)
	if err != nil {
		log.Printf("Ürün Güncelleme Hatası (ID: %s): %v\n", id, err)
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Ürün bilgileri güncellenemedi"})
	}

	// 5. Eski Reçeteyi Temizleme
	// Ürün içeriği tamamen değişebileceği için mevcut malzemeleri siliyoruz.
	_, err = tx.Exec(`DELETE FROM product_materials WHERE product_id = $1`, id)
	if err != nil {
		log.Println("Eski Reçete Silme Hatası:", err)
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Eski ürün reçetesi temizlenemedi"})
	}

	// 6. Yeni Reçeteyi Kaydetme
	// Gelen yeni malzeme listesini tek tek işliyoruz.
	for _, m := range req.Materials {
		_, err = tx.Exec(`INSERT INTO product_materials (product_id, material_id, quantity_needed) VALUES ($1, $2, $3)`,
			id, m.MaterialID, m.QuantityNeeded)
		if err != nil {
			log.Printf("Yeni Reçete Ekleme Hatası (Material: %s): %v\n", m.MaterialID, err)
			tx.Rollback()
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Yeni reçete kaydedilemedi"})
		}
	}

	// 7. İşlemi Onaylama
	if err := tx.Commit(); err != nil {
		log.Println("Transaction Commit Hatası:", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Değişiklikler kaydedilemedi"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Ürün ve reçetesi güncellendi!"})
}
