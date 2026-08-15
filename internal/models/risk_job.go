package models

import "time"

const (
	RiskJobStatusQueued  = "queued"
	RiskJobStatusRunning = "running"
	RiskJobStatusDone    = "done"
	RiskJobStatusFailed  = "failed"
)

type RiskJob struct {
	ID            uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	Status        string     `gorm:"type:varchar(20);index;not null" json:"status"`
	SourceFile    string     `gorm:"type:varchar(255)" json:"source_file"`
	LoadedRecords int64      `gorm:"not null;default:0" json:"loaded_records"`
	RisksFound    int        `gorm:"not null;default:0" json:"risks_found"`
	ErrorMessage  string     `gorm:"type:text" json:"error_message"`
	StartedAt     *time.Time `json:"started_at"`
	FinishedAt    *time.Time `json:"finished_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (RiskJob) TableName() string {
	return "risk_jobs"
}
