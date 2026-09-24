package handlers

import (
	"github.com/gofiber/fiber/v2"
)

// GetRiskA7 reads already calculated records from detected_risks.
func GetRiskA7(c *fiber.Ctx) error {
	risks, total, err := loadRisksByIndicator(c, "A7")
	if err != nil {
		return respondRiskLoadError(c, err, "Ошибка чтения рисков A7")
	}

	return respondRisksWithPagination(c, "A7", risks, total)
}

// GetRiskA8 reads already calculated records from detected_risks.
func GetRiskA8(c *fiber.Ctx) error {
	risks, total, err := loadRisksByIndicator(c, "A8")
	if err != nil {
		return respondRiskLoadError(c, err, "Ошибка чтения рисков A8")
	}

	return respondRisksWithPagination(c, "A8", risks, total)
}

// GetRiskA10 reads already calculated records from detected_risks.
func GetRiskA10(c *fiber.Ctx) error {
	risks, total, err := loadRisksByIndicator(c, "A10")
	if err != nil {
		return respondRiskLoadError(c, err, "Ошибка чтения рисков A10")
	}

	return respondRisksWithPagination(c, "A10", risks, total)
}
