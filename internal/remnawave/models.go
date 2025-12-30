package remnawave

type CreateUserRequest struct {
	Username             string   `json:"username"`
	Status               string   `json:"status"`
	TrafficLimitBytes    int64    `json:"trafficLimitBytes"`
	TrafficLimitStrategy string   `json:"trafficLimitStrategy"`
	ExpireAt             string   `json:"expireAt"` // ISO 8601 format
	Description          string   `json:"description,omitempty"`
	ActiveInternalSquads []string `json:"activeInternalSquads"`
}

type UserResponse struct {
	UUID                 string  `json:"uuid"`
	ID                   int     `json:"id"`
	ShortUUID            string  `json:"shortUuid"`
	Username             string  `json:"username"`
	Status               string  `json:"status"`
	TrafficLimitBytes    int64   `json:"trafficLimitBytes"`
	TrafficLimitStrategy string  `json:"trafficLimitStrategy"`
	ExpireAt             string  `json:"expireAt"`
	Description          string  `json:"description"`
	SubscriptionURL      string  `json:"subscriptionUrl"`
	ActiveInternalSquads []Squad `json:"activeInternalSquads"`
}

type Squad struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

// Wrapper for API responses
type APIResponse struct {
	Response UserResponse `json:"response"`
}

type ExtendSubscriptionRequest struct {
	ExpireAt string `json:"expireAt"` // ISO 8601 format
}
