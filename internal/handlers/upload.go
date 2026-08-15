package handlers

import (
	"fmt"
	"log"
	"os"

	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/parser"
	"github.com/danialmarat/batys-monitor-backend/internal/services"
	"github.com/gofiber/fiber/v2"
)

func UploadExcel(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Файл не найден в запросе. Убедитесь, что поле называется 'file'",
		})
	}

	log.Printf("Получен файл: %s, Размер: %d байт", file.Filename, file.Size)

	os.MkdirAll("./tmp", os.ModePerm)
	tempPath := fmt.Sprintf("./tmp/%s", file.Filename)

	if err := c.SaveFile(file, tempPath); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Не удалось сохранить файл на сервере",
		})
	}
	defer os.Remove(tempPath)

	records, err := parser.ParseExcel(tempPath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Ошибка парсинга Excel: %v", err),
		})
	}

	log.Println("Начинаем загрузку данных в PostgreSQL...")

	result := database.DB.CreateInBatches(&records, 1000)
	if result.Error != nil {
		log.Printf("Ошибка при сохранении в БД: %v", result.Error)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Ошибка сохранения в БД: %v", result.Error),
		})
	}

	log.Printf("Успешно сохранено %d записей в базу данных!", result.RowsAffected)

	job, err := services.EnqueueRiskCalculationJob(file.Filename, result.RowsAffected)
	if err != nil {
		log.Printf("Не удалось создать job трекинга для расчёта рисков: %v", err)
	}

	go func() {
		jobID := uint(0)
		if job != nil {
			jobID = job.ID
		}
		services.RunAllRiskEngines(jobID)
	}()

	var riskJobID interface{}
	if job != nil {
		riskJobID = job.ID
	}

	return c.JSON(fiber.Map{
		"status":      "success",
		"message":     fmt.Sprintf("Файл '%s' обработан! Загружено %d записей в базу данных.", file.Filename, result.RowsAffected),
		"risk_job_id": riskJobID,
	})
}
