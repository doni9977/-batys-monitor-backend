package handlers

import (
	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/gofiber/fiber/v2"
)

// GetRiskA1 читает уже рассчитанные риски из detected_risks.
func GetRiskA1(c *fiber.Ctx) error {
	var risks []models.DetectedRisk

	if err := database.DB.
		Where("indicator = ?", "A1").
		Order("risk_date DESC").
		Find(&risks).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Ошибка чтения рисков A1",
		})
	}

	payload := make([]map[string]interface{}, 0, len(risks))
	for _, r := range risks {
		payload = append(payload, riskToJSON(r))
	}

	return c.JSON(fiber.Map{
		"indicator":   "A1",
		"description": "Возраст пациента не соответствует требованиям классификатора услуг",
		"total_found": len(payload),
		"risks":       payload,
	})
}

// GetRiskA2 читает уже рассчитанные риски из detected_risks.
func GetRiskA2(c *fiber.Ctx) error {
	var risks []models.DetectedRisk

	if err := database.DB.
		Where("indicator = ?", "A2").
		Order("risk_date DESC").
		Find(&risks).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Ошибка чтения рисков A2",
		})
	}

	payload := make([]map[string]interface{}, 0, len(risks))
	for _, r := range risks {
		payload = append(payload, riskToJSON(r))
	}

	return c.JSON(fiber.Map{
		"indicator":   "A2",
		"description": "Пол пациента не соответствует требованиям классификатора услуг",
		"total_found": len(payload),
		"risks":       payload,
	})
}
