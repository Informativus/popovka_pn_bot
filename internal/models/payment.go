package models

import (
	"time"
)

type Payment struct {
	ID         uint    `gorm:"primaryKey"`
	UserID     uint    `gorm:"not null;index"`
	User       User    `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	Amount     float64 `gorm:"not null"`
	Status     string  `gorm:"default:'pending'"`
	YooKassaID string  `gorm:"size:255"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
