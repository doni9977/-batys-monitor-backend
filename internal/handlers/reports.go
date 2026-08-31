package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/gofiber/fiber/v2"
)

// GenerateReportDocx генерирует отчет Word через Python скрипт по конкретному индикатору (алгоритму)
func GenerateReportDocx(c *fiber.Ctx) error {
	indicatorRaw := c.Query("indicator")
	if indicatorRaw == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "indicator is required (e.g. A1, NR1)"})
	}
	
	indicator := strings.ToUpper(indicatorRaw)

	// Find the latest job_id for this indicator
	var latestRisk models.DetectedRisk
	database.DB.Where("indicator = ?", indicator).Order("created_at DESC").First(&latestRisk)

	// Fetch risks only for the latest job
	var risks []models.DetectedRisk
	query := database.DB.Where("indicator = ?", indicator)
	if latestRisk.JobID != 0 {
		query = query.Where("job_id = ?", latestRisk.JobID)
	}
	
	if err := query.Order("amount DESC, clinic_name ASC").Find(&risks).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch risks"})
	}

	// Calculate totals
	var totalAmount float64
	for _, r := range risks {
		totalAmount += r.Amount
	}

	payload := map[string]interface{}{
		"indicator":    indicator,
		"total_risks":  len(risks),
		"total_amount": totalAmount,
		"risks":        risks,
	}

	// Кодируем JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to marshal JSON"})
	}

	// Вызываем Python скрипт
	pythonBin := "../venv/bin/python" 
	if _, err := os.Stat(pythonBin); os.IsNotExist(err) {
		pythonBin = "python3" 
	}
	
	cmd := exec.Command(pythonBin, "scripts/generate_report.py")
	
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create stdin pipe"})
	}
	
	go func() {
		defer stdin.Close()
		io.WriteString(stdin, string(jsonData))
	}()

	outBytes, err := cmd.Output()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("Python script failed: %s", err.Error())})
	}

	outputPath := strings.TrimSpace(string(outBytes))
	
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Generated file not found"})
	}
	
	defer os.Remove(outputPath)
	
	c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=Report_%s.docx", indicator))
	
	return c.SendFile(outputPath)
}
