package models

import (
	"time"
)

type User struct {
	ID         uint      `gorm:"primaryKey"`
	TelegramID int64     `gorm:"uniqueIndex;not null"`
	Username   string    `gorm:"size:255"`
	Status     string    `gorm:"default:'active'"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
