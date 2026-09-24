package handlers

import (
	"strconv"

	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/gofiber/fiber/v2"
)

// loadRisksByIndicator загружает риски с поддержкой пагинации и фильтрации по job_id.
// Query params: ?job_id=1&page=1&limit=100&doctor=...&clinic=...
func loadRisksByIndicator(c *fiber.Ctx, indicator string) ([]models.DetectedRisk, int64, error) {
	query := database.DB.Model(&models.DetectedRisk{}).Where("indicator = ?", indicator)

	// Фильтр по job_id
	rawJobID := c.Query("job_id")
	if rawJobID != "" {
		parsedJobID, err := strconv.ParseUint(rawJobID, 10, 64)
		if err != nil || parsedJobID == 0 {
			return nil, 0, fiber.NewError(fiber.StatusBadRequest, "Параметр job_id должен быть положительным числом")
		}
		query = query.Where("job_id = ?", parsedJobID)

		

		
	} else {
		// Определяем домен по префиксу индикатора:
		//   S* → стационар, NR* → нерезиденты, остальное → osms.
		domain := "osms"
		if len(indicator) > 0 && (indicator[0] == 'S' || indicator[0] == 's') {
			domain = "inpatient"
		} else if len(indicator) >= 2 && (indicator[0] == 'N' || indicator[0] == 'n') {
			domain = "nr"
		}

		// Берём последний успешно завершённый job ИМЕННО этого домена.
		var latestDoneJob models.RiskJob
		err := database.DB.
			Where("status = ? AND domain = ?", models.RiskJobStatusDone, domain).
			Order("created_at DESC").
			First(&latestDoneJob).Error
		if err != nil {
			return []models.DetectedRisk{}, 0, nil
		}
		query = query.Where("job_id = ?", latestDoneJob.ID)
	}
	



	// Дополнительные фильтры
	if doctor := c.Query("doctor"); doctor != "" {
		query = query.Where("doctor_name ILIKE ?", "%"+doctor+"%")
	}
	if clinic := c.Query("clinic"); clinic != "" {
		query = query.Where("clinic_name ILIKE ?", "%"+clinic+"%")
	}
	if dateFrom := c.Query("date_from"); dateFrom != "" {
		query = query.Where("risk_date >= ?", dateFrom)
	}
	if dateTo := c.Query("date_to"); dateTo != "" {
		query = query.Where("risk_date <= ?", dateTo)
	}

	// Подсчёт общего числа (до пагинации)
	var total int64
	query.Count(&total)

	// Задача 9: Пагинация
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

// buildRiskResponse собирает финальный JSON-ответ с пагинацией.
func buildRiskResponse(c *fiber.Ctx, indicator, description string) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "100"))
	if limit > 1000 {
		limit = 1000
	}
	if page < 1 {
		page = 1
	}

	risks, total, err := loadRisksByIndicator(c, indicator)
	if err != nil {
		return respondRiskLoadError(c, err, "Ошибка чтения рисков "+indicator)
	}

	totalPages := int(total) / limit
	if int(total)%limit != 0 {
		totalPages++
	}

	payload := make([]map[string]interface{}, 0, len(risks))
	for _, r := range risks {
		payload = append(payload, riskToJSON(r))
	}

	return c.JSON(fiber.Map{
		"indicator":   indicator,
		"description": description,
		"total_found": total,
		"page":        page,
		"limit":       limit,
		"total_pages": totalPages,
		"risks":       payload,
	})
}
