package handlers

import (
	"github.com/gofiber/fiber/v2"
)

// GetRiskA1 читает уже рассчитанные риски из detected_risks.
func GetRiskA1(c *fiber.Ctx) error {
	risks, err := loadRisksByIndicator(c, "A1")
	if err != nil {
		return respondRiskLoadError(c, err, "Ошибка чтения рисков A1")
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
	risks, err := loadRisksByIndicator(c, "A2")
	if err != nil {
		return respondRiskLoadError(c, err, "Ошибка чтения рисков A2")
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
