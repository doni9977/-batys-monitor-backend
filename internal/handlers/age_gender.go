package handlers

import "github.com/gofiber/fiber/v2"

func GetRiskA1(c *fiber.Ctx) error {
	return buildRiskResponse(c, "A1", "Возраст пациента не соответствует требованиям классификатора услуг")
}

func GetRiskA2(c *fiber.Ctx) error {
	return buildRiskResponse(c, "A2", "Пол пациента не соответствует требованиям классификатора услуг")
}
