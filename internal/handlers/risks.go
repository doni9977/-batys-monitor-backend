package handlers

import "github.com/gofiber/fiber/v2"

func GetRiskA3(c *fiber.Ctx) error {
	return buildRiskResponse(c, "A3", "Аномальная нагрузка врача")
}
