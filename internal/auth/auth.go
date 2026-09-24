package auth

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

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
	claims := Claims{
		Username: admin.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   admin.Username,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(12 * time.Hour)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(jwtSecret()))
}

func RequireJWT(c *fiber.Ctx) error {
	if c.Method() == fiber.MethodOptions {
		return c.Next()
	}

	header := c.Get(fiber.HeaderAuthorization)
	if !strings.HasPrefix(header, "Bearer ") {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Требуется авторизация"})
	}

	tokenString := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("недопустимый метод подписи")
		}
		return []byte(jwtSecret()), nil
	})
	if err != nil || !token.Valid {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Недействительный или истёкший токен"})
	}

	c.Locals("admin_username", claims.Username)
	return c.Next()
}

func jwtSecret() string {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret == "" {
		return "batys-monitor-development-secret-change-in-production"
	}
	return secret
}
