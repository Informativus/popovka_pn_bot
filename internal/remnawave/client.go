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

func (c *Client) CreateUser(telegramID int64, username string, durationDays int, squadID string) (*UserResponse, error) {
	// Calculate expiration date
	expireAt := time.Now().Add(time.Duration(durationDays) * 24 * time.Hour)

	squads := []string{}
	if squadID != "" {
		squads = append(squads, squadID)
	}

	reqBody := CreateUserRequest{
		Username:             fmt.Sprintf("tg_%d", telegramID),
		Status:               "ACTIVE",
		TrafficLimitBytes:    0,
		TrafficLimitStrategy: "NO_RESET",
		ExpireAt:             expireAt.Format(time.RFC3339),
		Description:          fmt.Sprintf("Telegram User: %s (ID: %d)", username, telegramID),
		ActiveInternalSquads: squads,
	}

	resp, err := c.doRequest("POST", "/api/users/", reqBody)
	if err != nil {
		return nil, err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Debug logging
	fmt.Printf("DEBUG Remnawave CreateUser response: UUID='%s', Username='%s', Status='%s'\n", apiResp.Response.UUID, apiResp.Response.Username, apiResp.Response.Status)

	return &apiResp.Response, nil
}

func (c *Client) ExtendSubscription(remnawaveID string, durationDays int) error {
	// Calculate new expiration date
	expireAt := time.Now().Add(time.Duration(durationDays) * 24 * time.Hour)

	reqBody := ExtendSubscriptionRequest{
		ExpireAt: expireAt.Format(time.RFC3339),
	}

	_, err := c.doRequest("POST", fmt.Sprintf("/api/users/%s/extend", remnawaveID), reqBody)
	return err
}

func (c *Client) DeleteUser(remnawaveID string) error {
	_, err := c.doRequest("DELETE", fmt.Sprintf("/users/%s", remnawaveID), nil)
	return err
}

func (c *Client) DisableUser(remnawaveID string) error {
	_, err := c.doRequest("POST", fmt.Sprintf("/users/%s/actions/disable", remnawaveID), nil)
	return err
}

func (c *Client) GetUser(remnawaveID string) (*UserResponse, error) {
	resp, err := c.doRequest("GET", fmt.Sprintf("/api/users/%s", remnawaveID), nil)
	if err != nil {
		return nil, err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &apiResp.Response, nil
}
