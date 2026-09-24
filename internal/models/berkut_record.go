package models

import "time"

// BerkutRecord хранит данные о пересечении государственной границы РК
// из системы пограничного контроля ПС КНБ «Беркут».
// 30% записей синтетически сгенерированы как рисковые (въезд после регистрации / короткий визит).
type BerkutRecord struct {
	ID            uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	DirectorIIN   string    `gorm:"type:varchar(20);index;not null" json:"director_iin"` // ИИН нерезидента
	DirectorName  string    `gorm:"type:varchar(255)" json:"director_name"`
	PassportNo    string    `gorm:"type:varchar(50)" json:"passport_no"` // Номер паспорта
	EntryDate     time.Time `gorm:"type:date;index" json:"entry_date"`   // Дата въезда
	ExitDate      time.Time `gorm:"type:date;index" json:"exit_date"`    // Дата выезда
	VehiclePlate  string    `gorm:"type:varchar(20);index" json:"vehicle_plate"` // ГРНЗ (номер авто)
	CrossingPoint string    `gorm:"type:varchar(100)" json:"crossing_point"`     // КПП (Сырым, Шаган, Астана)
	IsSynthetic   bool      `gorm:"not null;default:true" json:"is_synthetic"`   // Синтетическая запись?
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (BerkutRecord) TableName() string {
	return "berkut_records"
}
