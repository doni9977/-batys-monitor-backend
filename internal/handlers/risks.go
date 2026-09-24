package handlers

import (
	"github.com/gofiber/fiber/v2"
)

// GetRiskA3 возвращает готовые записи из единой таблицы detected_risks.
func GetRiskA3(c *fiber.Ctx) error {
	risks, total, err := loadRisksByIndicator(c, "A3")
	if err != nil {
		return respondRiskLoadError(c, err, "Ошибка чтения рисков A3")
	}

	return respondRisksWithPagination(c, "A3", risks, total)
}
