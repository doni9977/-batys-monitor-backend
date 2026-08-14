package handlers

import (
	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/gofiber/fiber/v2"
)

// GetRiskA3 возвращает готовые записи из единой таблицы detected_risks.
func GetRiskA3(c *fiber.Ctx) error {
	var risks []models.DetectedRisk

	if err := database.DB.
		Where("indicator = ?", "A3").
		Order("risk_date DESC").
		Find(&risks).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Ошибка чтения рисков A3",
		})
	}

	payload := make([]map[string]interface{}, 0, len(risks))
	for _, r := range risks {
		payload = append(payload, riskToJSON(r))
	}

	return c.JSON(fiber.Map{
		"indicator":   "A3",
		"description": "Аномальная нагрузка врача",
		"total_found": len(payload),
		"risks":       payload,
	})
}
