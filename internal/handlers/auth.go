package handlers

import (
	"log"
	"net/http"
	"os"
	"time"

	"qr-kantin/internal/db"
	"qr-kantin/internal/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
)

// LoginRequest: Giriş yaparken beklediğimiz JSON yapısı
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// RegisterRequest: Kayıt olurken beklediğimiz JSON yapısı
type RegisterRequest struct {
	FullName string `json:"full_name" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
	Role     string `json:"role"` // Kantinci panelinden eklerken lazım olacak
}

// RegisterStudent: Öğrenciler mobil uygulamadan kayıt olur (Public - is_approved FALSE başlar)
func RegisterStudent(c echo.Context) error {
	req := new(RegisterRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Geçersiz veri formatı"})
	}

	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Eksik veya hatalı bilgi: " + err.Error()})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Şifre işlenemedi"})
	}

	//Veritabanında DEFAULT FALSE olduğu için is_approved göndermiyoruz, onaysız kaydediliyor. Kayıt kantinci tarafından onaylanıyor.
	query := `INSERT INTO users (full_name, email, password_hash, role) VALUES ($1, $2, $3, 'ogrenci') RETURNING id`
	var lastID string
	err = db.Instance.QueryRow(query, req.FullName, req.Email, string(hashedPassword)).Scan(&lastID)

	if err != nil {
		log.Println("Öğrenci Kayıt DB Hatası:", err)
		return c.JSON(http.StatusConflict, map[string]string{"error": "Kayıt işlemi başarısız veya e-posta kullanımda"})
	}

	return c.JSON(http.StatusCreated, map[string]string{"message": "Kayıt başarılı! Lütfen hesabınızı kantinden onaylatın.", "id": lastID})
}

// SADECE ADMİN PANELİNDEN YAPILAN KAYITLAR (Otomatik Onaylı)

// RegisterStudentByAdmin: Panelden Öğrenci Ekler
func RegisterStudentByAdmin(c echo.Context) error {
	req := new(RegisterRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Geçersiz veri formatı"})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Şifre işlenemedi"})
	}

	// Rolü ve onayı
	query := `INSERT INTO users (full_name, email, password_hash, role, is_approved) VALUES ($1, $2, $3, 'ogrenci', true) RETURNING id`
	var lastID string
	err = db.Instance.QueryRow(query, req.FullName, req.Email, string(hashedPassword)).Scan(&lastID)

	if err != nil {
		return c.JSON(http.StatusConflict, map[string]string{"error": "Bu e-posta adresi zaten kullanımda"})
	}

	return c.JSON(http.StatusCreated, map[string]string{"message": "Öğrenci eklendi!", "id": lastID})
}

// RegisterTeacherByAdmin: Panelden Öğretmen Ekler
func RegisterTeacherByAdmin(c echo.Context) error {
	req := new(RegisterRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Geçersiz veri formatı"})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Şifre işlenemedi"})
	}

	// Rol: ogretmen, Onay: true
	query := `INSERT INTO users (full_name, email, password_hash, role, is_approved) VALUES ($1, $2, $3, 'ogretmen', true) RETURNING id`
	var lastID string
	err = db.Instance.QueryRow(query, req.FullName, req.Email, string(hashedPassword)).Scan(&lastID)

	if err != nil {
		return c.JSON(http.StatusConflict, map[string]string{"error": "Bu e-posta adresi zaten kullanımda"})
	}

	return c.JSON(http.StatusCreated, map[string]string{"message": "Öğretmen eklendi!", "id": lastID})
}

// RegisterAdminByAdmin: Panelden Yeni Yönetici/Kantinci Ekler
func RegisterAdminByAdmin(c echo.Context) error {
	req := new(RegisterRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Geçersiz veri formatı"})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Şifre işlenemedi"})
	}

	// Rol: admin, Onay: true
	query := `INSERT INTO users (full_name, email, password_hash, role, is_approved) VALUES ($1, $2, $3, 'admin', true) RETURNING id`
	var lastID string
	err = db.Instance.QueryRow(query, req.FullName, req.Email, string(hashedPassword)).Scan(&lastID)

	if err != nil {
		return c.JSON(http.StatusConflict, map[string]string{"error": "Bu e-posta adresi zaten kullanımda"})
	}

	return c.JSON(http.StatusCreated, map[string]string{"message": "Yeni yönetici eklendi!", "id": lastID})
}

// Login: Kimlik doğrulaması yapar
func Login(c echo.Context) error {
	req := new(LoginRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Geçersiz giriş bilgileri"})
	}

	var user models.User
	query := `
		SELECT 
			id, full_name, email, password_hash, role, balance, is_approved,
			COALESCE(created_at, CURRENT_TIMESTAMP) as created_at, 
			COALESCE(updated_at, CURRENT_TIMESTAMP) as updated_at 
		FROM users WHERE email = $1`

	err := db.Instance.Get(&user, query, req.Email)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "E-posta veya şifre hatalı"})
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "E-posta veya şifre hatalı"})
	}

	// --- GÜVENLİK DUVARI: Kullanıcı Onaylı Mı? ---
	if !user.IsApproved {
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": "Erişim Engellendi: Hesabınız henüz onaylanmamış! Lütfen kantinden onaylatın.",
		})
	}
	// --------------------------------------------------------------

	// Her şey tamamsa JWT Token Üret
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":   user.ID,
		"role": user.Role,
		"exp":  time.Now().Add(time.Hour * 24).Unix(), // 1 gün geçerli
	})

	t, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"token": t,
		"user":  user,
	})
}
