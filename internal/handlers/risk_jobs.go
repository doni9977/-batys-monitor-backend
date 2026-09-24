package handlers

import (
	"errors"
	"strconv"

	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/danialmarat/batys-monitor-backend/internal/services"
	"github.com/gofiber/fiber/v2"
)

func CancelRiskJob(c *fiber.Ctx) error {
	jobID, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil || jobID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Некорректный id job"})
	}
	username, _ := c.Locals("admin_username").(string)

	err = services.CancelJob(uint(jobID), username)
	switch {
	case err == nil:
		return c.JSON(fiber.Map{"status": "success", "message": "Задача отменена"})
	case errors.Is(err, services.ErrJobNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Job не найден"})
	case errors.Is(err, services.ErrJobForbidden):
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Нет доступа к этой задаче"})
	case errors.Is(err, services.ErrJobNotActive):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Задача уже завершена или отменена"})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Не удалось отменить задачу"})
	}
}

func GetRiskJobByID(c *fiber.Ctx) error {
	rawID := c.Params("id")
	parsedID, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil || parsedID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Некорректный id job",
		})
	}

	var job models.RiskJob
	if err := database.DB.First(&job, parsedID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Job не найден",
		})
	}

	return c.JSON(job)
}

func GetLatestRiskJob(c *fiber.Ctx) error {
	var job models.RiskJob
	if err := database.DB.Order("id DESC").First(&job).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Job не найден",
		})
	}

	return c.JSON(job)
}
