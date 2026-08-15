package handlers

import (
	"strconv"

	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/gofiber/fiber/v2"
)

func loadRisksByIndicator(c *fiber.Ctx, indicator string) ([]models.DetectedRisk, error) {
	query := database.DB.Where("indicator = ?", indicator)

	rawJobID := c.Query("job_id")
	if rawJobID != "" {
		parsedJobID, err := strconv.ParseUint(rawJobID, 10, 64)
		if err != nil || parsedJobID == 0 {
			return nil, fiber.NewError(fiber.StatusBadRequest, "Параметр job_id должен быть положительным числом")
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
			return []models.DetectedRisk{}, nil
		}

		query = query.Where("job_id = ?", latestDoneJob.ID)
	}

	var risks []models.DetectedRisk
	if err := query.Order("risk_date DESC, id DESC").Find(&risks).Error; err != nil {
		return nil, err
	}

	return risks, nil
}

func respondRiskLoadError(c *fiber.Ctx, err error, fallbackMessage string) error {
	if fiberErr, ok := err.(*fiber.Error); ok {
		return c.Status(fiberErr.Code).JSON(fiber.Map{"error": fiberErr.Message})
	}

	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fallbackMessage})
}
