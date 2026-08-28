package handlers

import (
	"fmt"
	"log"
	"os"

	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/parser"
	"github.com/gofiber/fiber/v2"
)

// UploadBankResponses принимает Excel-файл с ответами БВУ (банков),
// очищает старые данные bank_responses и загружает новые.
// После загрузки NR5 алгоритм сработает автоматически при следующем запуске NR Engine.
func UploadBankResponses(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Файл не найден. Убедитесь, что поле называется 'file'",
		})
	}

	log.Printf("📥 [BANK] Получен файл: %s, Размер: %.2f КБ", file.Filename, float64(file.Size)/1024)

	os.MkdirAll("./tmp", os.ModePerm)
	tempPath := fmt.Sprintf("./tmp/bank_%s", file.Filename)
	if err := c.SaveFile(file, tempPath); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Не удалось сохранить файл",
		})
	}
	defer os.Remove(tempPath)

	// Парсим Excel
	records, err := parser.ParseBankResponseExcel(tempPath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Ошибка парсинга: %v", err),
		})
	}
	if len(records) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Файл не содержит данных или формат не распознан",
		})
	}

	// Очищаем старые данные БВУ
	database.DB.Exec("TRUNCATE TABLE bank_responses RESTART IDENTITY")
	log.Printf("[BANK] Очищены старые данные. Загружаем %d записей...", len(records))

	// Сохраняем
	res := database.DB.CreateInBatches(&records, 500)
	if res.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Ошибка сохранения в БД: " + res.Error.Error(),
		})
	}

	log.Printf("✅ [BANK] Сохранено %d записей ответов БВУ", res.RowsAffected)

	return c.JSON(fiber.Map{
		"status":         "success",
		"message":        fmt.Sprintf("Загружено %d ответов БВУ из файла '%s'.", res.RowsAffected, file.Filename),
		"loaded_records": res.RowsAffected,
	})
}
