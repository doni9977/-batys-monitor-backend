package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
)

// ExportRisksToXLSX экспортирует обнаруженные риски в Excel (ЗАДАЧА 12)
func ExportRisksToXLSX(c *fiber.Ctx) error {
	// Получаем последний успешный job
	var latestDoneJob models.RiskJob
	err := database.DB.
		Where("status = ?", models.RiskJobStatusDone).
		Order("created_at DESC").
		First(&latestDoneJob).Error
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Нет завершённых задач расчёта рисков",
		})
	}

	// Получаем все риски для этого job
	var risks []models.DetectedRisk
	err = database.DB.Where("job_id = ?", latestDoneJob.ID).
		Order("indicator, risk_date DESC").
		Find(&risks).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Ошибка чтения рисков: " + err.Error(),
		})
	}

	// Создаём новую Excel-книгу
	f := excelize.NewFile()
	defer f.Close()

	// Стиль заголовков
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:  true,
			Color: "#FFFFFF",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#4472C4"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Border: []excelize.Border{
			{Type: "left", Color: "#000000", Style: 1},
			{Type: "right", Color: "#000000", Style: 1},
			{Type: "top", Color: "#000000", Style: 1},
			{Type: "bottom", Color: "#000000", Style: 1},
		},
	})

	// Стиль для даты
	dateStyle, _ := f.NewStyle(&excelize.Style{
		NumFmt: "yyyy-mm-dd",
	})

	// Заголовки колонок
	headers := []string{
		"№",
		"Индикатор",
		"Клиника",
		"Врач",
		"ИИН пациента",
		"Дата риска",
		"Сумма",
		"Детали",
	}

	// Пишем заголовки
	for col, header := range headers {
		cell, _ := excelize.CoordinatesToCellID(col+1, 1)
		f.SetCellValue("Sheet1", cell, header)
		f.SetCellStyle("Sheet1", cell, cell, headerStyle)
	}

	// Устанавливаем ширину колонок
	colWidths := []float64{5, 12, 20, 15, 15, 15, 12, 40}
	for i, width := range colWidths {
		f.SetColWidth("Sheet1", string(rune('A'+i)), string(rune('A'+i)), width)
	}

	// Пишем данные
	for rowIdx, risk := range risks {
		row := rowIdx + 2 // Начинаем со второй строки (первая - заголовки)

		// Детали (JSON)
		var details map[string]interface{}
		json.Unmarshal(risk.Details, &details)
		detailsStr := fmt.Sprintf("%v", details)

		values := []interface{}{
			rowIdx + 1,
			risk.Indicator,
			risk.ClinicName,
			risk.DoctorName,
			risk.PatientIIN,
			risk.RiskDate,
			risk.Amount,
			detailsStr,
		}

		for col, value := range values {
			cell, _ := excelize.CoordinatesToCellID(col+1, row)
			f.SetCellValue("Sheet1", cell, value)

			// Применяем стиль даты для колонки "Дата риска"
			if col == 5 {
				f.SetCellStyle("Sheet1", cell, cell, dateStyle)
			}
		}
	}

	// Устанавливаем фильтры
	f.AutoFilter.Ref = "A1:H" + fmt.Sprintf("%d", len(risks)+1)
	err = f.SetAutoFilter("Sheet1", "A1", fmt.Sprintf("H%d", len(risks)+1), "")

	// Сохраняем в буфер
	buf, err := f.WriteToBuffer()
	if err != nil {
		log.Printf("Ошибка при сохранении Excel: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Ошибка генерации Excel файла: " + err.Error(),
		})
	}

	// Отправляем файл клиенту
	c.Response().Header.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"risks_%s.xlsx\"", time.Now().Format("2006-01-02_15-04-05")))

	return c.Send(buf.Bytes())
}
