package models

import (
	"time"

	"gorm.io/datatypes"
)

type DetectedRisk struct {
	ID         uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	JobID      uint           `gorm:"index;not null;default:0" json:"job_id"`
	Indicator  string         `gorm:"type:varchar(20);index;not null" json:"indicator"`
	ClinicName string         `gorm:"type:varchar(255);index" json:"clinic_name"`
	DoctorName string         `gorm:"type:varchar(255);index" json:"doctor_name"`
	PatientIIN string         `gorm:"type:varchar(20);index" json:"patient_iin"`
	RiskDate   time.Time      `gorm:"type:date;index" json:"risk_date"`
	Amount     float64        `gorm:"type:numeric(18,2);default:0" json:"amount"`
	Details    datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"details"`
	CreatedAt  time.Time      `gorm:"autoCreateTime" json:"created_at"`
}

func (DetectedRisk) TableName() string {
	return "detected_risks"
}
