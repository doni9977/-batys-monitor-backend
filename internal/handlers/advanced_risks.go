package handlers

import "github.com/gofiber/fiber/v2"

func GetRiskA7(c *fiber.Ctx) error {
	return buildRiskResponse(c, "A7", "Накопительный объём услуг пациенту за год превышает лимит")
}

func GetRiskA8(c *fiber.Ctx) error {
	return buildRiskResponse(c, "A8", "Сумма услуги превышает тариф классификатора с учётом количества")
}

func GetRiskA10(c *fiber.Ctx) error {
	return buildRiskResponse(c, "A10", "Интервал между услугами врача меньше норматива")
}
