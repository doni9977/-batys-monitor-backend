package handlers

import (
	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/gofiber/fiber/v2"
)

// GetClinicRisks агрегирует количество нарушений по каждой поликлинике для отображения на карте
func GetClinicRisks(c *fiber.Ctx) error {
	type ClinicStat struct {
		ClinicName string `json:"clinic_name"`
		TotalRisks int64  `json:"total_risks"`
	}

	var stats []ClinicStat

	var latestDoneJob models.RiskJob
	err := database.DB.
		Where("status = ?", models.RiskJobStatusDone).
		Order("created_at DESC").
		First(&latestDoneJob).Error

	if err != nil {
		return c.JSON(fiber.Map{
			"clinics": []ClinicStat{},
		})
	}

	err = database.DB.Model(&models.DetectedRisk{}).
		Select("clinic_name, COUNT(id) as total_risks").
		Where("job_id = ? AND clinic_name != ''", latestDoneJob.ID).
		Group("clinic_name").
		Order("total_risks DESC").
		Scan(&stats).Error

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Ошибка получения статистики по клиникам",
		})
	}

	return c.JSON(fiber.Map{
		"job_id":  latestDoneJob.ID,
		"clinics": stats,
	})
}
