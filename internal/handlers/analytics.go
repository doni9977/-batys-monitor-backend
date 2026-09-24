package handlers

import (
	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/gofiber/fiber/v2"
)

// GetAnalytics возвращает агрегированные данные для страницы "Аналитика и Тренды".
// Включает:
//   - KPI: общая сумма, кол-во нарушений, кол-во клиник, кол-во критических клиник
//   - Динамика нарушений по месяцам (из risk_date detected_risks)
//   - Разбивка по индикаторам (A1..A10)
//   - Разбивка по клиникам (топ-10)
func GetAnalytics(c *fiber.Ctx) error {
	domain := c.Query("domain", "osms")
	
	// Берём последний успешный job
	var latestJob models.RiskJob
	err := database.DB.
		Where("status = ? AND domain = ?", models.RiskJobStatusDone, domain).
		Order("created_at DESC").
		First(&latestJob).Error

	if err != nil {
		return c.JSON(fiber.Map{
			"kpi":        fiber.Map{"total_amount": 0, "total_risks": 0, "unique_clinics": 0, "critical_clinics": 0, "latest_date": ""},
			"by_month":   []fiber.Map{},
			"by_indicator": []fiber.Map{},
			"by_clinic":  []fiber.Map{},
		})
	}

	jobID := latestJob.ID

	// ── KPI ──────────────────────────────────────────────────────────────────
	type KpiRow struct {
		TotalAmount    float64 `json:"total_amount"`
		TotalRisks     int64   `json:"total_risks"`
		UniqueClinics  int64   `json:"unique_clinics"`
		LatestDate     string  `json:"latest_date"`
	}
	var kpi KpiRow
	database.DB.Model(&models.DetectedRisk{}).
		Select(`SUM(amount) as total_amount, COUNT(id) as total_risks,
		        COUNT(DISTINCT clinic_name) as unique_clinics,
		        MAX(risk_date::date) as latest_date`).
		Where("job_id = ?", jobID).
		Scan(&kpi)

	// Критические клиники — сумма ущерба > 5 000 000
	type CritRow struct{ Count int64 }
	var crit CritRow
	database.DB.Raw(`
		SELECT COUNT(*) as count FROM (
			SELECT clinic_name, SUM(amount) as s
			FROM detected_risks
			WHERE job_id = ?
			  AND BTRIM(COALESCE(clinic_name,'')) != ''
			GROUP BY clinic_name
			HAVING SUM(amount) > 5000000
		) t`, jobID).Scan(&crit)

	// ── По месяцам ───────────────────────────────────────────────────────────
	type MonthRow struct {
		Month  string  `json:"month"`
		Amount float64 `json:"amount"`
		Count  int64   `json:"count"`
	}
	var byMonth []MonthRow
	database.DB.Raw(`
		SELECT
			TO_CHAR(risk_date, 'YYYY-MM') as month,
			SUM(amount)                   as amount,
			COUNT(id)                     as count
		FROM detected_risks
		WHERE job_id = ?
		GROUP BY TO_CHAR(risk_date, 'YYYY-MM')
		ORDER BY month ASC`, jobID).Scan(&byMonth)

	// ── По индикатору ────────────────────────────────────────────────────────
	type IndRow struct {
		Indicator string  `json:"indicator"`
		Amount    float64 `json:"amount"`
		Count     int64   `json:"count"`
	}
	var byIndicator []IndRow
	database.DB.Model(&models.DetectedRisk{}).
		Select("indicator, SUM(amount) as amount, COUNT(id) as count").
		Where("job_id = ?", jobID).
		Group("indicator").
		Order("count DESC").
		Scan(&byIndicator)

	// ── Топ-10 клиник ────────────────────────────────────────────────────────
	type ClinicRow struct {
		ClinicName string  `json:"clinic_name"`
		Amount     float64 `json:"amount"`
		Count      int64   `json:"count"`
	}
	var byClinic []ClinicRow
	database.DB.Model(&models.DetectedRisk{}).
		Select("clinic_name, SUM(amount) as amount, COUNT(id) as count").
		Where("job_id = ? AND BTRIM(COALESCE(clinic_name,'')) != ''", jobID).
		Group("clinic_name").
		Order("amount DESC").
		Limit(10).
		Scan(&byClinic)

	return c.JSON(fiber.Map{
		"job_id": jobID,
		"kpi": fiber.Map{
			"total_amount":     kpi.TotalAmount,
			"total_risks":      kpi.TotalRisks,
			"unique_clinics":   kpi.UniqueClinics,
			"critical_clinics": crit.Count,
			"latest_date":      kpi.LatestDate,
		},
		"by_month":     byMonth,
		"by_indicator": byIndicator,
		"by_clinic":    byClinic,
	})
}
