package handlers

import (
	"log"
	"net/http"
	"qr-kantin/internal/db"
	"qr-kantin/internal/models"

	"github.com/labstack/echo/v4"
)

// AddMaterial: Kantinci yeni malzeme ekler veya mevcut malzemenin stoğunu günceller
func AddMaterial(c echo.Context) error {
	m := new(models.Material)
	if err := c.Bind(m); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Geçersiz veri"})
	}

	// Veritabanında bu isimde bir malzeme var mı kontrol et
	// LOWER() fonksiyonu ile büyük/küçük harf duyarsız arama yapıyoruz (Örn: "Sucuk" ile "sUcUK" aynı sayılır)
	var existingMaterial models.Material
	err := db.Instance.Get(&existingMaterial, `SELECT id, stock_quantity FROM materials WHERE LOWER(name) = LOWER($1)`, m.Name)

	if err == nil {
		// Malzeme bulunursa yeni kayıt açmaz sadece üstüne ekler.
		_, err = db.Instance.Exec(`UPDATE materials SET stock_quantity = stock_quantity + $1 WHERE id = $2`, m.StockQuantity, existingMaterial.ID)
		if err != nil {
			log.Println("Stok Güncelleme Hatası:", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Stok güncellenemedi"})
		}

		return c.JSON(http.StatusOK, map[string]string{"message": "Mevcut malzemenin stoğu mermi gibi güncellendi!"})
	}

	// Malzeme yoksa yeni kayıt açar ve malzemeyi ekler
	query := `INSERT INTO materials (name, stock_quantity, unit) VALUES ($1, $2, $3) RETURNING id, created_at`

	err = db.Instance.QueryRow(query, m.Name, m.StockQuantity, m.Unit).Scan(&m.ID, &m.CreatedAt)
	if err != nil {
		log.Println("Malzeme Ekleme DB Hatası:", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Malzeme eklenemedi"})
	}

	return c.JSON(http.StatusCreated, m)
}

// GetMaterials: Tüm malzemeleri listeler (Stok takibi için)
func GetMaterials(c echo.Context) error {
	var materials []models.Material
	query := `SELECT id, name, stock_quantity, unit, created_at, updated_at FROM materials ORDER BY name ASC`

	err := db.Instance.Select(&materials, query)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Malzemeler getirilemedi"})
	}

	return c.JSON(http.StatusOK, materials)
}

// DeleteMaterial: Kantinci mevcut bir malzemeyi siler
func DeleteMaterial(c echo.Context) error {
	id := c.Param("id") // URL'den gelecek olan malzeme ID'si

	// Silinen malzeme eğer reçetede kullanılıyorsa 'Foreign Key Violation' hatası verir
	res, err := db.Instance.Exec(`DELETE FROM materials WHERE id = $1`, id)

	if err != nil {
		// Büyük ihtimalle bir üründe kullanıldığı için silinemiyor
		log.Println("Malzeme Silme Hatası:", err)
		return c.JSON(http.StatusConflict, map[string]string{
			"error": "Bu malzeme satışta olan bir ürünün reçetesinde (içindekiler) kullanılıyor! Önce o ürünü menüden kaldırmalısınız.",
		})
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Malzeme bulunamadı"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Malzeme mermi gibi silindi!"})
}
