package handlers

import "github.com/gofiber/fiber/v2"

func GetRiskA3(c *fiber.Ctx) error {
	return buildRiskResponse(c, "A3", "Аномальная нагрузка врача")
}
func GetRiskS1(c *fiber.Ctx) error {
	return buildRiskResponse(c, "S1", "Пересечение стационара и поликлиники")
}

func GetRiskS2(c *fiber.Ctx) error {
	return buildRiskResponse(c, "S2", "Дробление госпитализаций")
}

func GetRiskS3(c *fiber.Ctx) error {
	return buildRiskResponse(c, "S3", "Фиктивный круглосуточный стационар")
}

func GetRiskS4(c *fiber.Ctx) error {
	return buildRiskResponse(c, "S4", "Аномалия экстренной госпитализации")
}

func GetRiskS5(c *fiber.Ctx) error {
	return buildRiskResponse(c, "S5", "Услуги после смерти")
}