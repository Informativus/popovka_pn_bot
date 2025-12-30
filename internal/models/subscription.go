package models

import (
	"time"
)

type Subscription struct {
	ID              uint   `gorm:"primaryKey"`
	UserID          uint   `gorm:"not null;index"`
	User            User   `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	RemnawaveID     string `gorm:"size:255"`
	SubscriptionURL string `gorm:"size:512"` // VPN subscription link
	ExpirationDate  time.Time
	PlanType        string `gorm:"size:50"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
