package models

import (
	"time"
)

type User struct {
	ID           uint    `gorm:"primaryKey"`
	TelegramID   int64   `gorm:"uniqueIndex;not null"`
	Username     string  `gorm:"size:255"`
	Status       string  `gorm:"default:'active'"`
	Balance      float64 `gorm:"default:0"`
	ReferrerID   *uint   `gorm:"index"`
	ReferralCode string  `gorm:"size:32;uniqueIndex"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
