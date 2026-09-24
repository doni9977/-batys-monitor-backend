package handlers

import (
	"fmt"
	"log"
	"os"

	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/danialmarat/batys-monitor-backend/internal/parser"
	"github.com/danialmarat/batys-monitor-backend/internal/services"
	"github.com/gofiber/fiber/v2"
)

// UploadInpatientExcel принимает один или несколько Excel-файлов стационара
// ("XXXX Талап.xlsx", "Юнисерв_XXXX.xlsx"), очищает старые данные стационара,
// загружает все записи в таблицу inpatient_records и (позже) запускает движок.
//
// Файлы отправляются в поле формы "files" (можно несколько за один запрос).
func UploadInpatientExcel(c *fiber.Ctx) error {
	// 1. Достаём все файлы из multipart-формы (поле "files")
	form, err := c.MultipartForm()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Не удалось прочитать форму. Отправьте файлы в поле 'files'",
		})
	}

	files := form.File["files"]
	if len(files) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Файлы не найдены. Поле формы должно называться 'files'",
		})
	}

	log.Printf("📥 Получено файлов стационара: %d", len(files))

	// 2. Чистим ТОЛЬКО таблицу стационара, и только один раз — до цикла.
	//    service_records (поликлиника) НЕ трогаем: она понадобится алгоритмам S1/S5.
	if err := database.DB.Exec("TRUNCATE TABLE inpatient_records RESTART IDENTITY;").Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Не удалось очистить старые данные стационара: " + err.Error(),
		})
	}
	log.Println("🗑️  Старые данные стационара удалены. Начинаем загрузку...")

	os.MkdirAll("./tmp", os.ModePerm)

	// 3. Идём по каждому файлу: сохраняем во временную папку и парсим.
	//    Собираем все записи в один общий срез allRecords.
	var allRecords []models.InpatientRecord
	var fileNames []string

	for _, fileHeader := range files {
		tempPath := fmt.Sprintf("./tmp/%s", fileHeader.Filename)

		if err := c.SaveFile(fileHeader, tempPath); err != nil {
			log.Printf("Не удалось сохранить файл %s: %v", fileHeader.Filename, err)
			continue
		}

		records, err := parser.ParseInpatientExcel(tempPath)
		os.Remove(tempPath) // временный файл больше не нужен

		if err != nil {
			log.Printf("Ошибка парсинга файла %s: %v", fileHeader.Filename, err)
			continue // один битый файл не должен ронять всю загрузку
		}

		log.Printf("✅ %s → распарсено %d записей", fileHeader.Filename, len(records))
		allRecords = append(allRecords, records...)
		fileNames = append(fileNames, fileHeader.Filename)
	}

	if len(allRecords) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Ни одной записи не распарсено. Проверьте, что файлы в формате .xlsx",
		})
	}

	// 4. Записываем все записи в базу пачками по 1000 (быстрее, чем по одной).
	result := database.DB.CreateInBatches(&allRecords, 1000)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Ошибка сохранения в БД: %v", result.Error),
		})
	}
	log.Printf("✅ Сохранено %d записей стационара в БД.", result.RowsAffected)

	// 5. Создаём запись-задачу (RiskJob) с доменом "inpatient", чтобы риски
	//    пометились как стационарные (а не osms по умолчанию).
	job := &models.RiskJob{
		Status:        models.RiskJobStatusQueued,
		Username:      c.Locals("admin_username").(string),
		Domain:        "inpatient",
		SourceFile:    fmt.Sprintf("Стационар: %d файл(ов)", len(fileNames)),
		LoadedRecords: result.RowsAffected,
	}
	if err := database.DB.Create(job).Error; err != nil {
		log.Printf("Не удалось создать RiskJob: %v", err)
	}

	// 6. Запуск движка — ВРЕМЕННО ОТКЛЮЧЁН: services.RunInpatientEngine ещё не написан.
	//    Раскомментируем этот блок (и импорт "services" выше), когда сделаем движок.
	//
	go func() {
		jobID := uint(0)
		if job != nil {
			jobID = job.ID
		}
		services.RunInpatientEngine(jobID)
	}()

	return c.JSON(fiber.Map{
		"status":         "success",
		"message":        fmt.Sprintf("Загружено %d записей из %d файлов.", result.RowsAffected, len(fileNames)),
		"loaded_records": result.RowsAffected,
		"files":          fileNames,
	})
}
