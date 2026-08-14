package database

import (
	"log"
	// Обязательно проверьте, что путь совпадает с именем вашего модуля
	"github.com/danialmarat/batys-monitor-backend/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func ConnectDb() {
	// Строка подключения соответствует настройкам в docker-compose.yml
	// TimeZone установили на Азию/Уральск
	dsn := "host=localhost user=batys_user password=batys_password dbname=batys_monitor port=5440 sslmode=disable TimeZone=Asia/Oral"

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info), // Будет выводить SQL-запросы в консоль
	})

	if err != nil {
		log.Fatal("Ошибка подключения к базе данных: \n", err)
	}

	log.Println("Успешное подключение к PostgreSQL!")

	// Авто-миграция: GORM сам создаст таблицы ServiceRecord и ServiceClassifier, если их нет
	log.Println("Запуск автоматической миграции таблиц...")
	err = db.AutoMigrate(&models.ServiceRecord{}, &models.ServiceClassifier{}, &models.DetectedRisk{})
	if err != nil {
		log.Fatal("Ошибка миграции: \n", err)
	}

	DB = db
}
