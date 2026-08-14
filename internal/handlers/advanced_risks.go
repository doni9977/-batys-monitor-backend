package handlers

import (
	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/gofiber/fiber/v2"
)

// GetRiskA7 reads already calculated records from detected_risks.
func GetRiskA7(c *fiber.Ctx) error {
	var risks []models.DetectedRisk

	if err := database.DB.
		Where("indicator = ?", "A7").
		Order("risk_date DESC").
		Find(&risks).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Ошибка чтения рисков A7"})
	}

	payload := make([]map[string]interface{}, 0, len(risks))
	for _, r := range risks {
		payload = append(payload, riskToJSON(r))
	}

	return c.JSON(fiber.Map{
		"indicator":   "A7",
		"description": "Накопительный объём услуг пациенту за год превышает лимит",
		"total_found": len(payload),
		"risks":       payload,
	})
}

// GetRiskA8 reads already calculated records from detected_risks.
func GetRiskA8(c *fiber.Ctx) error {
	var risks []models.DetectedRisk

	if err := database.DB.
		Where("indicator = ?", "A8").
		Order("risk_date DESC").
		Find(&risks).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Ошибка чтения рисков A8"})
	}

	payload := make([]map[string]interface{}, 0, len(risks))
	for _, r := range risks {
		payload = append(payload, riskToJSON(r))
	}

	return c.JSON(fiber.Map{
		"indicator":   "A8",
		"description": "Сумма услуги превышает тариф классификатора с учётом количества",
		"total_found": len(payload),
		"risks":       payload,
	})
}

// GetRiskA10 reads already calculated records from detected_risks.
func GetRiskA10(c *fiber.Ctx) error {
	var risks []models.DetectedRisk

	if err := database.DB.
		Where("indicator = ?", "A10").
		Order("risk_date DESC").
		Find(&risks).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Ошибка чтения рисков A10"})
	}

	payload := make([]map[string]interface{}, 0, len(risks))
	for _, r := range risks {
		payload = append(payload, riskToJSON(r))
	}

	return c.JSON(fiber.Map{
		"indicator":   "A10",
		"description": "Интервал между услугами врача меньше норматива",
		"total_found": len(payload),
		"risks":       payload,
	})
}
