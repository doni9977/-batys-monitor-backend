package handlers

import (
	"fmt"
	"log"
	"os"
	"time"

	"math/rand"

	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/danialmarat/batys-monitor-backend/internal/parser"
	"github.com/danialmarat/batys-monitor-backend/internal/services"
	"github.com/gofiber/fiber/v2"
)

// UploadNonResidentExcel принимает Excel-файл реестра нерезидентов,
// очищает старые NR-данные, загружает новые, генерирует синтетику Беркут
// и запускает NR Risk Engine.
func UploadNonResidentExcel(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Файл не найден. Убедитесь, что поле называется 'file'",
		})
	}

	log.Printf("📥 [NR] Получен файл: %s, Размер: %.2f КБ", file.Filename, float64(file.Size)/1024)

	os.MkdirAll("./tmp", os.ModePerm)
	tempPath := fmt.Sprintf("./tmp/nr_%s", file.Filename)
	if err := c.SaveFile(file, tempPath); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Не удалось сохранить файл",
		})
	}
	defer os.Remove(tempPath)

	// Парсим Excel
	records, err := parser.ParseNrExcel(tempPath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Ошибка парсинга: %v", err),
		})
	}
	if len(records) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Файл не содержит данных или формат не распознан",
		})
	}

	log.Printf("[NR] Распарсено %d записей. Очищаем старые данные...", len(records))

	// Очищаем старые NR-данные
	database.DB.Exec("TRUNCATE TABLE nr_records RESTART IDENTITY CASCADE")
	database.DB.Exec("TRUNCATE TABLE berkut_records RESTART IDENTITY CASCADE")
	// Удаляем только NR-риски (не ОСМС!)
	database.DB.Exec("DELETE FROM detected_risks WHERE indicator LIKE 'NR%'")
	database.DB.Exec("DELETE FROM risk_jobs WHERE domain = 'nr'")

	// Создаём job
	job := &models.RiskJob{
		Status:        models.RiskJobStatusQueued,
		Username:      c.Locals("admin_username").(string),
		Domain:        "nr",
		SourceFile:    file.Filename,
		LoadedRecords: int64(len(records)),
	}
	database.DB.Create(job)

	// Привязываем job_id к записям
	for i := range records {
		records[i].JobID = job.ID
	}

	// Сохраняем NR-записи
	res := database.DB.CreateInBatches(&records, 500)
	if res.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Ошибка сохранения в БД: " + res.Error.Error(),
		})
	}

	log.Printf("[NR] Сохранено %d записей. Генерируем синтетику Беркут...", len(records))

	// Генерируем синтетические данные Беркут (30% рисковые)
	berkutRecords := generateSyntheticBerkut(records)
	if len(berkutRecords) > 0 {
		database.DB.CreateInBatches(&berkutRecords, 500)
		log.Printf("[NR] Сгенерировано %d Беркут-записей", len(berkutRecords))
	}

	// Запускаем NR Risk Engine в фоне
	go services.RunAllNrRiskEngines(job.ID)

	return c.JSON(fiber.Map{
		"status":         "success",
		"message":        fmt.Sprintf("Файл '%s' загружен. Найдено %d компаний нерезидентов.", file.Filename, len(records)),
		"loaded_records": len(records),
		"berkut_records": len(berkutRecords),
		"risk_job_id":    job.ID,
		"domain":         "nr",
	})
}

// Общие ГРНЗ для рисковой группы (транзитный туризм NR2)
var sharedPlates = []string{"M025HB73", "K515HT73", "H946EP73"}

// generateSyntheticBerkut создаёт данные пересечения границы:
// 30% — рисковые (въезд после регистрации, короткий визит, общий ГРНЗ),
// 70% — нормальные (въезд до регистрации, 5-14 дней, уникальный ГРНЗ).
func generateSyntheticBerkut(nrRecords []models.NrRecord) []models.BerkutRecord {
	rng := rand.New(rand.NewSource(42)) // фиксированный seed для воспроизводимости
	crossingPoints := []string{"Сырым", "Шаган", "Астана (аэропорт)", "Уральск (ж/д)"}

	total := len(nrRecords)
	riskCount := int(float64(total) * 0.30)
	riskIndices := make(map[int]bool)

	// Случайно выбираем 30% как рисковые
	indices := rng.Perm(total)
	for i := 0; i < riskCount && i < len(indices); i++ {
		riskIndices[indices[i]] = true
	}

	var berkuts []models.BerkutRecord

	for i, nr := range nrRecords {
		if nr.DirectorIIN == "" || nr.RegDate.IsZero() {
			continue
		}

		isRisk := riskIndices[i]

		var entryDate, exitDate time.Time
		var plate string

		if isRisk {
			// Рисковый сценарий: въехал ПОСЛЕ регистрации (1-7 дней спустя)
			daysAfter := rng.Intn(7) + 1
			entryDate = nr.RegDate.AddDate(0, 0, daysAfter)
			// Пребывание 1-2 дня
			stayDays := rng.Intn(2) + 1
			exitDate = entryDate.AddDate(0, 0, stayDays)
			// Общий ГРНЗ из пула
			plate = sharedPlates[rng.Intn(len(sharedPlates))]
		} else {
			// Нормальный сценарий: въехал ДО регистрации
			daysBefore := rng.Intn(10) + 2
			entryDate = nr.RegDate.AddDate(0, 0, -daysBefore)
			// Нормальное пребывание 5-14 дней
			stayDays := rng.Intn(10) + 5
			exitDate = entryDate.AddDate(0, 0, stayDays)
			// Уникальный ГРНЗ: генерируем псевдослучайный
			letters := "АВЕКМНОРСТУХABEKMHOPCTYX"
			region := rng.Intn(90) + 10
			plate = fmt.Sprintf("%c%03d%c%c%d",
				letters[rng.Intn(len(letters))],
				rng.Intn(900)+100,
				letters[rng.Intn(len(letters))],
				letters[rng.Intn(len(letters))],
				region,
			)
		}

		berkuts = append(berkuts, models.BerkutRecord{
			DirectorIIN:   nr.DirectorIIN,
			DirectorName:  nr.DirectorName,
			PassportNo:    extractPassportFromDirector(nr.Director),
			EntryDate:     entryDate,
			ExitDate:      exitDate,
			VehiclePlate:  plate,
			CrossingPoint: crossingPoints[rng.Intn(len(crossingPoints))],
			IsSynthetic:   true,
		})
	}

	return berkuts
}

func extractPassportFromDirector(raw string) string {
	// Паспорт №: "паспорт № 663805289"
	start := -1
	for i, r := range raw {
		if r == '№' {
			start = i + 2
			break
		}
	}
	if start < 0 || start >= len(raw) {
		return ""
	}
	end := start
	for end < len(raw) && (raw[end] == ' ' || (raw[end] >= '0' && raw[end] <= '9') || raw[end] == '-') {
		end++
	}
	return raw[start:end]
}
