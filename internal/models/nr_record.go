package models

import "time"

// NrRecord хранит одну запись из реестра юридических лиц нерезидентов.
// Источник: Excel «таблица за 2025-2026 года».
type NrRecord struct {
	ID               uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	JobID            uint      `gorm:"index;not null;default:0" json:"job_id"`
	FullName         string    `gorm:"type:varchar(500);index" json:"full_name"`          // Полное наименование
	BIN              string    `gorm:"type:varchar(20);index" json:"bin"`                  // БИН
	RegType          string    `gorm:"type:varchar(100)" json:"reg_type"`                  // Регистрация / Перерегистрация
	RegDate          time.Time `gorm:"type:date;index" json:"reg_date"`                    // Дата регистрации
	ReregDate        time.Time `gorm:"type:date" json:"rereg_date"`                        // Дата перерегистрации
	CONEmployee      string    `gorm:"type:varchar(255)" json:"con_employee"`              // Сотрудник ЦОНа
	Translator       string    `gorm:"type:varchar(255);index" json:"translator"`          // Переводчик
	Notary           string    `gorm:"type:varchar(255);index" json:"notary"`              // Нотариус
	Director         string    `gorm:"type:text" json:"director"`                          // Руководитель (полный текст)
	DirectorName     string    `gorm:"type:varchar(255);index" json:"director_name"`       // ФИО руководителя (извлечённое)
	DirectorIIN      string    `gorm:"type:varchar(20);index" json:"director_iin"`         // ИИН руководителя
	DirectorCountry  string    `gorm:"type:varchar(100)" json:"director_country"`          // Страна руководителя
	Founders         string    `gorm:"type:text" json:"founders"`                          // Учредители (полный текст)
	ActivityType     string    `gorm:"type:varchar(500)" json:"activity_type"`             // Вид деятельности
	AuthorizedCapital float64  `gorm:"type:numeric(18,2);default:0" json:"authorized_capital"` // Уставной капитал
	LegalAddress     string    `gorm:"type:varchar(500);index" json:"legal_address"`       // Юридический адрес
	CreatedAt        time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (NrRecord) TableName() string {
	return "nr_records"
}
