package parser

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/xuri/excelize/v2"
)

func ParseExcel(filePath string) ([]models.ServiceRecord, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия файла: %v", err)
	}
	defer f.Close()

	sheetName := "TDSheet"
	rows, err := f.Rows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения строк листа: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, fmt.Errorf("файл пуст или не содержит данных")
	}

	// RawCellValue сохраняет серийный номер Excel для ячеек дат. Это позволяет
	// корректно обработать даты независимо от формата отображения в книге.
	headerRow, err := rows.Columns(excelize.Options{RawCellValue: true})
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения заголовка: %v", err)
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
				// Специальная проверка для коротких слов вроде "пол", чтобы не находить "дополнительная"
				if kwLower == "пол" {
					if colName == "пол" || strings.HasPrefix(colName, "пол ") || strings.HasSuffix(colName, " пол") {
						return idx
					}
				} else if strings.Contains(colName, kwLower) {
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
	patientNameIdx := findColIndex("пациенты", "пациент", "фиофизлица", "фио")
	genderIdx := findColIndex("пол")
	dobIdx := findColIndex("рождения", "дата рожд")
	codeIdx := findColIndex("код услуги", "код")
	serviceNameIdx := findColIndex("наименование услуги", "услуга")
	qtyIdx := findColIndex("количеств")
	amountIdx := findColIndex("сумм")
	diagIdx := findColIndex("мкб", "диагноз")
	clinicIdx := findColIndex("клиника", "клиник", "поставщик", "организация")

	var records []models.ServiceRecord

	workbookProps, err := f.GetWorkbookProps()
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения свойств книги: %v", err)
	}
	use1904Dates := workbookProps.Date1904 != nil && *workbookProps.Date1904

	for rows.Next() {
		row, err := rows.Columns(excelize.Options{RawCellValue: true})
		if err != nil {
			return nil, fmt.Errorf("ошибка чтения строки Excel: %v", err)
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

		patientIIN := getValByIndex(iinIdx)
		patientName := normalizeName(getValByIndex(patientNameIdx))
		patientGender := getValByIndex(genderIdx)

		// Если колонки «Пол» нет в файле или значение пустое — определяем пол автоматически
		if patientGender == "" || genderIdx == -1 {
			patientGender = inferGender(patientIIN, patientName)
		}

		record := models.ServiceRecord{
			ClinicName:     normalizeName(getValByIndex(clinicIdx)),
			DoctorName:     doctorName,
			PatientIIN:     patientIIN,
			PatientName:    patientName,
			PatientGender:  patientGender,
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

		records = append(records, record)
	}
	if err := rows.Error(); err != nil {
		return nil, fmt.Errorf("ошибка чтения Excel: %v", err)
	}

	log.Printf("Успешно распарсено %d записей из Excel файла.\n", len(records))
	return records, nil
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

// inferGender определяет пол пациента, когда колонка «Пол» отсутствует в файле.
//
// Стратегия 1 — ИИН (приоритетная, точность ~100%):
//
//	В казахстанском ИИН (12 цифр) 7-я цифра кодирует век рождения и пол:
//	  нечётная (1, 3, 5) → мужчина
//	  чётная   (2, 4, 6) → женщина
//
// Стратегия 2 — Лингвистический анализ ФИО (fallback, точность ~95%):
//
//	Анализируем окончания отчеств и фамилий:
//	  женские: -овна, -евна, -қызы, -кызы, -гызы, -ова, -ева, -ина, -ская, -цкая
//	  мужские: -ович, -евич, -ұлы, -улы, -ов, -ев, -ин, -ский, -цкий, -ий
func inferGender(iin, fullName string) string {
	// ── Стратегия 1: по ИИН ──────────────────────────────────────────
	cleanIIN := strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return r
		}
		return -1
	}, iin)

	if len(cleanIIN) == 12 {
		seventh := cleanIIN[6] // 7-я цифра (0-indexed = 6)
		switch seventh {
		case '1', '3', '5':
			return "Мужчина"
		case '2', '4', '6':
			return "Женщина"
		}
	}

	// ── Стратегия 2: по ФИО ──────────────────────────────────────────
	if fullName == "" {
		return ""
	}

	lower := strings.ToLower(strings.TrimSpace(fullName))
	// Разбиваем на слова (обычно: фамилия имя отчество)
	words := strings.Fields(lower)

	// Женские суффиксы (проверяем каждое слово)
	femaleSuffixes := []string{
		"овна", "евна", "ёвна", // русские отчества
		"қызы", "кызы", "гызы", "кизи", // казахские отчества
		"ова", "ева", "ёва", // фамилии
		"ина", "ына",
		"ская", "цкая",
		"ая",
	}
	// Мужские суффиксы
	maleSuffixes := []string{
		"ович", "евич", "ёвич", // русские отчества
		"ұлы", "улы", "у|лы", "углы", // казахские отчества
		"ов", "ев", "ёв", // фамилии
		"ин", "ын",
		"ский", "цкий",
	}

	// Проверяем каждое слово, начиная с последнего (отчество обычно в конце)
	for i := len(words) - 1; i >= 0; i-- {
		w := words[i]
		// Сначала проверяем женские (длинные суффиксы важнее — «овна» приоритетнее «на»)
		for _, s := range femaleSuffixes {
			if strings.HasSuffix(w, s) && len([]rune(w)) > len([]rune(s))+1 {
				return "Женщина"
			}
		}
		for _, s := range maleSuffixes {
			if strings.HasSuffix(w, s) && len([]rune(w)) > len([]rune(s))+1 {
				return "Мужчина"
			}
		}
	}

	return ""
}
