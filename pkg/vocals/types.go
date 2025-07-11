package vocals

import (
	"fmt"
	"math"
	"runtime"
	"strings"
	"time"
)

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
	Stack     string                 // Stack trace for debugging
}

func (e *VocalsError) Error() string {
	var sb strings.Builder
	
	// Base error message
	if e.err != nil {
		sb.WriteString(fmt.Sprintf("[%s] %s (%s)", 
			time.Unix(0, int64(e.Timestamp*1000000)).Format(time.RFC3339), 
			e.err.Error(), e.Code))
	} else {
		sb.WriteString(fmt.Sprintf("[%s] %s (%s)", 
			time.Unix(0, int64(e.Timestamp*1000000)).Format(time.RFC3339), 
			e.Message, e.Code))
	}
	
	// Add details if available
	if len(e.Details) > 0 {
		sb.WriteString("\nDetails:")
		for k, v := range e.Details {
			sb.WriteString(fmt.Sprintf("\n  %s: %v", k, v))
		}
	}
	
	// Add stack trace if available
	if e.Stack != "" {
		sb.WriteString("\nStack trace:\n" + e.Stack)
	}
	
	return sb.String()
}

func NewVocalsError(message, code string) *VocalsError {
	err := &VocalsError{
		Message:   message,
		Code:      code,
		Timestamp: float64(time.Now().UnixMilli()),
		Details:   make(map[string]interface{}),
	}
	err.captureStack()
	return err
}

// captureStack records the current stack trace (skipping this function and NewVocalsError).
func (e *VocalsError) captureStack() {
	buf := make([]byte, 8192) // Buffer size for stack; adjust if needed for deep stacks
	n := runtime.Stack(buf, false)
	e.Stack = string(buf[:n])
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
	Event      string      `json:"event"`
	Data       interface{} `json:"data"`
	Format     *string     `json:"format,omitempty"`
	SampleRate *int        `json:"sampleRate,omitempty"`
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

// StreamStats represents comprehensive streaming statistics
type StreamStats struct {
	StartTime          time.Time
	EndTime            time.Time
	Duration           time.Duration
	TotalSamples       int64
	TotalBytes         int64
	AverageAmplitude   float32
	MaxAmplitude       float32
	MinAmplitude       float32
	RMSAmplitude       float32
	SilenceDuration    time.Duration
	VoiceActivityRatio float32
	PacketsSent        int64
	PacketsLost        int64
	ConnectionDrops    int32
	ReconnectCount     int32
}

// StreamStatsCallback is called with updated statistics
type StreamStatsCallback func(*StreamStats)

// AudioLevelCallback is called with real-time audio levels
type AudioLevelCallback func(avgLevel, maxLevel float32)

// SilenceDetectionCallback is called when silence is detected
type SilenceDetectionCallback func(duration time.Duration)

// Helper methods for StreamStats
func (s *StreamStats) GetSampleRate() float64 {
	if s.Duration == 0 {
		return 0
	}
	return float64(s.TotalSamples) / s.Duration.Seconds()
}

func (s *StreamStats) GetBytesPerSecond() float64 {
	if s.Duration == 0 {
		return 0
	}
	return float64(s.TotalBytes) / s.Duration.Seconds()
}

func (s *StreamStats) GetVoiceActivityPercentage() float64 {
	return float64(s.VoiceActivityRatio * 100)
}

func (s *StreamStats) GetSilencePercentage() float64 {
	if s.Duration == 0 {
		return 0
	}
	return (s.SilenceDuration.Seconds() / s.Duration.Seconds()) * 100
}

func (s *StreamStats) IsHealthy() bool {
	return s.MaxAmplitude > 0.001 && s.VoiceActivityRatio > 0.1 && s.TotalSamples > 0
}

func (s *StreamStats) GetQualityScore() float64 {
	if !s.IsHealthy() {
		return 0.0
	}
	// Quality score based on voice activity ratio and amplitude consistency
	activityScore := math.Min(float64(s.VoiceActivityRatio*2), 1.0) // Max 1.0 for 50%+ voice activity
	amplitudeScore := math.Min(float64(s.AverageAmplitude*10), 1.0)  // Max 1.0 for 0.1+ average amplitude
	return (activityScore + amplitudeScore) / 2.0
}
