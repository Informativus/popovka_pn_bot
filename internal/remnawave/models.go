package remnawave

type CreateUserRequest struct {
	TelegramID int64  `json:"telegram_id"`
	Username   string `json:"username,omitempty"`
}

type UserResponse struct {
	ID           string        `json:"id"`
	TelegramID   int64         `json:"telegram_id"`
	Username     string        `json:"username"`
	Subscription *Subscription `json:"subscription,omitempty"`
}

type Subscription struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	ExpiresAt string `json:"expires_at"`
	Config    string `json:"config"` // VLESS/VKEY config link or string
}

type ExtendSubscriptionRequest struct {
	Duration string `json:"duration"` // e.g., "30d"
}
