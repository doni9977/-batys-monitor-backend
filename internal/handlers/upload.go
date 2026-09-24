package handlers

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
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

	if err := os.MkdirAll("./tmp", os.ModePerm); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Не удалось создать временную папку"})
	}
	tempPath := filepath.Join("./tmp", filepath.Base(file.Filename))

	if err := c.SaveFile(file, tempPath); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Не удалось сохранить файл на сервере",
		})
	}
	job, err := services.EnqueueRiskCalculationJob(file.Filename, 0)
	if err != nil {
		_ = os.Remove(tempPath)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Не удалось создать задачу обработки"})
	}

	go func() {
		defer os.Remove(tempPath)
		processUploadedExcel(job.ID, file.Filename, tempPath)
	}()

	return c.JSON(fiber.Map{
		"status":      "success",
		"message":     fmt.Sprintf("Файл '%s' принят в обработку.", file.Filename),
		"risk_job_id": job.ID,
	})
}

func processUploadedExcel(jobID uint, fileName string, tempPath string) {
	records, err := parser.ParseExcel(tempPath)
	if err != nil {
		failRiskJob(jobID, fmt.Errorf("ошибка парсинга Excel: %w", err))
		return
	}

	if err := database.DB.Exec("TRUNCATE TABLE service_records RESTART IDENTITY;").Error; err != nil {
		failRiskJob(jobID, fmt.Errorf("не удалось очистить старые данные: %w", err))
		return
	}

	result := database.DB.CreateInBatches(&records, 1000)
	if result.Error != nil {
		failRiskJob(jobID, fmt.Errorf("ошибка сохранения в БД: %w", result.Error))
		return
	}

	database.DB.Model(&models.RiskJob{}).Where("id = ?", jobID).Update("loaded_records", result.RowsAffected)
	log.Printf("Файл %s обработан: %d записей", fileName, result.RowsAffected)
	services.RunAllRiskEngines(jobID)
}

func failRiskJob(jobID uint, cause error) {
	log.Printf("Ошибка обработки job=%d: %v", jobID, cause)
	database.DB.Model(&models.RiskJob{}).Where("id = ?", jobID).Updates(map[string]interface{}{
		"status":        models.RiskJobStatusFailed,
		"finished_at":   time.Now(),
		"error_message": cause.Error(),
	})
}
