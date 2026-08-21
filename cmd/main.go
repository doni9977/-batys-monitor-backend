package main

import (
	"log"
	"os"

	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/handlers"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"
)

func main() {
	// Загружаем .env
	if err := godotenv.Load(); err != nil {
		log.Println("Файл .env не найден, используем переменные окружения системы")
	}

	database.ConnectDb()

	app := fiber.New(fiber.Config{
		// Лимит 200 МБ — файлы ДЭР могут быть очень большими
		BodyLimit: 200 * 1024 * 1024,
	})

	// Задача 2: CORS middleware — позволяет фронтенду обращаться к API
	corsOrigins := os.Getenv("CORS_ORIGINS")
	if corsOrigins == "" {
		corsOrigins = "*"
	}
	app.Use(cors.New(cors.Config{
		AllowOrigins: corsOrigins,
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))

	// HTTP-логгер запросов
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${latency} ${method} ${path}\n",
	}))

	// Health check
	app.Get("/api/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "success",
			"message": "BatysMonitor работает!",
			"version": "2.0.0",
		})
	})

	// Загрузка данных
	app.Post("/api/upload", handlers.UploadExcel)
	app.Post("/api/upload-classifier", handlers.UploadClassifierExcel)
	app.Post("/api/upload-inpatient", handlers.UploadInpatientExcel)

	// Мониторинг заданий
	app.Get("/api/risk-jobs/latest", handlers.GetLatestRiskJob)
	app.Get("/api/risk-jobs/:id", handlers.GetRiskJobByID)

	// Сводка по всем рискам (новый endpoint — Задача 10)
	app.Get("/api/summary", handlers.GetRiskSummary)

	// Агрегация по клиникам для карты (новый endpoint — Задача 11)
	app.Get("/api/clinics/risks", handlers.GetClinicRisks)

	// Экспорт в Excel (новый endpoint — Задача 12)
	app.Get("/api/export/xlsx", handlers.ExportRisksXlsx)

	// Реестр субъектов (клиники + суммы рисков)
	app.Get("/api/registry", handlers.GetRegistry)

	// Аналитика и тренды (реальные агрегированные данные)
	app.Get("/api/analytics", handlers.GetAnalytics)

	// Риски по индикаторам
	app.Get("/api/risks/a1", handlers.GetRiskA1)
	app.Get("/api/risks/a2", handlers.GetRiskA2)
	app.Get("/api/risks/a3", handlers.GetRiskA3)
	app.Get("/api/risks/a4", handlers.GetRiskDuplicates)
	app.Get("/api/risks/a7", handlers.GetRiskA7)
	app.Get("/api/risks/a8", handlers.GetRiskA8)
	app.Get("/api/risks/a10", handlers.GetRiskA10)

	// Риски стационара
	app.Get("/api/risks/s1", handlers.GetRiskS1)
	app.Get("/api/risks/s2", handlers.GetRiskS2)
	app.Get("/api/risks/s3", handlers.GetRiskS3)
	app.Get("/api/risks/s4", handlers.GetRiskS4)
	app.Get("/api/risks/s5", handlers.GetRiskS5)

	// Deprecated — оставляем для совместимости с фронтендом
	app.Get("/api/risks/duplicates", handlers.GetRiskDuplicates)

	// ── Домен: Нерезиденты (КГД) ──────────────────────────────────────────
	// Загрузка реестра нерезидентов
	app.Post("/api/upload-nr", handlers.UploadNonResidentExcel)
	

	// Риски нерезидентов по индикаторам
	app.Get("/api/risks/nr1", handlers.GetNrRisks("NR1"))
	app.Get("/api/risks/nr2", handlers.GetNrRisks("NR2"))
	app.Get("/api/risks/nr3", handlers.GetNrRisks("NR3"))
	app.Get("/api/risks/nr4", handlers.GetNrRisks("NR4"))

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("🚀 Запуск BatysMonitor на порту %s...", port)
	log.Fatal(app.Listen(":" + port))
}
