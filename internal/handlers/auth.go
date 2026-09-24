package handlers

import (
	"errors"
	"strings"

	"github.com/danialmarat/batys-monitor-backend/internal/auth"
	"github.com/danialmarat/batys-monitor-backend/internal/services"
	"github.com/gofiber/fiber/v2"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func Logout(c *fiber.Ctx) error {
	header := c.Get(fiber.HeaderAuthorization)
	tokenString := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if err := auth.RevokeToken(tokenString); err != nil {
		if errors.Is(err, auth.ErrInvalidToken) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Недействительный или истёкший токен"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Не удалось завершить сессию"})
	}

	username, _ := c.Locals("admin_username").(string)
	if err := services.CancelActiveJobs(username); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Не удалось остановить активные задачи",
		})
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "Сессия завершена",
	})
}

func Login(c *fiber.Ctx) error {
	var input loginRequest
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Некорректный JSON запроса"})
	}

	token, err := auth.Login(input.Username, input.Password)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"status":   "success",
		"token":    token,
		"username": input.Username,
	})
}
