package handlers

import (
	"github.com/gofiber/fiber/v2"
)

// GetRiskDuplicates возвращает уже рассчитанные дубликаты A4 из detected_risks.
func GetRiskDuplicates(c *fiber.Ctx) error {
	risks, err := loadRisksByIndicator(c, "A4")
	if err != nil {
		return respondRiskLoadError(c, err, "Ошибка чтения рисков A4")
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
