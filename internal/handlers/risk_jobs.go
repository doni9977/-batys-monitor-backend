package handlers

import (
	"strconv"

	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/gofiber/fiber/v2"
)

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
