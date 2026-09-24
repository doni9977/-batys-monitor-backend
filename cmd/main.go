package main

import (
	"log"
	"os"

	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/handlers" // <--- Добавили импорт
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/joho/godotenv"
)

func main() {
	// Загружаем переменные из .env
	_ = godotenv.Load()

	database.ConnectDb()

	app := fiber.New()

	// Увеличим лимит размера файла (по умолчанию Fiber принимает только 4MB)
	// Ставим лимит в 100 Мегабайт, так как файлы ДЭР очень тяжелые
	app.Server().MaxRequestBodySize = 100 * 1024 * 1024

	// CORS middleware - должен быть ДО регистрации роутов
	corsOrigins := os.Getenv("CORS_ORIGINS")
	if corsOrigins == "" {
		corsOrigins = "http://localhost:3001,http://localhost:3002,http://localhost:5173"
	}

	app.Use(cors.New(cors.Config{
		AllowOrigins: corsOrigins,
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))

	app.Get("/api/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "success", "message": "Система работает!"})
	})

	// ДОБАВЛЯЕМ НАШ НОВЫЙ РОУТ ДЛЯ ЗАГРУЗКИ ФАЙЛА
	app.Post("/api/upload", handlers.UploadExcel)
	app.Get("/api/risks/a3", handlers.GetRiskA3)
	app.Get("/api/risks/a1", handlers.GetRiskA1)
	app.Get("/api/risks/a2", handlers.GetRiskA2)
	app.Get("/api/risks/a4", handlers.GetRiskDuplicates)
	app.Get("/api/risks/a7", handlers.GetRiskA7)
	app.Get("/api/risks/a8", handlers.GetRiskA8)
	app.Get("/api/risks/a10", handlers.GetRiskA10)
	app.Get("/api/risks/duplicates", handlers.GetRiskDuplicates)
	app.Post("/api/upload-classifier", handlers.UploadClassifierExcel)
	app.Get("/api/risk-jobs/latest", handlers.GetLatestRiskJob)
	app.Get("/api/risk-jobs/:id", handlers.GetRiskJobByID)
	app.Get("/api/summary", handlers.GetRiskSummary)
	app.Get("/api/clinics/risks", handlers.GetClinicsRisks)
	app.Post("/api/export/xlsx", handlers.ExportRisksToXLSX)

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("Запуск сервера на порту %s...", port)
	log.Fatal(app.Listen(":" + port))
}
