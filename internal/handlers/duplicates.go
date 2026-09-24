package handlers

import (
	"github.com/gofiber/fiber/v2"
)

// GetRiskDuplicates возвращает уже рассчитанные дубликаты A4 из detected_risks.
func GetRiskDuplicates(c *fiber.Ctx) error {
	risks, total, err := loadRisksByIndicator(c, "A4")
	if err != nil {
		return respondRiskLoadError(c, err, "Ошибка чтения рисков A4")
	}

	return respondRisksWithPagination(c, "A4", risks, total)
}
