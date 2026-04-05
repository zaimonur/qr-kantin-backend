package handlers

import (
	"net/http"
	"qr-kantin/internal/db"
	"qr-kantin/internal/models"

	"github.com/labstack/echo/v4"
)

// Kantincinin girdiği ilk bakiyeyi alacağımız yapı
type ApproveRequest struct {
	Balance float64 `json:"balance"`
}

// Sadece 'ogrenci' rolündeki ve is_approved = false olanları getirir
func GetPendingStudents(c echo.Context) error {
	var users []models.User

	// Onay bekleyenleri en yeni kayıt olana göre sıralayarak çekiyoruz
	query := `
		SELECT id, full_name, email, role, balance, is_approved, created_at 
		FROM users 
		WHERE role = 'ogrenci' AND is_approved = false 
		ORDER BY created_at DESC`

	err := db.Instance.Select(&users, query)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Onay bekleyen öğrenciler getirilemedi"})
	}

	// Eğer boş dönerse null yerine boş dizi [] gitsin diye ufak bir dokunuş
	if users == nil {
		users = []models.User{}
	}

	return c.JSON(http.StatusOK, users)
}

// GetAllCustomers: Sistemdeki tüm müşterileri listeler (Müşteri İşlemleri ekranı için)
func GetAllCustomers(c echo.Context) error {
	var users []models.User

	query := `
		SELECT id, full_name, email, role, balance, is_approved, created_at 
		FROM users 
		WHERE role IN ('ogrenci', 'ogretmen') AND is_approved = true 
		ORDER BY full_name ASC`

	err := db.Instance.Select(&users, query)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Müşteri listesi alınamadı"})
	}

	if users == nil {
		users = []models.User{}
	}
	return c.JSON(http.StatusOK, users)
}

// ApproveAndLoadBalance: Öğrenciyi onaylar ve ilk bakiyesini loglayarak yükler
func ApproveAndLoadBalance(c echo.Context) error {
	userID := c.Param("id")
	req := new(ApproveRequest)

	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Geçersiz bakiye formatı"})
	}

	// TRANSACTION BAŞLAT: Hem onay hem log aynı anda olmalı
	tx, err := db.Instance.Beginx()
	if err != nil {
		return err
	}

	// Kullanıcıyı Onayla ve Bakiyeyi Güncelle
	_, err = tx.Exec(`UPDATE users SET is_approved = true, balance = balance + $1 WHERE id = $2`, req.Balance, userID)
	if err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Onay işlemi başarısız"})
	}

	// Cüzdan Loguna Yaz (Böylece çocuk geçmişinde "Kantin Tarafından Yüklendi"yi görür)
	if req.Balance > 0 {
		_, err = tx.Exec(`INSERT INTO wallet_transactions (user_id, amount, transaction_type, source) VALUES ($1, $2, 'load', 'admin_panel')`, userID, req.Balance)
		if err != nil {
			tx.Rollback()
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Cüzdan logu oluşturulamadı"})
		}
	}

	tx.Commit()
	return c.JSON(http.StatusOK, map[string]string{"message": "Öğrenci onaylandı ve ilk bakiye tanımlandı!"})
}

func AdminLoadBalance(c echo.Context) error {
	userID := c.Param("id")
	req := new(ApproveRequest)
	if err := c.Bind(req); err != nil || req.Balance <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Geçerli bir tutar girin"})
	}

	tx, _ := db.Instance.Beginx()
	_, err := tx.Exec(`UPDATE users SET balance = balance + $1 WHERE id = $2`, req.Balance, userID)
	if err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Bakiye güncellenemedi"})
	}

	// Log tablosuna 'admin_panel' kaynağıyla ekliyoruz
	_, err = tx.Exec(`INSERT INTO wallet_transactions (user_id, amount, transaction_type, source) VALUES ($1, $2, 'load', 'admin_panel')`, userID, req.Balance)
	if err != nil {
		tx.Rollback()
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "İşlem loglanamadı"})
	}

	tx.Commit()
	return c.JSON(http.StatusOK, map[string]string{"message": "Bakiye yüklendi!"})
}
