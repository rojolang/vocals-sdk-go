package vocals

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"
)

// Error codes as constants
const (
	ErrCodeConnectionFailed    = "CONNECTION_FAILED"
	ErrCodeReconnectFailed     = "RECONNECT_FAILED"
	ErrCodeTokenExpired        = "TOKEN_EXPIRED"
	ErrCodeAudioDevice         = "AUDIO_DEVICE_ERROR"
	ErrCodePlayback            = "PLAYBACK_ERROR"
	ErrCodeWebSocket           = "WEBSOCKET_ERROR"
	ErrCodeTranscriptionFailed = "TRANSCRIPTION_FAILED"
	ErrCodeResponseFailed      = "RESPONSE_FAILED"
	ErrCodeInterruptFailed     = "INTERRUPT_FAILED"
	ErrCodeConfigInvalid       = "CONFIG_INVALID"
	ErrCodeJSONParse           = "JSON_PARSE_ERROR"
	ErrCodeUnknown             = "UNKNOWN_ERROR"
	ErrCodeTimeout             = "TIMEOUT_ERROR"
	ErrCodeAuthFailed          = "AUTH_FAILED"
)

// VocalsError represents an enhanced error with additional context
type VocalsErrorEnhanced struct {
	Message   string
	Code      string
	Details   map[string]interface{}
	Stack     string
	Timestamp time.Time
}

func NewVocalsErrorEnhanced(message, code string) *VocalsErrorEnhanced {
	err := &VocalsErrorEnhanced{
		Message:   message,
		Code:      code,
		Details:   make(map[string]interface{}),
		Timestamp: time.Now(),
	}
	err.captureStack()
	log.Printf("[%s] New error: %s (%s)", err.Timestamp.Format(time.RFC3339), message, code)
	return err
}

func (e *VocalsErrorEnhanced) captureStack() {
	buf := make([]byte, 8192)
	n := runtime.Stack(buf, false)
	e.Stack = string(buf[:n])
}

func (e *VocalsErrorEnhanced) AddDetail(key string, value interface{}) *VocalsErrorEnhanced {
	e.Details[key] = value
	return e
}

func (e *VocalsErrorEnhanced) Error() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s] %s (%s)", e.Timestamp.Format(time.RFC3339), e.Message, e.Code))
	if len(e.Details) > 0 {
		sb.WriteString(": Details - ")
		for k, v := range e.Details {
			sb.WriteString(fmt.Sprintf("%s: %v; ", k, v))
		}
	}
	if e.Stack != "" {
		sb.WriteString("\nStack trace:\n" + e.Stack)
	}
	return sb.String()
}

// Specific error creators with common codes
func NewConnectionError(message string) *VocalsError {
	return NewVocalsError(message, ErrCodeConnectionFailed)
}

func NewReconnectError(message string, attempts int) *VocalsError {
	return NewVocalsError(message, ErrCodeReconnectFailed).AddDetail("attempts", attempts).AddDetail("max_attempts", 5)
}

func NewAudioError(message string) *VocalsError {
	return NewVocalsError(message, ErrCodeAudioDevice).AddDetail("device", "default")
}

func NewPlaybackError(message string) *VocalsError {
	return NewVocalsError(message, ErrCodePlayback).AddDetail("segment_id", "unknown")
}

func NewTokenError(message string) *VocalsError {
	return NewVocalsError(message, ErrCodeTokenExpired).AddDetail("expiry", time.Now().UnixMilli())
}

func NewWebSocketError(message string) *VocalsError {
	return NewVocalsError(message, ErrCodeWebSocket).AddDetail("endpoint", "ws://default")
}

func NewTranscriptionError(message string) *VocalsError {
	return NewVocalsError(message, ErrCodeTranscriptionFailed).AddDetail("text_length", 0)
}

func NewResponseError(message string) *VocalsError {
	return NewVocalsError(message, ErrCodeResponseFailed).AddDetail("prompt_length", 0)
}

func NewInterruptError(message string) *VocalsError {
	return NewVocalsError(message, ErrCodeInterruptFailed).AddDetail("reason", "unknown")
}

func NewConfigError(message string) *VocalsError {
	return NewVocalsError(message, ErrCodeConfigInvalid)
}

func NewJSONError(message string) *VocalsError {
	return NewVocalsError(message, ErrCodeJSONParse)
}

func NewUnknownError(message string) *VocalsError {
	return NewVocalsError(message, ErrCodeUnknown)
}

func NewTimeoutError(message string) *VocalsError {
	return NewVocalsError(message, ErrCodeTimeout)
}

func NewAuthError(message string) *VocalsError {
	return NewVocalsError(message, ErrCodeAuthFailed)
}

// Helper to wrap any error as VocalsError
func WrapError(err error, code string) *VocalsError {
	if err == nil {
		return nil
	}
	vErr := NewVocalsError(err.Error(), code)
	vErr.AddDetail("original_error", err.Error())
	return vErr
}

// Helper to check if error has specific code
func IsErrorCode(err *VocalsError, code string) bool {
	if err == nil {
		return false
	}
	return err.Code == code
}

// Helper to log errors
func LogError(err *VocalsError) {
	if err != nil {
		log.Println(err.Error())
	}
}

// Helper to add details to existing VocalsError
func (e *VocalsError) AddDetail(key string, value interface{}) *VocalsError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// Helper to get error details
func (e *VocalsError) GetDetail(key string) (interface{}, bool) {
	if e.Details == nil {
		return nil, false
	}
	value, exists := e.Details[key]
	return value, exists
}

// Helper to check if error is retryable
func IsRetryableError(err *VocalsError) bool {
	if err == nil {
		return false
	}
	retryableCodes := []string{
		ErrCodeConnectionFailed,
		ErrCodeReconnectFailed,
		ErrCodeWebSocket,
		ErrCodeTimeout,
	}
	for _, code := range retryableCodes {
		if err.Code == code {
			return true
		}
	}
	return false
}

// Helper to check if error is critical
func IsCriticalError(err *VocalsError) bool {
	if err == nil {
		return false
	}
	criticalCodes := []string{
		ErrCodeAuthFailed,
		ErrCodeTokenExpired,
		ErrCodeConfigInvalid,
	}
	for _, code := range criticalCodes {
		if err.Code == code {
			return true
		}
	}
	return false
}