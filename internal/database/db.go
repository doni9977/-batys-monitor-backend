package database

import (
	"fmt"
	"log"
	"os"

	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func ConnectDb() {
	// Загружаем .env файл (если он есть)
	if err := godotenv.Load(); err != nil {
		log.Println("Файл .env не найден, используем переменные окружения системы")
	}

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=%s",
		getEnvOrDefault("DB_HOST", "localhost"),
		getEnvOrDefault("DB_USER", "batys_user"),
		getEnvOrDefault("DB_PASSWORD", "batys_password"),
		getEnvOrDefault("DB_NAME", "batys_monitor"),
		getEnvOrDefault("DB_PORT", "5440"),
		getEnvOrDefault("DB_TIMEZONE", "Asia/Oral"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn), // Warn вместо Info — меньше шума в консоли
	})
	if err != nil {
		log.Fatal("Ошибка подключения к базе данных: \n", err)
	}

	log.Println("✅ Успешное подключение к PostgreSQL!")

	log.Println("Запуск автоматической миграции таблиц...")
	err = db.AutoMigrate(
		&models.ServiceRecord{},
		&models.ServiceClassifier{},
		&models.DetectedRisk{},
		&models.RiskJob{},
		&models.NrRecord{},
		&models.BerkutRecord{},
		&models.InpatientRecord{},
	)
	if err != nil {
		log.Fatal("Ошибка миграции: \n", err)
	}

	// Приводим колонки к TIMESTAMPTZ для корректной работы с временными зонами
	if err := db.Exec(`ALTER TABLE service_records ALTER COLUMN service_date TYPE TIMESTAMPTZ USING service_date::timestamptz`).Error; err != nil {
		log.Printf("Предупреждение (миграция service_date): %v", err)
	}
	if err := db.Exec(`ALTER TABLE detected_risks ALTER COLUMN risk_date TYPE TIMESTAMPTZ USING risk_date::timestamptz`).Error; err != nil {
		log.Printf("Предупреждение (миграция risk_date): %v", err)
	}

	// Задача 13: Индексы PostgreSQL для ускорения SQL-запросов алгоритмов рисков
	createIndexes(db)

	DB = db
}

// createIndexes создаёт индексы для ускорения запросов Risk Engine.
// Использует IF NOT EXISTS — безопасно вызывать при каждом старте.
func createIndexes(db *gorm.DB) {
	indexes := []string{
		// service_records — основные запросы идут по врачу+дата, пациент+код+дата
		`CREATE INDEX IF NOT EXISTS idx_sr_doctor_date     ON service_records(doctor_name, service_date)`,
		`CREATE INDEX IF NOT EXISTS idx_sr_patient_code    ON service_records(patient_iin, service_code)`,
		`CREATE INDEX IF NOT EXISTS idx_sr_code            ON service_records(service_code)`,
		`CREATE INDEX IF NOT EXISTS idx_sr_clinic          ON service_records(clinic_name)`,
		// detected_risks — фильтрация по индикатору и job_id
		`CREATE INDEX IF NOT EXISTS idx_dr_indicator_job   ON detected_risks(indicator, job_id)`,
		`CREATE INDEX IF NOT EXISTS idx_dr_clinic          ON detected_risks(clinic_name)`,
		`CREATE INDEX IF NOT EXISTS idx_dr_job_id          ON detected_risks(job_id)`,
	}

	for _, idx := range indexes {
		if err := db.Exec(idx).Error; err != nil {
			log.Printf("Предупреждение (создание индекса): %v", err)
		}
	}
	log.Println("✅ Индексы PostgreSQL созданы/проверены")
}
