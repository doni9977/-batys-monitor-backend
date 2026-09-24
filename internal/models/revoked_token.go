package models

import "time"

type RevokedToken struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	JTI       string    `gorm:"type:varchar(128);uniqueIndex;not null" json:"-"`
	ExpiresAt time.Time `gorm:"index;not null" json:"-"`
	CreatedAt time.Time `json:"-"`
}

func (RevokedToken) TableName() string {
	return "revoked_tokens"
}
