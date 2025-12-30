package models

import (
	"time"
)

type ReferralTransaction struct {
	ID            uint    `gorm:"primaryKey"`
	ReferrerID    uint    `gorm:"not null;index"`
	InvitedUserID uint    `gorm:"not null;index"`
	Amount        float64 `gorm:"not null"`
	CreatedAt     time.Time
}
