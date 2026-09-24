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

// UploadExcel принимает Excel-файл с реестром услуг, очищает старые данные,
// загружает новые записи в БД и запускает Risk Engine в фоне.
func UploadExcel(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Файл не найден в запросе. Убедитесь, что поле называется 'file'",
		})
	}

	log.Printf("📥 Получен файл: %s, Размер: %.2f МБ", file.Filename, float64(file.Size)/1024/1024)

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

	log.Printf("✅ Распарсено %d записей. Очищаем старые данные...", len(records))

	// Задача 3 (БАГ #1): TRUNCATE перед загрузкой новых данных.
	// Без этого при повторной загрузке данные дублируются и все счётчики рисков
	// растут вдвое, втрое и т.д.
	if err := database.DB.Exec("TRUNCATE TABLE service_records RESTART IDENTITY;").Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Не удалось очистить старые данные перед загрузкой: " + err.Error(),
		})
	}
	log.Println("🗑️  Старые данные удалены. Начинаем загрузку в PostgreSQL...")

	// Очищаем таблицу перед новой загрузкой (БАГ #1 FIX)
	if err := database.DB.Exec("TRUNCATE TABLE service_records RESTART IDENTITY;").Error; err != nil {
		log.Printf("Предупреждение: не удалось очистить таблицу service_records: %v", err)
	} else {
		log.Println("Таблица service_records очищена.")
	}

	result := database.DB.CreateInBatches(&records, 1000)
	if result.Error != nil {
		log.Printf("Ошибка при сохранении в БД: %v", result.Error)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Ошибка сохранения в БД: %v", result.Error),
		})
	}

	log.Printf("✅ Сохранено %d записей в БД. Запускаем Risk Engine...", result.RowsAffected)

	job, err := services.EnqueueRiskCalculationJob(file.Filename, result.RowsAffected)
	if err != nil {
		log.Printf("Не удалось создать job трекинга: %v", err)
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
		"status":        "success",
		"message":       fmt.Sprintf("Файл '%s' обработан! Загружено %d записей.", file.Filename, result.RowsAffected),
		"loaded_records": result.RowsAffected,
		"risk_job_id":   riskJobID,
	})
}
