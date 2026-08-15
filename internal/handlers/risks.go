package handlers

import (
	"github.com/gofiber/fiber/v2"
)

// GetRiskA3 возвращает готовые записи из единой таблицы detected_risks.
func GetRiskA3(c *fiber.Ctx) error {
	risks, err := loadRisksByIndicator(c, "A3")
	if err != nil {
		return respondRiskLoadError(c, err, "Ошибка чтения рисков A3")
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
