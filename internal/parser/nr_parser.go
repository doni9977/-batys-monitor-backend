package parser

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/xuri/excelize/v2"
)

// Колонки таблицы «нерезиденты 2025-2026»
// №, Полное наименование, БИН, Регистрация, Дата рег., В ЦОНе,
// Переводчик, Нотариус, Дата перерег., Руководитель, Беркут(рук),
// Учредители, Беркут(уч), Вид деятельности, Уставной капитал, Юр. адрес

var reFIO = regexp.MustCompile(`^([А-ЯЁа-яёA-Za-z\s\-]+),\s*[А-ЯЁа-яёA-Za-z]+,\s*паспорт`)
var reIIN = regexp.MustCompile(`ИИН\s+(\d{12})`)

func extractDirectorName(raw string) string {
	raw = strings.TrimSpace(raw)
	// Strip prefix like "Руководитель, назначенный..., "
	if idx := strings.Index(raw, "лица, "); idx != -1 {
		raw = raw[idx+6:]
	}
	// Take up to first comma after name
	parts := strings.SplitN(raw, ",", 2)
	name := strings.TrimSpace(parts[0])
	// Clean non-letter chars at edges
	name = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsSpace(r) || r == '-' {
			return r
		}
		return -1
	}, name)
	return strings.TrimSpace(name)
}

func extractIIN(raw string) string {
	m := reIIN.FindStringSubmatch(raw)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

func extractCountry(raw string) string {
	// Pattern: "..., Россия, паспорт ..."
	countries := []string{"Россия", "Казахстан", "Узбекистан", "Таджикистан", "Туркменистан",
		"Кыргызстан", "Беларусь", "Украина", "Азербайджан", "Армения", "Молдова", "Китай"}
	for _, c := range countries {
		if strings.Contains(raw, c) {
			return c
		}
	}
	return "Неизвестно"
}

func parseFlexibleDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	// Попробуем парсить как Excel serial number (например, "45300" или "45300.00")
	if serial, err := strconv.ParseFloat(s, 64); err == nil && serial > 10000 && serial < 100000 {
		return time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).AddDate(0, 0, int(serial))
	}
	
	formats := []string{
		"2006-01-02",
		"02.01.2006",
		"02-01-2006",
		"02-01-06",
		"02/01/2006",
		"02/01/06",
		"01-02-06",
		"01/02/06", // US short format sometimes output by Excelize
		"1/2/06",
		"2.1.2006",
		"02.01.06",
	}
	for _, f := range formats {
		if d, e := time.ParseInLocation(f, s, time.Local); e == nil {
			return d
		}
	}
	return time.Time{}
}

func parseCapital(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	}
	return 0
}

// ParseNrExcel читает Excel-файл реестра нерезидентов и возвращает срез NrRecord.
// Поддерживает формат таблицы «2025-2026 года».
func ParseNrExcel(filePath string) ([]models.NrRecord, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть файл: %w", err)
	}
	defer f.Close()

	// Берём первый лист
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("файл не содержит листов")
	}
	sheetName := sheets[0]

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения листа: %w", err)
	}

	if len(rows) < 2 {
		return nil, fmt.Errorf("файл пустой или содержит только заголовок")
	}

	// Определяем индексы колонок по заголовку (первая строка)
	headers := rows[0]
	colIdx := make(map[string]int)
	for i, h := range headers {
		key := normalizeNrHeader(h)
		colIdx[key] = i
	}

	getCol := func(row []string, key string) string {
		idx, ok := colIdx[key]
		if !ok || idx >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[idx])
	}

	var records []models.NrRecord

	for rowNum, row := range rows[1:] {
		if len(row) < 3 {
			continue
		}

		bin := strings.TrimSpace(getCol(row, "бин"))
		if bin == "" {
			// Попробовать взять числовое значение из колонки 2 (индекс 2)
			if len(row) > 2 {
				bin = strings.TrimSpace(row[2])
			}
		}
		if bin == "" {
			continue
		}

		fullName := getCol(row, "полное наименование")
		if fullName == "" && len(row) > 1 {
			fullName = strings.TrimSpace(row[1])
		}

		// Читаем даты через excelize cell value (для date-типов)
		regDateRaw := getCol(row, "дата регист-рации")
		if regDateRaw == "" {
			regDateRaw = getCol(row, "дата регистрации")
		}
		reregDateRaw := getCol(row, "дата перерегистрации")

		directorRaw := getCol(row, "руководитель")
		foundersRaw := getCol(row, "учредители")
		translatorRaw := getCol(row, "переводчик")
		notaryRaw := getCol(row, "нотариус")
		conRaw := getCol(row, "в цоне")
		activityRaw := getCol(row, "вид деятельности")
		addressRaw := getCol(row, "юридический адрес")
		regType := getCol(row, "регистрация-перерегистрация")

		capitalRaw := getCol(row, "уставной капитал, тенге")
		capital := 0.0
		if capitalRaw != "" {
			// Parse number
			s := strings.ReplaceAll(capitalRaw, " ", "")
			s = strings.ReplaceAll(s, ",", ".")
			var v float64
			fmt.Sscanf(s, "%f", &v)
			capital = v
		}

		directorName := extractDirectorName(directorRaw)
		directorIIN := extractIIN(directorRaw)
		directorCountry := extractCountry(directorRaw)

		// Parse dates from string (excelize возвращает их как строки)
		regDate := parseFlexibleDate(regDateRaw)
		reregDate := parseFlexibleDate(reregDateRaw)

		rec := models.NrRecord{
			FullName:          fullName,
			BIN:               bin,
			RegType:           regType,
			RegDate:           regDate,
			ReregDate:         reregDate,
			CONEmployee:       conRaw,
			Translator:        translatorRaw,
			Notary:            notaryRaw,
			Director:          directorRaw,
			DirectorName:      directorName,
			DirectorIIN:       directorIIN,
			DirectorCountry:   directorCountry,
			Founders:          foundersRaw,
			ActivityType:      activityRaw,
			AuthorizedCapital: capital,
			LegalAddress:      addressRaw,
		}

		records = append(records, rec)
		_ = rowNum
	}

	log.Printf("[NR Parser] Распарсено %d записей из файла %s", len(records), filePath)
	return records, nil
}

func normalizeNrHeader(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	// Remove non-breaking and zero-width spaces
	s = strings.Map(func(r rune) rune {
		switch r {
		case '\u00a0', '\u200b', '\ufeff', '\u2007', '\u202f':
			return -1
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

// ParseBerkutExcel читает файл Беркут и возвращает срез BerkutRecord.
// Ожидает колонки: ИИН, ФИО, Паспорт, Дата въезда, Дата выезда, ГРНЗ, КПП
func ParseBerkutExcel(filePath string) ([]models.BerkutRecord, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть Беркут-файл: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("файл Беркут пустой")
	}

	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения Беркут: %w", err)
	}

	if len(rows) < 2 {
		return nil, fmt.Errorf("Беркут-файл пуст")
	}

	headers := rows[0]
	colIdx := make(map[string]int)
	for i, h := range headers {
		colIdx[strings.ToLower(strings.TrimSpace(h))] = i
	}

	getCol := func(row []string, key string) string {
		idx, ok := colIdx[key]
		if !ok || idx >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[idx])
	}

	var records []models.BerkutRecord
	for _, row := range rows[1:] {
		iin := getCol(row, "иин")
		if iin == "" {
			continue
		}
		parseDate := func(s string) time.Time {
			s = strings.TrimSpace(s)
			for _, f := range []string{"02.01.2006", "2006-01-02"} {
				if d, e := time.ParseInLocation(f, s, time.Local); e == nil {
					return d
				}
			}
			return time.Time{}
		}

		records = append(records, models.BerkutRecord{
			DirectorIIN:   iin,
			DirectorName:  getCol(row, "фио"),
			PassportNo:    getCol(row, "паспорт"),
			EntryDate:     parseDate(getCol(row, "дата въезда")),
			ExitDate:      parseDate(getCol(row, "дата выезда")),
			VehiclePlate:  getCol(row, "грнз"),
			CrossingPoint: getCol(row, "кпп"),
			IsSynthetic:   false,
		})
	}

	log.Printf("[Berkut Parser] Распарсено %d записей из %s", len(records), filePath)
	return records, nil
}
