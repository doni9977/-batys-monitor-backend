package models

import (
	"time"
)

// ServiceRecord соответствует таблице "Реестр услуг" из методологии ДЭР
// Эта структура будет маппиться на колонки вкладки TDSheet из вашего Excel
type ServiceRecord struct {
	ID             uint      `gorm:"primaryKey"`
	ClinicName     string    `gorm:"index"` // "Поставщик" из Excel
	DoctorName     string    `gorm:"index"` // "Врачи"
	PatientIIN     string    `gorm:"index"` // "ИИНы" пациента
	PatientGender  string    // "Пол" пациента
	PatientDOB     string    // "Дата рождения" (оставляем строкой для простоты парсинга)
	ServiceCode    string    `gorm:"index"` // "Код услуги"
	ServiceName    string    // "Услуга"
	ServiceDate    time.Time `gorm:"index"` // "Дата услуги" и "Период услуги"
	Quantity       int       // "Количество"
	Amount         float64   // "Сумма" (для проверки A8 Upcoding)
	DiagnosisMKB10 string    // "Код диагноза МКБ10"
	CreatedAt      time.Time // Системное время загрузки записи
}
