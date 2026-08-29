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

// ParseInpatientExcel читает Excel-файл "Пролеченные случаи стационара"
// (файлы XXXX Талап) и возвращает срез InpatientRecord.
//
// Отличия этих файлов от поликлиники, которые здесь учтены:
//   - названия колонок лежат НЕ в первой строке, а ниже (после шапки отчёта:
//     "Список пролеченных...", период, регион, время выдачи);
//   - между строкой заголовков и данными есть 2 служебные строки
//     (подзаголовки и нумерация колонок "1.0, 2.0..."), их надо пропустить;
//   - числовые поля (койко-дни, флаги) приходят как "3.0", "1.0" — приводим float -> int.
//
// ВНИМАНИЕ: excelize умеет только .xlsx. Исходники в формате .xls нужно
// заранее пересохранить в .xlsx (Excel/LibreOffice: "Сохранить как").
func ParseInpatientExcel(filePath string) ([]models.InpatientRecord, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия файла стационара: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("файл не содержит листов")
	}
	sheetName := sheets[0]

	// RawCellValue=true сохраняет серийные номера дат Excel — так их надёжнее
	// парсить независимо от формата отображения в книге (как в ParseExcel).
	rows, err := f.GetRows(sheetName, excelize.Options{RawCellValue: true})
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения листа '%s': %w", sheetName, err)
	}

	// Флаг системы дат 1904 — нужен для корректного перевода серийных дат.
	use1904Dates := false
	if props, err := f.GetWorkbookProps(); err == nil {
		use1904Dates = props.Date1904 != nil && *props.Date1904
	}

	// --- Шаг 1: находим строку заголовков (она не первая!) ---
	// Ищем строку, где одновременно есть "Ф.И.О." и "поступления".
	headerIdx := -1
	for i := 0; i < len(rows) && i < 20; i++ {
		joined := normalizeHeader(strings.Join(rows[i], "|"))
		if strings.Contains(joined, "ф.и.о") && strings.Contains(joined, "поступления") {
			headerIdx = i
			break
		}
	}
	if headerIdx == -1 {
		return nil, fmt.Errorf("не найдена строка заголовков (ожидались 'Ф.И.О.' и 'Дата поступления')")
	}

	// --- Шаг 2: нормализуем заголовки и ищем индексы нужных колонок ---
	header := rows[headerIdx]
	columnNames := make([]string, len(header))
	for i, name := range header {
		columnNames[i] = normalizeHeader(name)
	}

	// Первое совпадение по ключевому слову. Порядок keyword-ов = приоритет.
	findColIndex := func(keywords ...string) int {
		for _, kw := range keywords {
			kw = normalizeHeader(kw)
			for idx, col := range columnNames {
				if strings.Contains(col, kw) {
					return idx
				}
			}
		}
		return -1
	}

	nameIdx := findColIndex("ф.и.о")
	dobIdx := findColIndex("рождения")
	rpnIdx := findColIndex("rpn")
	admDateIdx := findColIndex("дата поступления")
	admTimeIdx := findColIndex("время поступления")
	disDateIdx := findColIndex("дата выписки")
	disTimeIdx := findColIndex("время выписки")
	bedDaysIdx := findColIndex("койко") // первое совпадение = "Проведено койко-дней"
	outcomeIdx := findColIndex("исход пребывания")
	icdIdx := findColIndex("код мкб")
	diagIdx := findColIndex("диагноз")
	plannedIdx := findColIndex("планово")
	emergencyIdx := findColIndex("экстренно")
	amountIdx := findColIndex("предъявленная сумма", "сумм")
	hospitalIdx := findColIndex("стационара выписки", "стационар")
	deptIdx := findColIndex("отделение")
	doctorIdx := findColIndex("врач")
	careTypeIdx := findColIndex("вид медицинской помощи", "вид мед")
	caseIdx := findColIndex("пролеченного случая", "пролеченного")

	// --- Шаг 3: читаем данные (со строки после заголовка) ---
	var records []models.InpatientRecord

	for i := headerIdx + 1; i < len(rows); i++ {
		row := rows[i]

		getVal := func(idx int) string {
			if idx != -1 && idx < len(row) {
				return strings.TrimSpace(row[idx])
			}
			return ""
		}

		name := normalizeName(getVal(nameIdx))
		// Пропуск 2 служебных строк после заголовка и любых пустых:
		//   - у строки подзаголовков ФИО пустое;
		//   - у строки нумерации колонок ФИО = "3.0" (чистое число).
		if name == "" || isNumericLike(name) {
			continue
		}

		rec := models.InpatientRecord{
			PatientName:   name,
			PatientDOB:    getVal(dobIdx),
			RpnID:         getVal(rpnIdx),
			AdmissionDate: parseServiceDate(getVal(admDateIdx), use1904Dates),
			AdmissionTime: getVal(admTimeIdx),
			DischargeDate: parseServiceDate(getVal(disDateIdx), use1904Dates),
			DischargeTime: getVal(disTimeIdx),
			BedDays:       toIntFromFloat(getVal(bedDaysIdx)),
			Outcome:       getVal(outcomeIdx),
			ICD10Code:     getVal(icdIdx),
			Diagnosis:     getVal(diagIdx),
			IsPlanned:     toIntFromFloat(getVal(plannedIdx)),
			IsEmergency:   toIntFromFloat(getVal(emergencyIdx)),
			Amount:        toFloat(getVal(amountIdx)),
			HospitalName:  normalizeName(getVal(hospitalIdx)),
			Department:    normalizeName(getVal(deptIdx)),
			DoctorName:    normalizeName(getVal(doctorIdx)),
			CareType:      getVal(careTypeIdx),
			CaseID:        getVal(caseIdx),
			CreatedAt:     time.Now(),
		}

		// Минимальная валидация: без даты поступления запись бесполезна для алгоритмов.
		if rec.AdmissionDate.IsZero() {
			continue
		}

		records = append(records, rec)
	}

	log.Printf("[Inpatient Parser] Распарсено %d записей из %s (лист '%s')",
		len(records), filePath, sheetName)
	return records, nil
}

// toIntFromFloat парсит значения вида "3.0", "1.0" в int.
// strconv.Atoi такое не умеет (падает на точке), поэтому идём через float.
func toIntFromFloat(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if fv, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64); err == nil {
		return int(fv)
	}
	n, _ := strconv.Atoi(s)
	return n
}

// toFloat парсит сумму ("191723.14"), допускает запятую как разделитель.
func toFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
	return v
}

// isNumericLike сообщает, что строка целиком является числом.
// Используется, чтобы отсеять служебную строку нумерации колонок, где ФИО = "3.0".
func isNumericLike(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	_, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
	return err == nil
}
