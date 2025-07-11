package vocals

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type APIClient struct {
	baseURL    string
	apiKey     *string
	httpClient *http.Client
}

func NewAPIClient(baseURL string, apiKey *string) *APIClient {
	if baseURL == "" {
		baseURL = "https://api.vocals.example.com" // Default base URL
	}
	return &APIClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				IdleConnTimeout:     30 * time.Second,
				DisableCompression:  false,
				MaxIdleConnsPerHost: 2,
			},
		},
	}
}

func (ac *APIClient) request(method, endpoint string, body interface{}, headers map[string]string) ([]byte, error) {
	url := ac.baseURL + endpoint
	var reqBody io.Reader

	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, NewJSONError(err.Error())
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, NewConfigError(err.Error())
	}

	// Set default headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "VocalsSDK-Go/1.0")

	// Set authorization header if API key is provided
	if ac.apiKey != nil {
		req.Header.Set("Authorization", "Bearer "+*ac.apiKey)
	}

	// Set custom headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := ac.httpClient.Do(req)
	if err != nil {
		return nil, NewConnectionError(err.Error())
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewUnknownError(err.Error())
	}

	if resp.StatusCode >= 400 {
		errMsg := string(respBody)
		if errMsg == "" {
			errMsg = http.StatusText(resp.StatusCode)
		}
		return nil, NewVocalsError(errMsg, fmt.Sprintf("HTTP_%d", resp.StatusCode)).AddDetail("status_code", resp.StatusCode)
	}

	return respBody, nil
}

// Token operations
func (ac *APIClient) GenerateWsToken() Result[*WSToken] {
	resp, err := ac.request("POST", "/v1/tokens/ws", nil, nil)
	if err != nil {
		return Result[*WSToken]{Error: WrapError(err, ErrCodeTokenExpired)}
	}

	var token WSToken
	if err := json.Unmarshal(resp, &token); err != nil {
		return Result[*WSToken]{Error: NewJSONError(err.Error())}
	}

	return Result[*WSToken]{Success: true, Data: &token}
}

func (ac *APIClient) GenerateWsTokenWithUserId(userID string) Result[*WSToken] {
	if userID == "" {
		return Result[*WSToken]{Error: NewConfigError("user ID cannot be empty")}
	}

	body := map[string]string{"user_id": userID}
	resp, err := ac.request("POST", "/v1/tokens/ws", body, nil)
	if err != nil {
		return Result[*WSToken]{Error: WrapError(err, ErrCodeTokenExpired)}
	}

	var token WSToken
	if err := json.Unmarshal(resp, &token); err != nil {
		return Result[*WSToken]{Error: NewJSONError(err.Error())}
	}

	return Result[*WSToken]{Success: true, Data: &token}
}

func (ac *APIClient) RefreshToken(token string) Result[*WSToken] {
	if token == "" {
		return Result[*WSToken]{Error: NewConfigError("token cannot be empty")}
	}

	body := map[string]string{"token": token}
	resp, err := ac.request("POST", "/v1/tokens/refresh", body, nil)
	if err != nil {
		return Result[*WSToken]{Error: WrapError(err, ErrCodeTokenExpired)}
	}

	var newToken WSToken
	if err := json.Unmarshal(resp, &newToken); err != nil {
		return Result[*WSToken]{Error: NewJSONError(err.Error())}
	}

	return Result[*WSToken]{Success: true, Data: &newToken}
}

// User operations
func (ac *APIClient) AddUser(email, password string) Result[*User] {
	if email == "" || password == "" {
		return Result[*User]{Error: NewConfigError("email and password cannot be empty")}
	}

	body := map[string]string{"email": email, "password": password}
	resp, err := ac.request("POST", "/v1/users", body, nil)
	if err != nil {
		return Result[*User]{Error: WrapError(err, ErrCodeAuthFailed)}
	}

	var user User
	if err := json.Unmarshal(resp, &user); err != nil {
		return Result[*User]{Error: NewJSONError(err.Error())}
	}

	return Result[*User]{Success: true, Data: &user}
}

func (ac *APIClient) GetUser(userID string) Result[*User] {
	if userID == "" {
		return Result[*User]{Error: NewConfigError("user ID cannot be empty")}
	}

	resp, err := ac.request("GET", "/v1/users/"+userID, nil, nil)
	if err != nil {
		return Result[*User]{Error: WrapError(err, ErrCodeUnknown)}
	}

	var user User
	if err := json.Unmarshal(resp, &user); err != nil {
		return Result[*User]{Error: NewJSONError(err.Error())}
	}

	return Result[*User]{Success: true, Data: &user}
}

func (ac *APIClient) UpdateUser(userID string, updates map[string]interface{}) Result[*User] {
	if userID == "" {
		return Result[*User]{Error: NewConfigError("user ID cannot be empty")}
	}

	resp, err := ac.request("PUT", "/v1/users/"+userID, updates, nil)
	if err != nil {
		return Result[*User]{Error: WrapError(err, ErrCodeUnknown)}
	}

	var user User
	if err := json.Unmarshal(resp, &user); err != nil {
		return Result[*User]{Error: NewJSONError(err.Error())}
	}

	return Result[*User]{Success: true, Data: &user}
}

func (ac *APIClient) DeleteUser(userID string) Result[bool] {
	if userID == "" {
		return Result[bool]{Error: NewConfigError("user ID cannot be empty")}
	}

	_, err := ac.request("DELETE", "/v1/users/"+userID, nil, nil)
	if err != nil {
		return Result[bool]{Error: WrapError(err, ErrCodeUnknown)}
	}

	return Result[bool]{Success: true, Data: true}
}

func (ac *APIClient) ListUsers(page, limit int) Result[[]*User] {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	headers := map[string]string{
		"X-Page":  fmt.Sprint(page),
		"X-Limit": fmt.Sprint(limit),
	}

	resp, err := ac.request("GET", "/v1/users", nil, headers)
	if err != nil {
		return Result[[]*User]{Error: WrapError(err, ErrCodeUnknown)}
	}

	var users []*User
	if err := json.Unmarshal(resp, &users); err != nil {
		return Result[[]*User]{Error: NewJSONError(err.Error())}
	}

	return Result[[]*User]{Success: true, Data: users}
}

// Audio operations
func (ac *APIClient) UploadAudio(audioData []byte, sampleRate int, format string) Result[map[string]interface{}] {
	if len(audioData) == 0 {
		return Result[map[string]interface{}]{Error: NewConfigError("audio data cannot be empty")}
	}

	body := map[string]interface{}{
		"audio_data":  audioData,
		"sample_rate": sampleRate,
		"format":      format,
	}

	resp, err := ac.request("POST", "/v1/audio/upload", body, nil)
	if err != nil {
		return Result[map[string]interface{}]{Error: WrapError(err, ErrCodeUnknown)}
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return Result[map[string]interface{}]{Error: NewJSONError(err.Error())}
	}

	return Result[map[string]interface{}]{Success: true, Data: result}
}

func (ac *APIClient) GetAudioDevices() Result[[]map[string]interface{}] {
	resp, err := ac.request("GET", "/v1/audio/devices", nil, nil)
	if err != nil {
		return Result[[]map[string]interface{}]{Error: WrapError(err, ErrCodeUnknown)}
	}

	var devices []map[string]interface{}
	if err := json.Unmarshal(resp, &devices); err != nil {
		return Result[[]map[string]interface{}]{Error: NewJSONError(err.Error())}
	}

	return Result[[]map[string]interface{}]{Success: true, Data: devices}
}

// Conversation operations
func (ac *APIClient) CreateConversation(userID string, config map[string]interface{}) Result[map[string]interface{}] {
	if userID == "" {
		return Result[map[string]interface{}]{Error: NewConfigError("user ID cannot be empty")}
	}

	body := map[string]interface{}{
		"user_id": userID,
		"config":  config,
	}

	resp, err := ac.request("POST", "/v1/conversations", body, nil)
	if err != nil {
		return Result[map[string]interface{}]{Error: WrapError(err, ErrCodeUnknown)}
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return Result[map[string]interface{}]{Error: NewJSONError(err.Error())}
	}

	return Result[map[string]interface{}]{Success: true, Data: result}
}

func (ac *APIClient) GetConversation(conversationID string) Result[map[string]interface{}] {
	if conversationID == "" {
		return Result[map[string]interface{}]{Error: NewConfigError("conversation ID cannot be empty")}
	}

	resp, err := ac.request("GET", "/v1/conversations/"+conversationID, nil, nil)
	if err != nil {
		return Result[map[string]interface{}]{Error: WrapError(err, ErrCodeUnknown)}
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return Result[map[string]interface{}]{Error: NewJSONError(err.Error())}
	}

	return Result[map[string]interface{}]{Success: true, Data: result}
}

// Utility methods
func (ac *APIClient) SetAPIKey(apiKey string) {
	ac.apiKey = &apiKey
}

func (ac *APIClient) SetBaseURL(baseURL string) {
	ac.baseURL = baseURL
}

func (ac *APIClient) SetTimeout(timeout time.Duration) {
	ac.httpClient.Timeout = timeout
}

func (ac *APIClient) HealthCheck() Result[map[string]interface{}] {
	resp, err := ac.request("GET", "/v1/health", nil, nil)
	if err != nil {
		return Result[map[string]interface{}]{Error: WrapError(err, ErrCodeConnectionFailed)}
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return Result[map[string]interface{}]{Error: NewJSONError(err.Error())}
	}

	return Result[map[string]interface{}]{Success: true, Data: result}
}

// Helper function to create a default API client from environment
func NewAPIClientFromEnv() *APIClient {
	apiKey := os.Getenv("VOCALS_API_KEY")
	baseURL := os.Getenv("VOCALS_API_BASE_URL")

	var apiKeyPtr *string
	if apiKey != "" {
		apiKeyPtr = &apiKey
	}

	return NewAPIClient(baseURL, apiKeyPtr)
}
