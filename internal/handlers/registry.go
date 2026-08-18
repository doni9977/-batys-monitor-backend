package handlers

import (
	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/gofiber/fiber/v2"
)

// GetRegistry возвращает список поставщиков (клиник) с агрегированными данными о рисках:
// название (из поля clinic_name), сумма риска, кол-во нарушений — для страницы "Реестр субъектов".
func GetRegistry(c *fiber.Ctx) error {
	type RegistryRow struct {
		ClinicName  string  `json:"clinic_name"`
		TotalAmount float64 `json:"total_amount"`
		TotalRisks  int64   `json:"total_risks"`
		Bin         string  `json:"bin"`
		District    string  `json:"district"`
	}

	domain := c.Query("domain", "osms")

	// Берём только последний успешный job для конкретного домена
	var latestJob models.RiskJob
	err := database.DB.
		Where("status = ? AND domain = ?", models.RiskJobStatusDone, domain).
		Order("created_at DESC").
		First(&latestJob).Error

	if err != nil {
		// Нет завершённых задач — реестр пуст
		return c.JSON(fiber.Map{
			"subjects": []RegistryRow{},
		})
	}

	var rows []RegistryRow
	err = database.DB.Model(&models.DetectedRisk{}).
		Select("clinic_name, SUM(amount) as total_amount, COUNT(id) as total_risks, MAX(details->>'bin') as bin, MAX(details->>'legal_address') as district").
		Where("job_id = ? AND BTRIM(COALESCE(clinic_name,'')) != ''", latestJob.ID).
		Group("clinic_name").
		Order("total_amount DESC").
		Scan(&rows).Error

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Ошибка при формировании реестра: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"job_id":   latestJob.ID,
		"subjects": rows,
	})
}
