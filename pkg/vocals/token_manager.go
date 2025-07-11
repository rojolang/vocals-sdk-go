package vocals

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type TokenManager struct {
	endpoint      string
	headers       map[string]string
	refreshBuffer float64
	token         *string
	expiresAt     time.Time
}

func NewTokenManager(endpoint string, headers map[string]string, refreshBuffer float64) *TokenManager {
	return &TokenManager{
		endpoint:      endpoint,
		headers:       headers,
		refreshBuffer: refreshBuffer,
	}
}

func (tm *TokenManager) GetToken() (string, error) {
	currentTime := time.Now()
	if tm.token != nil && currentTime.Before(tm.expiresAt.Add(time.Duration(-tm.refreshBuffer)*time.Second)) {
		return *tm.token, nil
	}

	return tm.refreshToken()
}

func (tm *TokenManager) refreshToken() (string, error) {
	reqHeaders := map[string]string{
		"Content-Type": "application/json",
	}
	for k, v := range tm.headers {
		reqHeaders[k] = v
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", tm.endpoint, bytes.NewBuffer([]byte("{}")))
	if err != nil {
		return "", err
	}

	for k, v := range reqHeaders {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to refresh token: %s", resp.Status)
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	token, ok := data["token"].(string)
	if !ok || token == "" {
		return "", fmt.Errorf("no token received")
	}

	expiresAt, ok := data["expiresAt"].(float64)
	if !ok {
		return "", fmt.Errorf("invalid expiresAt")
	}

	tm.token = &token
	tm.expiresAt = time.UnixMilli(int64(expiresAt))

	return token, nil
}

func (tm *TokenManager) Clear() {
	tm.token = nil
	tm.expiresAt = time.Time{}
}

func (tm *TokenManager) GetTokenInfo() (*string, *float64) {
	if tm.token == nil {
		return nil, nil
	}
	expires := float64(tm.expiresAt.UnixMilli())
	return tm.token, &expires
}