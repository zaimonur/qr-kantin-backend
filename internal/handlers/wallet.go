package handlers

import (
	"log"
	"net/http"
	"qr-kantin/internal/db"

	"github.com/labstack/echo/v4"
)

type AddBalanceRequest struct {
	Amount float64 `json:"amount"`
}

type WalletTransaction struct {
	Amount          float64 `db:"amount" json:"amount"`
	TransactionType string  `db:"transaction_type" json:"type"`
	Source          string  `db:"source" json:"source"`
	CreatedAt       string  `db:"created_at" json:"created_at"`
}

func AddBalance(c echo.Context) error {
	userID := c.Get("user_id").(string)
	req := new(AddBalanceRequest)

	if err := c.Bind(req); err != nil || req.Amount <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Geçerli bir tutar girin"})
	}

	// Bakiyeyi güvenli bir şekilde artır
	_, err := db.Instance.Exec(`UPDATE users SET balance = balance + $1 WHERE id = $2`, req.Amount, userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Bakiye yüklenemedi"})
	}

	// İşlemi log tablosuna (wallet_transactions) "load" olarak kaydet
	_, err = db.Instance.Exec(`INSERT INTO wallet_transactions (user_id, amount, transaction_type, source) VALUES ($1, $2, 'load', 'mobile_app')`, userID, req.Amount)
	if err != nil {
		log.Println("Cüzdan Log Hatası:", err) // Terminalde görelim, sistemi patlatmaya gerek yok
	}

	// Yeni bakiyeyi çekip kullanıcıya dönelim
	var newBalance float64
	err = db.Instance.Get(&newBalance, `SELECT balance FROM users WHERE id = $1`, userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Güncel bakiye okunamadı"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":     "Bakiye yüklendi!",
		"new_balance": newBalance,
	})
}

func GetBalance(c echo.Context) error {
	userID := c.Get("user_id").(string)

	var balance float64
	err := db.Instance.Get(&balance, `SELECT balance FROM users WHERE id = $1`, userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Bakiye okunamadı"})
	}

	return c.JSON(http.StatusOK, map[string]float64{
		"balance": balance,
	})
}

// Müşterinin işlem geçmişini çeken API fonksiyonu
func GetWalletHistory(c echo.Context) error {
	userID := c.Get("user_id").(string)
	var history []WalletTransaction

	query := `
		SELECT amount, transaction_type, source, TO_CHAR(created_at AT TIME ZONE 'Europe/Istanbul', 'YYYY-MM-DD HH24:MI') as created_at
		FROM wallet_transactions
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	err := db.Instance.Select(&history, query, userID)
	if err != nil {
		log.Println("Geçmiş Çekme Hatası:", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "İşlem geçmişi alınamadı"})
	}

	if history == nil {
		history = []WalletTransaction{}
	}

	return c.JSON(http.StatusOK, history)
}
