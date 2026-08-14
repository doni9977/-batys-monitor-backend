package handlers

import (
	"path/filepath"

	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/parser"
	"github.com/gofiber/fiber/v2"
)

func UploadClassifierExcel(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Не удалось получить файл классификатора",
		})
	}

	tempPath := filepath.Join("./", file.Filename)
	if err := c.SaveFile(file, tempPath); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Не удалось сохранить временный файл",
		})
	}

	classifiers, err := parser.ParseClassifierExcel(tempPath)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Очищаем старые данные классификатора перед записью новых
	database.DB.Exec("TRUNCATE TABLE service_classifiers;")

	result := database.DB.CreateInBatches(&classifiers, 500)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Ошибка при сохранении классификатора в БД: " + result.Error.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"status":      "success",
		"total_saved": len(classifiers),
		"message":     "Классификатор успешно загружен в базу данных",
	})
}
