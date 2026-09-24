package handlers

import (
	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/gofiber/fiber/v2"
)

// GetClinicsRisks возвращает список клиник с суммарным риск-баллом для карты (ЗАДАЧА 11)
func GetClinicsRisks(c *fiber.Ctx) error {
	type clinicRiskRow struct {
		ClinicName     string
		TotalRisks     int64
		TotalAmount    float64
		A1Count        int64
		A2Count        int64
		A3Count        int64
		A4Count        int64
		A7Count        int64
		A8Count        int64
		A10Count       int64
		RiskScoreLevel string // "low", "medium", "high", "critical"
	}

	var rows []clinicRiskRow

	// Получаем последний успешный job
	var latestDoneJob models.RiskJob
	err := database.DB.
		Where("status = ?", models.RiskJobStatusDone).
		Order("created_at DESC").
		First(&latestDoneJob).Error
	if err != nil {
		// Если успешных задач ещё не было, возвращаем пустой список
		return c.JSON(fiber.Map{
			"clinics": []fiber.Map{},
			"job_id":  nil,
		})
	}

	// Подробный SQL для агрегации по клиникам
	err = database.DB.Raw(`
		SELECT
			COALESCE(clinic_name, 'Неизвестная клиника') AS clinic_name,
			COUNT(*) AS total_risks,
			COALESCE(SUM(amount), 0) AS total_amount,
			SUM(CASE WHEN indicator = 'A1' THEN 1 ELSE 0 END) AS a1_count,
			SUM(CASE WHEN indicator = 'A2' THEN 1 ELSE 0 END) AS a2_count,
			SUM(CASE WHEN indicator = 'A3' THEN 1 ELSE 0 END) AS a3_count,
			SUM(CASE WHEN indicator = 'A4' THEN 1 ELSE 0 END) AS a4_count,
			SUM(CASE WHEN indicator = 'A7' THEN 1 ELSE 0 END) AS a7_count,
			SUM(CASE WHEN indicator = 'A8' THEN 1 ELSE 0 END) AS a8_count,
			SUM(CASE WHEN indicator = 'A10' THEN 1 ELSE 0 END) AS a10_count,
			CASE
				WHEN COUNT(*) > 500 THEN 'critical'
				WHEN COUNT(*) > 200 THEN 'high'
				WHEN COUNT(*) > 50 THEN 'medium'
				ELSE 'low'
			END AS risk_score_level
		FROM detected_risks
		WHERE job_id = ?
		GROUP BY clinic_name
		ORDER BY total_risks DESC
	`, latestDoneJob.ID).Scan(&rows).Error

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Ошибка чтения рисков по клиникам: " + err.Error(),
		})
	}

	// Преобразуем результаты
	clinics := make([]fiber.Map, 0, len(rows))
	for _, row := range rows {
		clinics = append(clinics, fiber.Map{
			"clinic_name":  row.ClinicName,
			"total_risks":  row.TotalRisks,
			"total_amount": row.TotalAmount,
			"risks_by_indicator": fiber.Map{
				"A1":  row.A1Count,
				"A2":  row.A2Count,
				"A3":  row.A3Count,
				"A4":  row.A4Count,
				"A7":  row.A7Count,
				"A8":  row.A8Count,
				"A10": row.A10Count,
			},
			"risk_score_level": row.RiskScoreLevel,
		})
	}

	return c.JSON(fiber.Map{
		"job_id":  latestDoneJob.ID,
		"clinics": clinics,
	})
}
