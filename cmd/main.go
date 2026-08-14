package main

import (
	"log"

	"github.com/danialmarat/batys-monitor-backend/internal/database"
	"github.com/danialmarat/batys-monitor-backend/internal/handlers" // <--- Добавили импорт
	"github.com/gofiber/fiber/v2"
)

func main() {
	database.ConnectDb()

	app := fiber.New()

	// Увеличим лимит размера файла (по умолчанию Fiber принимает только 4MB)
	// Ставим лимит в 100 Мегабайт, так как файлы ДЭР очень тяжелые
	app.Server().MaxRequestBodySize = 100 * 1024 * 1024

	app.Get("/api/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "success", "message": "Система работает!"})
	})

	// ДОБАВЛЯЕМ НАШ НОВЫЙ РОУТ ДЛЯ ЗАГРУЗКИ ФАЙЛА
	app.Post("/api/upload", handlers.UploadExcel)
	app.Get("/api/risks/a3", handlers.GetRiskA3)
	app.Get("/api/risks/a1", handlers.GetRiskA1)
	app.Get("/api/risks/a2", handlers.GetRiskA2)
	app.Get("/api/risks/a7", handlers.GetRiskA7)
	app.Get("/api/risks/a8", handlers.GetRiskA8)
	app.Get("/api/risks/a10", handlers.GetRiskA10)
	app.Get("/api/risks/duplicates", handlers.GetRiskDuplicates)
	app.Post("/api/upload-classifier", handlers.UploadClassifierExcel)

	log.Println("Запуск сервера на порту 3000...")
	log.Fatal(app.Listen(":3000"))
}
