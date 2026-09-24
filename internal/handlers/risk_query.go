package handlers

import (
	"math"
	"strconv"

	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/gofiber/fiber/v2"
)

func loadRisksByIndicator(c *fiber.Ctx, indicator string) ([]models.DetectedRisk, int64, error) {
	query := database.DB.Where("indicator = ?", indicator)

	rawJobID := c.Query("job_id")
	if rawJobID != "" {
		parsedJobID, err := strconv.ParseUint(rawJobID, 10, 64)
		if err != nil || parsedJobID == 0 {
			return nil, 0, fiber.NewError(fiber.StatusBadRequest, "Параметр job_id должен быть положительным числом")
		}
		query = query.Where("job_id = ?", parsedJobID)
	} else {
		var latestDoneJob models.RiskJob
		err := database.DB.
			Where("status = ?", models.RiskJobStatusDone).
			Order("created_at DESC").
			First(&latestDoneJob).Error
		if err != nil {
			// Если успешных задач ещё не было, возвращаем пустой результат.
			return []models.DetectedRisk{}, 0, nil
		}

		query = query.Where("job_id = ?", latestDoneJob.ID)
	}

	// Подсчитаем общее количество результатов
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Пагинация (ЗАДАЧА 9 FIX)
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "100"))
	if limit > 1000 {
		limit = 1000
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	var risks []models.DetectedRisk
	if err := query.Offset(offset).Limit(limit).Order("risk_date DESC, id DESC").Find(&risks).Error; err != nil {
		return nil, 0, err
	}

	return risks, total, nil
}

func respondRiskLoadError(c *fiber.Ctx, err error, fallbackMessage string) error {
	if fiberErr, ok := err.(*fiber.Error); ok {
		return c.Status(fiberErr.Code).JSON(fiber.Map{"error": fiberErr.Message})
	}

	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fallbackMessage})
}

func respondRisksWithPagination(c *fiber.Ctx, indicator string, risks []models.DetectedRisk, total int64) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "100"))
	if limit > 1000 {
		limit = 1000
	}
	if page < 1 {
		page = 1
	}

	totalPages := int64(math.Ceil(float64(total) / float64(limit)))

	riskMaps := make([]map[string]interface{}, 0, len(risks))
	for _, r := range risks {
		riskMaps = append(riskMaps, riskToJSON(r))
	}

	return c.JSON(fiber.Map{
		"indicator":   indicator,
		"total_found": total,
		"page":        page,
		"limit":       limit,
		"total_pages": totalPages,
		"risks":       riskMaps,
	})
}
