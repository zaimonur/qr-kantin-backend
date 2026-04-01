package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

func JWTMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Authorization header'ı kontrol et
		authHeader := c.Request().Header.Get("Authorization")
		if authHeader == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Yetkilendirme başlığı eksik"})
		}

		// "Bearer <token>" formatını ayır
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Geçersiz yetkilendirme formatı"})
		}

		tokenString := parts[1]

		// Token'ı doğrula
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		})

		if err != nil || !token.Valid {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Geçersiz veya süresi dolmuş token"})
		}

		// Token içindeki bilgileri (id, role) context'e ekle
		claims := token.Claims.(jwt.MapClaims)
		c.Set("user_id", claims["id"])
		c.Set("user_role", claims["role"])

		return next(c)
	}
}

// AdminMiddleware: Sadece yetkili personellerin geçmesine izin verir
func AdminMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Tip dönüşümünü (string) güvenli yapıyoruz ki eski token'larda veya eksik verilerde sistem patlamasın
		role, ok := c.Get("user_role").(string)

		if !ok || (role != "admin") {
			return c.JSON(http.StatusForbidden, map[string]string{
				"error": "Erişim Reddedildi: Bu işlem için kantin yetkiniz bulunmuyor!",
			})
		}

		return next(c)
	}
}
