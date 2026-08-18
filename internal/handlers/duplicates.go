package handlers

import "github.com/gofiber/fiber/v2"

func GetRiskDuplicates(c *fiber.Ctx) error {
	return buildRiskResponse(c, "A4", "Превышение дневного лимита услуг для пациента (или кросс-клиничное дублирование)")
}
