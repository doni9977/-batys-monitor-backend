package parser

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/xuri/excelize/v2"
)

func ParseClassifierExcel(filePath string) ([]models.ServiceClassifier, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия файла классификатора: %v", err)
	}
	defer f.Close()

	sheetName := "Классификатор"
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения строк листа '%s': %v", sheetName, err)
	}

	if len(rows) < 2 {
		return nil, fmt.Errorf("лист классификатора пуст")
	}

	headerRow := rows[0]
	columnMap := make(map[string]int)
	for i, colName := range headerRow {
		columnMap[strings.TrimSpace(colName)] = i
	}

	var classifiers []models.ServiceClassifier

	for rowIndex, row := range rows {
		if rowIndex == 0 {
			continue
		}

		getVal := func(colName string) string {
			if idx, ok := columnMap[colName]; ok && idx < len(row) {
				return strings.TrimSpace(row[idx])
			}
			return ""
		}

		tariff, _ := strconv.ParseFloat(getVal("Демо-тариф, ₸"), 64)
		minAge, _ := strconv.Atoi(getVal("Мин. возраст"))
		maxAge, _ := strconv.Atoi(getVal("Макс. возраст"))
		normMinutes, _ := strconv.Atoi(getVal("Норматив, минут"))
		maxPerDay, _ := strconv.Atoi(getVal("Макс. в день"))
		maxPerYear, _ := strconv.Atoi(getVal("Макс. в год"))

		code := getVal("Код услуги")
		if code == "" {
			continue
		}

		item := models.ServiceClassifier{
			Code:              code,
			Name:              getVal("Наименование услуги"),
			Tariff:            tariff,
			MinAge:            minAge,
			MaxAge:            maxAge,
			GenderRestriction: getVal("Ограничение по полу"),
			NormMinutes:       normMinutes,
			MaxPerDay:         maxPerDay,
			MaxPerYear:        maxPerYear,
			IsComplex:         getVal("Сложная услуга"),
			RuleStatus:        getVal("Статус правила"),
			Note:              getVal("Источник / примечание"),
			CreatedAt:         time.Now(),
		}

		classifiers = append(classifiers, item)
	}

	log.Printf("Успешно распарсено %d записей классификатора.\n", len(classifiers))
	return classifiers, nil
}
