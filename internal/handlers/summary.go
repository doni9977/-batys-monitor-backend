package handlers

import (
	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/gofiber/fiber/v2"
)

// GetRiskSummary возвращает сводку по всем алгоритмам одним запросом (ЗАДАЧА 10)
func GetRiskSummary(c *fiber.Ctx) error {
	type summaryRow struct {
		Indicator   string
		RiskCount   int64
		TotalAmount float64
	}

	var rows []summaryRow

	// Получаем последний успешный job
	var latestDoneJob models.RiskJob
	err := database.DB.
		Where("status = ?", models.RiskJobStatusDone).
		Order("created_at DESC").
		First(&latestDoneJob).Error
	if err != nil {
		// Если успешных задач ещё не было, возвращаем пустую сводку
		return c.JSON(fiber.Map{
			"job_id":    nil,
			"summary":   []fiber.Map{},
			"total_all": 0,
		})
	}

	// SQL-запрос для сводки по всем индикаторам
	err = database.DB.Raw(`
		SELECT
			indicator,
			COUNT(*) AS risk_count,
			COALESCE(SUM(amount), 0) AS total_amount
		FROM detected_risks
		WHERE job_id = ?
		GROUP BY indicator
		ORDER BY indicator
	`, latestDoneJob.ID).Scan(&rows).Error

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Ошибка чтения сводки рисков: " + err.Error(),
		})
	}

	// Преобразуем результаты
	summary := make([]fiber.Map, 0, len(rows))
	totalAll := int64(0)

	for _, row := range rows {
		summary = append(summary, fiber.Map{
			"indicator":    row.Indicator,
			"risk_count":   row.RiskCount,
			"total_amount": row.TotalAmount,
		})
		totalAll += row.RiskCount
	}

	return c.JSON(fiber.Map{
		"job_id":     latestDoneJob.ID,
		"created_at": latestDoneJob.CreatedAt,
		"summary":    summary,
		"total_all":  totalAll,
	})
}
