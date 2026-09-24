package handlers

import (
	"fmt"
	"strconv"
	"time"

	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
)

// ExportRisksXlsx экспортирует результаты проверок (по конкретному алгоритму или все) в Excel
func ExportRisksXlsx(c *fiber.Ctx) error {
	indicator := c.Query("indicator")
	jobIDStr := c.Query("job_id")

	var jobID uint64
	if jobIDStr != "" {
		parsed, err := strconv.ParseUint(jobIDStr, 10, 64)
		if err == nil {
			jobID = parsed
		}
	} else {
		var latest models.RiskJob
		if err := database.DB.Where("status = ?", models.RiskJobStatusDone).Order("created_at DESC").First(&latest).Error; err == nil {
			jobID = uint64(latest.ID)
		}
	}

	query := database.DB.Model(&models.DetectedRisk{}).Where("job_id = ?", jobID)
	if indicator != "" {
		query = query.Where("indicator = ?", indicator)
	}

	var risks []models.DetectedRisk
	if err := query.Order("clinic_name ASC, risk_date ASC").Find(&risks).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch risks for export"})
	}

	f := excelize.NewFile()
	defer f.Close()

	sheet := "Risks"
	f.SetSheetName("Sheet1", sheet)

	// Заголовки
	headers := []string{"Поликлиника", "Врач", "ИИН пациента", "Индикатор", "Дата риска", "Сумма ущерба (тг)"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	// Данные
	for i, r := range risks {
		row := i + 2
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), r.ClinicName)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), r.DoctorName)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), r.PatientIIN)
		f.SetCellValue(sheet, fmt.Sprintf("D%d", row), r.Indicator)
		f.SetCellValue(sheet, fmt.Sprintf("E%d", row), r.RiskDate.Format("02.01.2006"))
		f.SetCellValue(sheet, fmt.Sprintf("F%d", row), r.Amount)
	}

	// Стилизация (жирный заголовок)
	style, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	f.SetRowStyle(sheet, 1, 1, style)
	f.SetColWidth(sheet, "A", "F", 20)

	c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=Risks_Export_%s.xlsx", time.Now().Format("2006-01-02_1504")))

	return f.Write(c.Response().BodyWriter())
}
