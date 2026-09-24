package database

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	_ "embed"

	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

//go:embed default_classifiers.json
var defaultClassifiersJSON []byte

//go:embed default_bank_responses.json
var defaultBankResponsesJSON []byte

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
		&models.BankResponse{},
		&models.Admin{},
		&models.RevokedToken{},
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
	seedAdmin(db)
	seedClassifiers(db)
	seedBankResponses(db)

	DB = db
}

func seedAdmin(db *gorm.DB) {
	username := getEnvOrDefault("ADMIN_USERNAME", "admin")
	password := getEnvOrDefault("ADMIN_PASSWORD", "afm")

	var admin models.Admin
	if err := db.Where("username = ?", username).First(&admin).Error; err == nil {
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Предупреждение (создание администратора): %v", err)
		return
	}
	if err := db.Create(&models.Admin{Username: username, PasswordHash: string(hash)}).Error; err != nil {
		log.Printf("Предупреждение (сохранение администратора): %v", err)
		return
	}
	log.Printf("✅ Администратор %q создан", username)
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

func seedClassifiers(db *gorm.DB) {
	var count int64
	db.Model(&models.ServiceClassifier{}).Count(&count)
	if count > 0 {
		return
	}

	var classifiers []models.ServiceClassifier
	if err := json.Unmarshal(defaultClassifiersJSON, &classifiers); err != nil {
		log.Printf("Ошибка при чтении дефолтного классификатора: %v", err)
		return
	}

	result := db.CreateInBatches(&classifiers, 500)
	if result.Error != nil {
		log.Printf("Ошибка при сохранении классификаторов по умолчанию: %v", result.Error)
		return
	}

	log.Printf("✅ Загружено %d записей классификатора по умолчанию", result.RowsAffected)
}

func seedBankResponses(db *gorm.DB) {
	var count int64
	db.Model(&models.BankResponse{}).Count(&count)
	if count > 0 {
		return
	}

	var banks []models.BankResponse
	if err := json.Unmarshal(defaultBankResponsesJSON, &banks); err != nil {
		log.Printf("Ошибка при чтении дефолтных банковских ответов: %v", err)
		return
	}

	for i := range banks {
		banks[i].ID = 0
	}

	result := db.CreateInBatches(&banks, 500)
	if result.Error != nil {
		log.Printf("Ошибка при сохранении банковских ответов по умолчанию: %v", result.Error)
		return
	}

	log.Printf("✅ Загружено %d банковских ответов по умолчанию", result.RowsAffected)
}
