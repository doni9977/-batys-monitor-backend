package models

import (
	"time"
)

// InpatientRecord соответствует таблице "Пролеченные случаи стационара"
// (файлы XXXX Талап.xls). Одна строка Excel = одна госпитализация.
//
// ВАЖНО по парсингу: названия колонок в файле лежат в 7-й строке (счёт с 1),
// а реальные данные начинаются с 10-й строки — между ними 2 мусорные строки
// (подзаголовки и нумерация "1.0, 2.0..."), их нужно пропустить.
//
// ВАЖНО по ключу пациента: ИИН в этих файлах НЕТ. Уникальный пациент = RpnID
// (если заполнен), иначе связка PatientName + PatientDOB.
type InpatientRecord struct {
	ID uint `gorm:"primaryKey"`

	// --- Пациент ---
	PatientName string `gorm:"index"` // "Ф.И.О." (в данных обезличено до инициалов, напр. "А.К.С")
	PatientDOB  string // "Дата рождения" (храним строкой, парсим в SQL — как в ОСМС)
	RpnID       string `gorm:"index"` // "RPN ID" — id из регистра населения (бывает "0"/пусто)

	// --- Даты и время госпитализации ---
	AdmissionDate time.Time `gorm:"type:timestamptz;index"` // "Дата поступления"
	AdmissionTime string    // "Время поступления" (для алгоритмов не нужно, держим как есть)
	DischargeDate time.Time `gorm:"type:timestamptz;index"` // "Дата выписки"
	DischargeTime string    // "Время выписки"

	// --- Клиника случая ---
	BedDays   int    // "Проведено койко-дней" (в файле как 3.0 → парсим float→int)
	Outcome   string `gorm:"index"` // "Исход пребывания": Выписан / Переведен / Умер
	ICD10Code string `gorm:"index"` // "Код МКБ-10" (нужен для S2 — дробление по диагнозу)
	Diagnosis string // "Диагноз заключительный"

	// --- Флаги поступления (в файле 1.0 / 0.0 → 1 / 0) ---
	IsPlanned   int // "Планово"
	IsEmergency int // "Экстренно" (нужен для S4)

	// --- Финансы ---
	Amount float64 // "Предъявленная сумма к оплате" (сумма риска в S2/S3/S5)

	// --- Кто и где оказал ---
	HospitalName string `gorm:"index"` // "Наименование СТАЦИОНАРА выписки"
	Department   string `gorm:"index"` // "Отделение выписки" (нужен для S4)
	DoctorName   string `gorm:"index"` // "Врач" (нужен для S4)
	CareType     string `gorm:"index"` // "Вид медицинской помощи" (для S3: содержит "Круглосуточный")

	// --- Идентификатор случая ---
	CaseID string `gorm:"index"` // "ID пролеченного случая"

	CreatedAt time.Time // системное время загрузки записи
}
