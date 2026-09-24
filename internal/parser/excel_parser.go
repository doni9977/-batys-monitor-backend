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

func ParseExcel(filePath string) ([]models.ServiceRecord, error) {
	var records []models.ServiceRecord
	err := ParseExcelBatched(filePath, 5000, func(batch []models.ServiceRecord) error {
		records = append(records, batch...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return records, nil
}

// ParseExcelBatched reads TDSheet incrementally and passes each batch to onBatch.
// It avoids keeping the complete workbook contents in memory.
func ParseExcelBatched(filePath string, batchSize int, onBatch func([]models.ServiceRecord) error) error {
	if batchSize < 1 {
		return fmt.Errorf("размер батча должен быть положительным")
	}

	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return fmt.Errorf("ошибка открытия файла: %v", err)
	}
	defer f.Close()

	sheetName := "TDSheet"
	rows, err := f.Rows(sheetName)
	if err != nil {
		return fmt.Errorf("ошибка получения строк листа: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return fmt.Errorf("файл пуст или не содержит данных")
	}

	// RawCellValue сохраняет серийный номер Excel для ячеек дат. Это позволяет
	// корректно обработать даты независимо от формата отображения в книге.
	headerRow, err := rows.Columns(excelize.Options{RawCellValue: true})
	if err != nil {
		return fmt.Errorf("ошибка чтения заголовка: %v", err)
	}

	// Храним заголовки в исходном порядке: map здесь давала случайный результат,
	// когда несколько колонок содержали одно и то же ключевое слово.
	columnNames := make([]string, len(headerRow))
	for i, colName := range headerRow {
		columnNames[i] = normalizeHeader(colName)
	}

	// Функция умного поиска индекса колонки по ключевым словам
	findColIndex := func(keywords ...string) int {
		for _, kw := range keywords {
			kwLower := normalizeHeader(kw)
			for idx, colName := range columnNames {
				if strings.Contains(colName, kwLower) {
					return idx
				}
			}
		}
		return -1
	}

	// Находим ключевые индексы автоматически
	// «Период услуги» содержит дату и время процедуры, поэтому он важнее
	// отдельной колонки «Дата услуги» для расчёта интервалов A10.
	dateIdx := findColIndex("период услуги", "дата услуги", "период", "дата")
	doctorIdx := findColIndex("врач", "специалист")
	iinIdx := findColIndex("иин", "инн")
	patientNameIdx := findColIndex("пациенты")
	genderIdx := findColIndex("пол")
	dobIdx := findColIndex("рождения", "дата рожд")
	codeIdx := findColIndex("код услуги", "код")
	serviceNameIdx := findColIndex("наименование услуги", "услуга")
	qtyIdx := findColIndex("количеств")
	amountIdx := findColIndex("сумм")
	diagIdx := findColIndex("мкб", "диагноз")
	clinicIdx := findColIndex("поставщик", "организация", "клиника", "клиник")

	batch := make([]models.ServiceRecord, 0, batchSize)
	totalRecords := 0

	workbookProps, err := f.GetWorkbookProps()
	if err != nil {
		return fmt.Errorf("ошибка чтения свойств книги: %v", err)
	}
	use1904Dates := workbookProps.Date1904 != nil && *workbookProps.Date1904

	for rows.Next() {
		row, err := rows.Columns(excelize.Options{RawCellValue: true})
		if err != nil {
			return fmt.Errorf("ошибка чтения строки Excel: %v", err)
		}

		getValByIndex := func(idx int) string {
			if idx != -1 && idx < len(row) {
				return strings.TrimSpace(row[idx])
			}
			return ""
		}

		quantity, _ := strconv.Atoi(getValByIndex(qtyIdx))
		amount, _ := strconv.ParseFloat(getValByIndex(amountIdx), 64)

		serviceDate := parseServiceDate(getValByIndex(dateIdx), use1904Dates)

		doctorName := normalizeName(getValByIndex(doctorIdx))
		if doctorName == "" || isIgnorableName(doctorName) {
			continue
		}

		record := models.ServiceRecord{
			ClinicName:     normalizeName(getValByIndex(clinicIdx)),
			DoctorName:     doctorName,
			PatientIIN:     getValByIndex(iinIdx),
			PatientName:    normalizeName(getValByIndex(patientNameIdx)),
			PatientGender:  getValByIndex(genderIdx),
			PatientDOB:     getValByIndex(dobIdx),
			ServiceCode:    getValByIndex(codeIdx),
			ServiceName:    getValByIndex(serviceNameIdx),
			ServiceDate:    serviceDate,
			Quantity:       quantity,
			Amount:         amount,
			DiagnosisMKB10: getValByIndex(diagIdx),
			CreatedAt:      time.Now(),
		}

		if record.PatientIIN == "" || record.ServiceCode == "" {
			continue
		}

		batch = append(batch, record)
		totalRecords++
		if len(batch) == batchSize {
			if err := onBatch(batch); err != nil {
				return err
			}
			batch = make([]models.ServiceRecord, 0, batchSize)
		}
	}
	if err := rows.Error(); err != nil {
		return fmt.Errorf("ошибка чтения Excel: %v", err)
	}
	if len(batch) > 0 {
		if err := onBatch(batch); err != nil {
			return err
		}
	}

	log.Printf("Успешно распарсено %d записей из Excel файла.\n", totalRecords)
	return nil
}

// normalizeHeader makes matching resilient to case, non-breaking spaces and
// zero-width characters commonly introduced by copied Excel headers.
func normalizeHeader(value string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '\u00a0', '\u2007', '\u202f', '\u200b', '\ufeff':
			return -1
		default:
			return r
		}
	}, strings.ToLower(strings.TrimSpace(value)))
}

func normalizeName(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func isIgnorableName(value string) bool {
	cleaned := normalizeName(value)
	if cleaned == "" {
		return true
	}

	lower := strings.ToLower(cleaned)
	return strings.Contains(lower, "пустое имя") || strings.Contains(lower, "empty name")
}

// parseServiceDate accepts both Excel serial values and the textual date
// representations encountered in source reports.
func parseServiceDate(value string, use1904Dates bool) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}

	if serial, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", "."), 64); err == nil {
		if date, err := excelize.ExcelDateToTime(serial, use1904Dates); err == nil {
			return date
		}
	}

	value = strings.Join(strings.Fields(value), " ")
	formats := []string{
		// Date-time values from Excel with hours and minutes: 07.09.23 17:02
		"02.01.06 15:04", "02.01.06 15:04:05",
		"02.01.2006 15:04", "02.01.2006 15:04:05",
		"2.1.06 15:04", "2.1.2006 15:04",
		"02.01.06", "02.01.2006",
		"2006-01-02 15:04", "2006-01-02 15:04:05", "2006-01-02",
		"02-01-06 15:04", "02-01-06 15:04:05",
		"02-01-2006 15:04", "02-01-2006 15:04:05",
		"02-01-06", "02-01-2006",
		"02/01/06 15:04", "02/01/06 15:04:05",
		"02/01/2006 15:04", "02/01/2006 15:04:05",
		"02/01/06", "02/01/2006",
		"2006-01-02T15:04:05", "2006-01-02T15:04",
		"2006-01-02 15:04:05Z07:00", "2006-01-02T15:04:05Z07:00",
	}

	for _, format := range formats {
		if date, err := time.ParseInLocation(format, value, time.Local); err == nil {
			return date
		}
	}

	// Fallback for strings that include only a date without time: keep the date but
	// set the time to 00:00:00 instead of silently dropping the whole value.
	if parsed, err := time.ParseInLocation("2006-01-02", value, time.Local); err == nil {
		return parsed
	}
	if parsed, err := time.ParseInLocation("02.01.2006", value, time.Local); err == nil {
		return parsed
	}
	if parsed, err := time.ParseInLocation("02.01.06", value, time.Local); err == nil {
		return parsed
	}

	return time.Time{}
}
