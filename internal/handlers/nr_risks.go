package handlers

import (
	"strconv"

	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/gofiber/fiber/v2"
)

// getNrIndicatorDescription возвращает описание NR-индикатора.
func getNrIndicatorDescription(indicator string) string {
	descs := map[string]string{
		"NR1": "Фиктивное присутствие: регистрация без физического въезда нерезидента в РК",
		"NR2": "Транзитный туризм: групповой ввоз номиналов для регистрации компаний",
		"NR3": "Аффилированные сети посредников: нотариус и переводчик зарегистрировали 3+ компаний",
		"NR4": "Финансовая пустышка: уставной капитал ниже минимального порога",
	}
	if d, ok := descs[indicator]; ok {
		return d
	}
	return indicator
}

// GetNrRisks — универсальный обработчик для получения NR-рисков по индикатору.
func GetNrRisks(indicator string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Пагинация
		page, _ := strconv.Atoi(c.Query("page", "1"))
		limit, _ := strconv.Atoi(c.Query("limit", "50"))
		if page < 1 {
			page = 1
		}
		if limit < 1 || limit > 200 {
			limit = 50
		}
		offset := (page - 1) * limit

		// Последний NR job
		var latestJob models.RiskJob
		err := database.DB.
			Where("status = ? AND domain = 'nr'", models.RiskJobStatusDone).
			Order("created_at DESC").
			First(&latestJob).Error

		if err != nil {
			return c.JSON(fiber.Map{
				"indicator":   indicator,
				"description": getNrIndicatorDescription(indicator),
				"total_found": 0,
				"page":        page,
				"limit":       limit,
				"total_pages": 1,
				"risks":       []models.DetectedRisk{},
			})
		}

		var total int64
		database.DB.Model(&models.DetectedRisk{}).
			Where("job_id = ? AND indicator = ?", latestJob.ID, indicator).
			Count(&total)

		var risks []models.DetectedRisk
		database.DB.
			Where("job_id = ? AND indicator = ?", latestJob.ID, indicator).
			Order("id ASC").
			Limit(limit).
			Offset(offset).
			Find(&risks)

		totalPages := int(total) / limit
		if int(total)%limit != 0 {
			totalPages++
		}
		if totalPages < 1 {
			totalPages = 1
		}

		return c.JSON(fiber.Map{
			"indicator":   indicator,
			"description": getNrIndicatorDescription(indicator),
			"total_found": total,
			"page":        page,
			"limit":       limit,
			"total_pages": totalPages,
			"job_id":      latestJob.ID,
			"risks":       risks,
		})
	}
}
