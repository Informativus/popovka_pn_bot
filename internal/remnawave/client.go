package remnawave

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) doRequest(method, endpoint string, body interface{}) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewBuffer(jsonBody)
	}

	url := fmt.Sprintf("%s%s", c.BaseURL, endpoint)
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("api error: %s (status: %d)", string(respBody), resp.StatusCode)
	}

	return respBody, nil
}

func (c *Client) CreateUser(telegramID int64, username string) (*UserResponse, error) {
	reqBody := CreateUserRequest{
		TelegramID: telegramID,
		Username:   username,
	}

	resp, err := c.doRequest("POST", "/users", reqBody)
	if err != nil {
		return nil, err
	}

	var user UserResponse
	if err := json.Unmarshal(resp, &user); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &user, nil
}

func (c *Client) GetConfig(remnawaveID string) (string, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/users/%s/config", remnawaveID), nil)
	if err != nil {
		return "", err
	}

	// Assuming the API returns the config string directly or a JSON with a config field.
	// Based on typical behavior, let's assume it returns a JSON object with a "config" field.
	var result struct {
		Config string `json:"config"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		// If unmarshal fails, maybe it returned raw text?
		return string(resp), nil
	}

	if result.Config == "" {
		return string(resp), nil
	}

	return result.Config, nil
}

func (c *Client) ExtendSubscription(remnawaveID string, duration string) error {
	reqBody := ExtendSubscriptionRequest{
		Duration: duration,
	}

	_, err := c.doRequest("POST", fmt.Sprintf("/users/%s/extend", remnawaveID), reqBody)
	return err
}

func (c *Client) DeleteUser(remnawaveID string) error {
	_, err := c.doRequest("DELETE", fmt.Sprintf("/users/%s", remnawaveID), nil)
	return err
}
