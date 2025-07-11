package vocals

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type VocalsConfig struct {
	TokenEndpoint        *string           `json:"token_endpoint,omitempty"`
	Headers              map[string]string `json:"headers,omitempty"`
	AutoConnect          bool              `json:"auto_connect"`
	MaxReconnectAttempts int               `json:"max_reconnect_attempts"`
	ReconnectDelay       float64           `json:"reconnect_delay"`
	TokenRefreshBuffer   float64           `json:"token_refresh_buffer"`
	WsEndpoint           *string           `json:"ws_endpoint,omitempty"`
	UseTokenAuth         bool              `json:"use_token_auth"`
	DebugLevel           string            `json:"debug_level"`
	DebugWebsocket       bool              `json:"debug_websocket"`
	DebugAudio           bool              `json:"debug_audio"`
	AudioDeviceID        *int              `json:"audio_device_id,omitempty"`
}

func NewVocalsConfig() *VocalsConfig {
	c := &VocalsConfig{
		AutoConnect:          false,
		MaxReconnectAttempts: 3,
		ReconnectDelay:       1.0,
		TokenRefreshBuffer:   60.0,
		UseTokenAuth:         true,
		DebugLevel:           "INFO",
		Headers:              make(map[string]string),
	}

	// Load from env
	c.loadFromEnv()

	return c
}

func (c *VocalsConfig) loadFromEnv() {
	// Load .env if exists
	_ = godotenv.Load()

	if endpoint := os.Getenv("VOCALS_TOKEN_ENDPOINT"); endpoint != "" {
		c.TokenEndpoint = &endpoint
	} else {
		defaultEndpoint := "/api/wstoken"
		c.TokenEndpoint = &defaultEndpoint
	}

	if wsEndpoint := os.Getenv("VOCALS_WS_ENDPOINT"); wsEndpoint != "" {
		c.WsEndpoint = &wsEndpoint
	} else {
		defaultWs := "ws://192.168.1.46:8000/v1/stream/conversation"
		c.WsEndpoint = &defaultWs
	}

	c.AutoConnect = os.Getenv("VOCALS_AUTO_CONNECT") == "true"
	
	if maxReconnect := os.Getenv("VOCALS_MAX_RECONNECT_ATTEMPTS"); maxReconnect != "" {
		if val, err := strconv.Atoi(maxReconnect); err == nil {
			c.MaxReconnectAttempts = val
		}
	}
	
	if delay := os.Getenv("VOCALS_RECONNECT_DELAY"); delay != "" {
		if val, err := strconv.ParseFloat(delay, 64); err == nil {
			c.ReconnectDelay = val
		}
	}
	
	if buffer := os.Getenv("VOCALS_TOKEN_REFRESH_BUFFER"); buffer != "" {
		if val, err := strconv.ParseFloat(buffer, 64); err == nil {
			c.TokenRefreshBuffer = val
		}
	}
	
	c.UseTokenAuth = os.Getenv("VOCALS_USE_TOKEN_AUTH") != "false"
	
	if level := os.Getenv("VOCALS_DEBUG_LEVEL"); level != "" {
		c.DebugLevel = level
	}
	
	c.DebugWebsocket = os.Getenv("VOCALS_DEBUG_WEBSOCKET") == "true"
	c.DebugAudio = os.Getenv("VOCALS_DEBUG_AUDIO") == "true"

	if deviceIDStr := os.Getenv("VOCALS_AUDIO_DEVICE_ID"); deviceIDStr != "" {
		if deviceID, err := strconv.Atoi(deviceIDStr); err == nil {
			c.AudioDeviceID = &deviceID
		}
	}
}

// Validate returns list of issues
func (c *VocalsConfig) Validate() []string {
	issues := []string{}

	// Check API key
	apiKey := os.Getenv("VOCALS_DEV_API_KEY")
	if apiKey == "" {
		issues = append(issues, "VOCALS_DEV_API_KEY environment variable not set")
	} else if !strings.HasPrefix(apiKey, "vdev_") {
		issues = append(issues, "Invalid API key format (should start with 'vdev_')")
	}

	// Check WebSocket endpoint
	if c.WsEndpoint != nil && !strings.HasPrefix(*c.WsEndpoint, "ws") {
		issues = append(issues, "Invalid WebSocket endpoint format")
	}

	// Check debug level
	validLevels := []string{"DEBUG", "INFO", "WARNING", "ERROR"}
	found := false
	for _, level := range validLevels {
		if level == c.DebugLevel {
			found = true
			break
		}
	}
	if !found {
		issues = append(issues, fmt.Sprintf("Invalid debug level: %s", c.DebugLevel))
	}

	// Check audio device (simplified, as Go has different audio handling)
	if c.AudioDeviceID != nil {
		// Audio device validation would require checking available devices
		// For now, we accept any device ID
	}

	return issues
}

func (c *VocalsConfig) PrintConfig() {
	fmt.Println("🎤 Vocals SDK Configuration")
	fmt.Println("==================================================")

	apiKey := os.Getenv("VOCALS_DEV_API_KEY")
	if apiKey != "" {
		fmt.Printf("API Key: %s...\n", apiKey[:10])
	} else {
		fmt.Println("API Key: NOT SET")
	}

	if c.WsEndpoint != nil {
		fmt.Printf("WebSocket Endpoint: %s\n", *c.WsEndpoint)
	}
	fmt.Printf("Auto Connect: %t\n", c.AutoConnect)
	fmt.Printf("Max Reconnect Attempts: %d\n", c.MaxReconnectAttempts)
	fmt.Printf("Reconnect Delay: %.1fs\n", c.ReconnectDelay)
	fmt.Printf("Token Refresh Buffer: %.1fs\n", c.TokenRefreshBuffer)
	fmt.Printf("Use Token Auth: %t\n", c.UseTokenAuth)
	fmt.Printf("Debug Level: %s\n", c.DebugLevel)
	fmt.Printf("Debug WebSocket: %t\n", c.DebugWebsocket)
	fmt.Printf("Debug Audio: %t\n", c.DebugAudio)

	if c.AudioDeviceID != nil {
		fmt.Printf("Audio Device ID: %d\n", *c.AudioDeviceID)
	} else {
		fmt.Println("Audio Device: Default")
	}
}