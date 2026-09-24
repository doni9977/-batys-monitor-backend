package handlers

import (
	"github.com/gofiber/fiber/v2"
)

// GetRiskA1 читает уже рассчитанные риски из detected_risks.
func GetRiskA1(c *fiber.Ctx) error {
	risks, total, err := loadRisksByIndicator(c, "A1")
	if err != nil {
		return respondRiskLoadError(c, err, "Ошибка чтения рисков A1")
	}

	return respondRisksWithPagination(c, "A1", risks, total)
}

// GetRiskA2 читает уже рассчитанные риски из detected_risks.
func GetRiskA2(c *fiber.Ctx) error {
	risks, total, err := loadRisksByIndicator(c, "A2")
	if err != nil {
		return respondRiskLoadError(c, err, "Ошибка чтения рисков A2")
	}

	return respondRisksWithPagination(c, "A2", risks, total)
}
