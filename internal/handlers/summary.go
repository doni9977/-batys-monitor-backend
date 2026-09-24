package handlers

import (
	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/gofiber/fiber/v2"
)

// GetRiskSummary возвращает сводную статистику по всем найденным рискам
func GetRiskSummary(c *fiber.Ctx) error {
	type IndicatorStat struct {
		Indicator string  `json:"indicator"`
		Count     int64   `json:"count"`
		Amount    float64 `json:"amount"`
	}

	var stats []IndicatorStat

	// Получаем последний job
	var latestDoneJob models.RiskJob
	err := database.DB.
		Where("status = ?", models.RiskJobStatusDone).
		Order("created_at DESC").
		First(&latestDoneJob).Error

	if err != nil {
		return c.JSON(fiber.Map{
			"total_risks": 0,
			"indicators":  []IndicatorStat{},
		})
	}

	// Агрегируем риски по индикаторам для последнего файла
	err = database.DB.Model(&models.DetectedRisk{}).
		Select("indicator, COUNT(id) as count, SUM(amount) as amount").
		Where("job_id = ?", latestDoneJob.ID).
		Group("indicator").
		Scan(&stats).Error

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Ошибка агрегации данных: " + err.Error(),
		})
	}

	var totalRisks int64
	var totalAmount float64
	for _, stat := range stats {
		totalRisks += stat.Count
		totalAmount += stat.Amount
	}

	return c.JSON(fiber.Map{
		"job_id":       latestDoneJob.ID,
		"source_file":  latestDoneJob.SourceFile,
		"total_risks":  totalRisks,
		"total_amount": totalAmount,
		"indicators":   stats,
	})
}
