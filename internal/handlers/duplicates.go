package handlers

import (
	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/gofiber/fiber/v2"
)

// GetRiskDuplicates возвращает уже рассчитанные дубликаты A4 из detected_risks.
func GetRiskDuplicates(c *fiber.Ctx) error {
	var risks []models.DetectedRisk

	if err := database.DB.
		Where("indicator = ?", "A4").
		Order("risk_date DESC").
		Find(&risks).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Ошибка чтения рисков A4",
		})
	}

	payload := make([]map[string]interface{}, 0, len(risks))
	for _, r := range risks {
		payload = append(payload, riskToJSON(r))
	}

	return c.JSON(fiber.Map{
		"indicator":   "A4",
		"description": "Одинаковая услуга одному пациенту в один день; different_clinics=true означает совпадение в разных клиниках",
		"total_found": len(payload),
		"risks":       payload,
	})
}
