package payment

type Amount struct {
	Value    string `json:"value"`
	Currency string `json:"currency"`
}

type Confirmation struct {
	Type            string `json:"type"`
	ReturnURL       string `json:"return_url,omitempty"`       // For redirect
	ConfirmationURL string `json:"confirmation_url,omitempty"` // From response
}

type CreatePaymentRequest struct {
	Amount       Amount            `json:"amount"`
	Capture      bool              `json:"capture"`
	Confirmation Confirmation      `json:"confirmation"`
	Description  string            `json:"description,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type PaymentResponse struct {
	ID           string            `json:"id"`
	Status       string            `json:"status"`
	Paid         bool              `json:"paid"`
	Amount       Amount            `json:"amount"`
	Confirmation Confirmation      `json:"confirmation"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// Webhook structures

type WebhookNotification struct {
	Type   string        `json:"type"`
	Event  string        `json:"event"`
	Object WebhookObject `json:"object"`
}

type WebhookObject struct {
	ID       string            `json:"id"`
	Status   string            `json:"status"`
	Paid     bool              `json:"paid"`
	Amount   Amount            `json:"amount"`
	Metadata map[string]string `json:"metadata"`
}
