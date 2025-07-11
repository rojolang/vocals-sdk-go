package vocals

import "time"

// Result types for error handling
type Result[T any] struct {
	Data    T
	Error   *VocalsError
	Success bool
}

func Ok[T any](data T) Result[T] {
	return Result[T]{Data: data, Success: true}
}

func Err[T any](err *VocalsError) Result[T] {
	return Result[T]{Error: err, Success: false}
}

// ValidatedApiKey is a type alias for string
type ValidatedApiKey string

// WSToken struct
type WSToken struct {
	Token     string
	ExpiresAt int64 // Unix timestamp in milliseconds
}

// ConnectionState enum
type ConnectionState string

const (
	Disconnected ConnectionState = "disconnected"
	Connecting   ConnectionState = "connecting"
	Connected    ConnectionState = "connected"
	Reconnecting ConnectionState = "reconnecting"
	ErrorState   ConnectionState = "error"
)

// RecordingState enum
type RecordingState string

const (
	IdleRecording       RecordingState = "idle"
	Recording           RecordingState = "recording"
	ProcessingRecording RecordingState = "processing"
	CompletedRecording  RecordingState = "completed"
	ErrorRecording      RecordingState = "error"
)

// PlaybackState enum
type PlaybackState string

const (
	IdlePlayback    PlaybackState = "idle"
	PlayingPlayback PlaybackState = "playing"
	PausedPlayback  PlaybackState = "paused"
	QueuedPlayback  PlaybackState = "queued"
	ErrorPlayback   PlaybackState = "error"
)

// VocalsError struct
type VocalsError struct {
	Message   string
	Code      string
	Timestamp float64
	err       error
	Details   map[string]interface{} // Additional details about the error
}

func (e *VocalsError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return e.Message
}

func NewVocalsError(message, code string) *VocalsError {
	return &VocalsError{
		Message:   message,
		Code:      code,
		Timestamp: float64(time.Now().UnixMilli()),
	}
}

// VocalsSDKException is a custom error type
type VocalsSDKException struct {
	Code      string
	Message   string
	Timestamp float64
}

func (e *VocalsSDKException) Error() string {
	return e.Message
}

// TTSAudioSegment struct
type TTSAudioSegment struct {
	Text             string
	AudioData        string // Base64 encoded WAV
	SampleRate       int
	SegmentID        string
	SentenceNumber   int
	GenerationTimeMs int
	Format           string
	DurationSeconds  float64
}

// SpeechInterruptionData struct
type SpeechInterruptionData struct {
	SegmentID    string
	StartTime    float64
	Reason       string
	ConnectionID *int
	Timestamp    *float64
}

// WebSocketMessage struct
type WebSocketMessage struct {
	Event string

	Data       any
	Format     *string
	SampleRate *int
	msg        *string
}

// WebSocketResponse struct
type WebSocketResponse struct {
	Event string
	Data  any
	Type  *string
}

// User struct for API operations
type User struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name,omitempty"`
	Created  int64  `json:"created"`
	Modified int64  `json:"modified,omitempty"`
}

// Handler types
type MessageHandler func(*WebSocketResponse)
type ConnectionHandler func(ConnectionState)
type ErrorHandler func(*VocalsError)
type AudioDataHandler func([]float32)
