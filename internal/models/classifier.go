package models

import "time"

type ServiceClassifier struct {
	Code              string    `gorm:"primaryKey;column:code" json:"code"`
	Name              string    `gorm:"column:name" json:"name"`
	Tariff            float64   `gorm:"column:tariff" json:"tariff"`
	MinAge            int       `gorm:"column:min_age" json:"min_age"`
	MaxAge            int       `gorm:"column:max_age" json:"max_age"`
	GenderRestriction string    `gorm:"column:gender_restriction" json:"gender_restriction"`
	NormMinutes       int       `gorm:"column:norm_minutes" json:"norm_minutes"`
	MaxPerDay         int       `gorm:"column:max_per_day" json:"max_per_day"`
	MaxPerYear        int       `gorm:"column:max_per_year" json:"max_per_year"`
	IsComplex         string    `gorm:"column:is_complex" json:"is_complex"`
	RuleStatus        string    `gorm:"column:rule_status" json:"rule_status"`
	Note              string    `gorm:"column:note" json:"note"`
	CreatedAt         time.Time `gorm:"column:created_at" json:"created_at"`
}

func (ServiceClassifier) TableName() string {
	return "service_classifiers"
}
