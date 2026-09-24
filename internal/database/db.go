package database

import (
	"fmt"
	"log"
	"os"

	// Обязательно проверьте, что путь совпадает с именем вашего модуля
	"github.com/danialmarat/batys-monitor-backend/internal/models"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func ConnectDb() {
	// Загружаем переменные из .env файла
	_ = godotenv.Load()

	// Читаем конфигурацию из переменных окружения
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	dbTimezone := os.Getenv("DB_TIMEZONE")

	// Строка подключения соответствует настройкам в docker-compose.yml
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=%s",
		dbHost, dbUser, dbPassword, dbName, dbPort, dbTimezone,
	)

	log.Printf("Подключаемся к БД: host=%s user=%s dbname=%s port=%s", dbHost, dbUser, dbName, dbPort)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info), // Будет выводить SQL-запросы в консоль
	})

	if err != nil {
		log.Fatal("Ошибка подключения к базе данных: \n", err)
	}

	log.Println("Успешное подключение к PostgreSQL!")

	// Авто-миграция: GORM сам создаст таблицы ServiceRecord и ServiceClassifier, если их нет
	log.Println("Запуск автоматической миграции таблиц...")
	err = db.AutoMigrate(
		&models.ServiceRecord{},
		&models.ServiceClassifier{},
		&models.DetectedRisk{},
		&models.RiskJob{},
	)
	if err != nil {
		log.Fatal("Ошибка миграции: \n", err)
	}

	if err := db.Exec(`ALTER TABLE service_records ALTER COLUMN service_date TYPE TIMESTAMPTZ USING service_date::timestamptz`).Error; err != nil {
		log.Printf("Предупреждение: не удалось привести service_records.service_date к TIMESTAMPTZ: %v", err)
	}
	if err := db.Exec(`ALTER TABLE detected_risks ALTER COLUMN risk_date TYPE TIMESTAMPTZ USING risk_date::timestamptz`).Error; err != nil {
		log.Printf("Предупреждение: не удалось привести detected_risks.risk_date к TIMESTAMPTZ: %v", err)
	}

	// Создаём индексы для оптимизации запросов
	log.Println("Создание индексов для оптимизации...")

	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_service_records_clinic_doctor ON service_records(clinic_name, doctor_name)",
		"CREATE INDEX IF NOT EXISTS idx_service_records_patient_service ON service_records(patient_iin, service_code)",
		"CREATE INDEX IF NOT EXISTS idx_service_records_service_date ON service_records(service_date)",
		"CREATE INDEX IF NOT EXISTS idx_service_records_doctor_date ON service_records(doctor_name, service_date)",
		"CREATE INDEX IF NOT EXISTS idx_detected_risks_indicator ON detected_risks(indicator)",
		"CREATE INDEX IF NOT EXISTS idx_detected_risks_job_id ON detected_risks(job_id)",
		"CREATE INDEX IF NOT EXISTS idx_detected_risks_clinic ON detected_risks(clinic_name)",
		"CREATE INDEX IF NOT EXISTS idx_risk_jobs_status ON risk_jobs(status)",
	}

	for _, idxSQL := range indexes {
		if err := db.Exec(idxSQL).Error; err != nil {
			log.Printf("Предупреждение при создании индекса: %v", err)
		}
	}

	DB = db
}
