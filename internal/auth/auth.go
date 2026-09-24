package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

var ErrInvalidToken = errors.New("недействительный или истёкший токен")

func Login(username, password string) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return "", errors.New("логин и пароль обязательны")
	}

	var admin models.Admin
	if err := database.DB.Where("username = ?", username).First(&admin).Error; err != nil {
		return "", errors.New("неверный логин или пароль")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)); err != nil {
		return "", errors.New("неверный логин или пароль")
	}

	now := time.Now()
	jti, err := newJTI()
	if err != nil {
		return "", err
	}
	claims := Claims{
		Username: admin.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   admin.Username,
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(12 * time.Hour)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(jwtSecret()))
}

func newJTI() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func RevokeToken(tokenString string) error {
	claims := &Claims{}
	token, err := parseToken(tokenString, claims)
	if err != nil || !token.Valid || claims.ID == "" || claims.ExpiresAt == nil {
		return ErrInvalidToken
	}

	return database.DB.Where("jti = ?", claims.ID).FirstOrCreate(&models.RevokedToken{
		JTI:       claims.ID,
		ExpiresAt: claims.ExpiresAt.Time,
	}).Error
}

func RequireJWT(c *fiber.Ctx) error {
	if c.Method() == fiber.MethodOptions {
		return c.Next()
	}

	header := c.Get(fiber.HeaderAuthorization)
	var tokenString string

	if strings.HasPrefix(header, "Bearer ") {
		tokenString = strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	} else if c.Query("token") != "" {
		tokenString = c.Query("token")
	} else {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Требуется авторизация"})
	}

	claims := &Claims{}
	token, err := parseToken(tokenString, claims)
	if err != nil || !token.Valid || claims.ID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Недействительный или истёкший токен"})
	}
	if database.DB == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "База данных недоступна"})
	}
	var revoked models.RevokedToken
	err = database.DB.Where("jti = ?", claims.ID).First(&revoked).Error
	if err == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Сессия завершена"})
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Не удалось проверить сессию"})
	}

	c.Locals("admin_username", claims.Username)
	return c.Next()
}

func parseToken(tokenString string, claims *Claims) (*jwt.Token, error) {
	return jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("недопустимый метод подписи")
		}
		return []byte(jwtSecret()), nil
	})
}

func jwtSecret() string {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret == "" {
		return "batys-monitor-development-secret-change-in-production"
	}
	return secret
}
