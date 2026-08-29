package parser

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/xuri/excelize/v2"
)

// ParseBankResponseExcel читает Excel-файл ответов БВУ (банков второго уровня)
// и возвращает срез BankResponse.
//
// Ожидаемые колонки (порядок может отличаться):
//   - Наименование клиента
//   - ИИН/БИН клиента
//   - Номер счета
//   - Валюта
//   - Тип счета
//   - Статус
//   - Баланс
func ParseBankResponseExcel(filePath string) ([]models.BankResponse, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия файла БВУ: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("файл не содержит листов")
	}
	sheetName := sheets[0]

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения листа '%s': %w", sheetName, err)
	}

	if len(rows) < 2 {
		return nil, fmt.Errorf("файл пуст или не содержит данных (нужен хотя бы заголовок + 1 строка)")
	}

	// --- Ищем индексы колонок по заголовку (первая строка) ---
	header := rows[0]
	colIdx := map[string]int{
		"name":    -1,
		"bin":     -1,
		"account": -1,
		"currency": -1,
		"type":    -1,
		"status":  -1,
		"balance": -1,
	}

	for i, h := range header {
		hl := strings.ToLower(strings.TrimSpace(h))
		switch {
		case strings.Contains(hl, "наименование"):
			colIdx["name"] = i
		case strings.Contains(hl, "иин") || strings.Contains(hl, "бин"):
			colIdx["bin"] = i
		case strings.Contains(hl, "номер счет"):
			colIdx["account"] = i
		case strings.Contains(hl, "валют"):
			colIdx["currency"] = i
		case strings.Contains(hl, "тип счет"):
			colIdx["type"] = i
		case strings.Contains(hl, "статус"):
			colIdx["status"] = i
		case strings.Contains(hl, "баланс"):
			colIdx["balance"] = i
		}
	}

	// Проверяем, что нашли хотя бы БИН и Статус
	if colIdx["bin"] == -1 || colIdx["status"] == -1 {
		return nil, fmt.Errorf("не найдены обязательные колонки 'ИИН/БИН клиента' и 'Статус' в заголовке")
	}

	// --- Парсим строки данных ---
	var records []models.BankResponse

	getVal := func(row []string, idx int) string {
		if idx >= 0 && idx < len(row) {
			return strings.TrimSpace(row[idx])
		}
		return ""
	}

	for i := 1; i < len(rows); i++ {
		row := rows[i]
		bin := getVal(row, colIdx["bin"])
		if bin == "" {
			continue
		}

		balanceStr := getVal(row, colIdx["balance"])
		balance := 0.0
		if balanceStr != "" {
			balanceStr = strings.ReplaceAll(balanceStr, ",", ".")
			balanceStr = strings.ReplaceAll(balanceStr, " ", "")
			if v, err := strconv.ParseFloat(balanceStr, 64); err == nil {
				balance = v
			}
		}

		records = append(records, models.BankResponse{
			BIN:           bin,
			CompanyName:   getVal(row, colIdx["name"]),
			AccountNumber: getVal(row, colIdx["account"]),
			Currency:      getVal(row, colIdx["currency"]),
			AccountType:   getVal(row, colIdx["type"]),
			Status:        getVal(row, colIdx["status"]),
			Balance:       balance,
		})
	}

	log.Printf("[Bank Parser] Распарсено %d записей из %s", len(records), filePath)
	return records, nil
}
