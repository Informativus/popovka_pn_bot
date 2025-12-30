package payment

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type Client struct {
	ShopID     string
	SecretKey  string
	APIURL     string
	HTTPClient *http.Client
}

func NewClient(shopID, secretKey string) *Client {
	return &Client{
		ShopID:    shopID,
		SecretKey: secretKey,
		APIURL:    "https://api.yookassa.ru/v3",
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) CreatePayment(amount string, currency string, description string, returnURL string, metadata map[string]string) (*PaymentResponse, error) {
	reqBody := CreatePaymentRequest{
		Amount: Amount{
			Value:    amount,
			Currency: currency,
		},
		Capture: true,
		Confirmation: Confirmation{
			Type:      "redirect",
			ReturnURL: returnURL,
		},
		Description: description,
		Metadata:    metadata,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/payments", c.APIURL), bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Idempotence Key
	req.Header.Set("Idempotence-Key", uuid.New().String())
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.ShopID, c.SecretKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("api error: %s (status: %d)", string(respBody), resp.StatusCode)
	}

	var paymentResponse PaymentResponse
	if err := json.Unmarshal(respBody, &paymentResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &paymentResponse, nil
}
