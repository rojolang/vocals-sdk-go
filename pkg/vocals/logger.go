package vocals

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// VocalsLogger wraps zerolog for structured logging
type VocalsLogger struct {
	logger zerolog.Logger
}

// LogLevel represents the logging level
type LogLevel int

const (
	TraceLevel LogLevel = iota
	DebugLevel
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
	PanicLevel
)

// LogConfig represents the configuration for logging
type LogConfig struct {
	Level     LogLevel
	Pretty    bool
	Output    io.Writer
	AddSource bool
	Fields    map[string]interface{}
}

// DefaultLogConfig returns a default logging configuration
func DefaultLogConfig() *LogConfig {
	return &LogConfig{
		Level:     InfoLevel,
		Pretty:    true,
		Output:    os.Stdout,
		AddSource: false,
		Fields:    make(map[string]interface{}),
	}
}

// NewVocalsLogger creates a new structured logger
func NewVocalsLogger(config *LogConfig) *VocalsLogger {
	if config == nil {
		config = DefaultLogConfig()
	}

	// Configure zerolog
	zerolog.TimeFieldFormat = time.RFC3339
	
	var logger zerolog.Logger
	
	if config.Pretty {
		logger = log.Output(zerolog.ConsoleWriter{
			Out:        config.Output,
			TimeFormat: time.Kitchen,
		})
	} else {
		logger = zerolog.New(config.Output)
	}

	// Set level
	switch config.Level {
	case TraceLevel:
		logger = logger.Level(zerolog.TraceLevel)
	case DebugLevel:
		logger = logger.Level(zerolog.DebugLevel)
	case InfoLevel:
		logger = logger.Level(zerolog.InfoLevel)
	case WarnLevel:
		logger = logger.Level(zerolog.WarnLevel)
	case ErrorLevel:
		logger = logger.Level(zerolog.ErrorLevel)
	case FatalLevel:
		logger = logger.Level(zerolog.FatalLevel)
	case PanicLevel:
		logger = logger.Level(zerolog.PanicLevel)
	}

	// Add timestamp
	logger = logger.With().Timestamp().Logger()

	// Add source if requested
	if config.AddSource {
		logger = logger.With().Caller().Logger()
	}

	// Add global fields
	if len(config.Fields) > 0 {
		logger = logger.With().Fields(config.Fields).Logger()
	}

	return &VocalsLogger{
		logger: logger,
	}
}

// WithComponent adds a component field to the logger
func (l *VocalsLogger) WithComponent(component string) *VocalsLogger {
	return &VocalsLogger{
		logger: l.logger.With().Str("component", component).Logger(),
	}
}

// WithField adds a field to the logger
func (l *VocalsLogger) WithField(key string, value interface{}) *VocalsLogger {
	return &VocalsLogger{
		logger: l.logger.With().Interface(key, value).Logger(),
	}
}

// WithFields adds multiple fields to the logger
func (l *VocalsLogger) WithFields(fields map[string]interface{}) *VocalsLogger {
	return &VocalsLogger{
		logger: l.logger.With().Fields(fields).Logger(),
	}
}

// WithError adds an error field to the logger
func (l *VocalsLogger) WithError(err error) *VocalsLogger {
	return &VocalsLogger{
		logger: l.logger.With().Err(err).Logger(),
	}
}

// Trace logs a trace level message
func (l *VocalsLogger) Trace(msg string) {
	l.logger.Trace().Msg(msg)
}

// Tracef logs a trace level formatted message
func (l *VocalsLogger) Tracef(format string, args ...interface{}) {
	l.logger.Trace().Msgf(format, args...)
}

// Debug logs a debug level message
func (l *VocalsLogger) Debug(msg string) {
	l.logger.Debug().Msg(msg)
}

// Debugf logs a debug level formatted message
func (l *VocalsLogger) Debugf(format string, args ...interface{}) {
	l.logger.Debug().Msgf(format, args...)
}

// Info logs an info level message
func (l *VocalsLogger) Info(msg string) {
	l.logger.Info().Msg(msg)
}

// Infof logs an info level formatted message
func (l *VocalsLogger) Infof(format string, args ...interface{}) {
	l.logger.Info().Msgf(format, args...)
}

// Warn logs a warn level message
func (l *VocalsLogger) Warn(msg string) {
	l.logger.Warn().Msg(msg)
}

// Warnf logs a warn level formatted message
func (l *VocalsLogger) Warnf(format string, args ...interface{}) {
	l.logger.Warn().Msgf(format, args...)
}

// Error logs an error level message
func (l *VocalsLogger) Error(msg string) {
	l.logger.Error().Msg(msg)
}

// Errorf logs an error level formatted message
func (l *VocalsLogger) Errorf(format string, args ...interface{}) {
	l.logger.Error().Msgf(format, args...)
}

// Fatal logs a fatal level message and exits
func (l *VocalsLogger) Fatal(msg string) {
	l.logger.Fatal().Msg(msg)
}

// Fatalf logs a fatal level formatted message and exits
func (l *VocalsLogger) Fatalf(format string, args ...interface{}) {
	l.logger.Fatal().Msgf(format, args...)
}

// LogAudioEvent logs audio-related events with structured fields
func (l *VocalsLogger) LogAudioEvent(event string, fields map[string]interface{}) {
	l.logger.Info().
		Str("event_type", "audio").
		Str("event", event).
		Fields(fields).
		Msg("Audio event")
}

// LogConnectionEvent logs connection-related events
func (l *VocalsLogger) LogConnectionEvent(event string, state ConnectionState, fields map[string]interface{}) {
	l.logger.Info().
		Str("event_type", "connection").
		Str("event", event).
		Str("state", string(state)).
		Fields(fields).
		Msg("Connection event")
}

// LogError logs a VocalsError with structured fields
func (l *VocalsLogger) LogError(err *VocalsError) {
	event := l.logger.Error().
		Str("error_code", err.Code).
		Float64("timestamp", err.Timestamp).
		Fields(err.Details)

	if err.Stack != "" {
		event = event.Str("stack", err.Stack)
	}

	event.Msg(err.Message)
}

// LogMessageEvent logs WebSocket message events
func (l *VocalsLogger) LogMessageEvent(msgType string, fields map[string]interface{}) {
	l.logger.Debug().
		Str("event_type", "message").
		Str("message_type", msgType).
		Fields(fields).
		Msg("Message event")
}

// LogStats logs streaming statistics
func (l *VocalsLogger) LogStats(stats *StreamStats) {
	l.logger.Info().
		Str("event_type", "stats").
		Dur("duration", stats.Duration).
		Int64("total_samples", stats.TotalSamples).
		Int64("total_bytes", stats.TotalBytes).
		Float32("avg_amplitude", stats.AverageAmplitude).
		Float32("max_amplitude", stats.MaxAmplitude).
		Float32("rms_amplitude", stats.RMSAmplitude).
		Float32("voice_activity_ratio", stats.VoiceActivityRatio).
		Float64("quality_score", stats.GetQualityScore()).
		Bool("healthy", stats.IsHealthy()).
		Msg("Stream statistics")
}

// Global logger instance
var globalLogger *VocalsLogger

func init() {
	globalLogger = NewVocalsLogger(DefaultLogConfig())
}

// GetGlobalLogger returns the global logger instance
func GetGlobalLogger() *VocalsLogger {
	return globalLogger
}

// SetGlobalLogger sets the global logger instance
func SetGlobalLogger(logger *VocalsLogger) {
	globalLogger = logger
}

// Global logging functions for convenience
func Trace(msg string) {
	globalLogger.Trace(msg)
}

func Tracef(format string, args ...interface{}) {
	globalLogger.Tracef(format, args...)
}

func Debug(msg string) {
	globalLogger.Debug(msg)
}

func Debugf(format string, args ...interface{}) {
	globalLogger.Debugf(format, args...)
}

func Info(msg string) {
	globalLogger.Info(msg)
}

func Infof(format string, args ...interface{}) {
	globalLogger.Infof(format, args...)
}

func Warn(msg string) {
	globalLogger.Warn(msg)
}

func Warnf(format string, args ...interface{}) {
	globalLogger.Warnf(format, args...)
}

func Error(msg string) {
	globalLogger.Error(msg)
}

func Errorf(format string, args ...interface{}) {
	globalLogger.Errorf(format, args...)
}

func Fatal(msg string) {
	globalLogger.Fatal(msg)
}

func Fatalf(format string, args ...interface{}) {
	globalLogger.Fatalf(format, args...)
}

func LogAudioEvent(event string, fields map[string]interface{}) {
	globalLogger.LogAudioEvent(event, fields)
}

func LogConnectionEvent(event string, state ConnectionState, fields map[string]interface{}) {
	globalLogger.LogConnectionEvent(event, state, fields)
}

func LogVocalsError(err *VocalsError) {
	globalLogger.LogError(err)
}

func LogMessageEvent(msgType string, fields map[string]interface{}) {
	globalLogger.LogMessageEvent(msgType, fields)
}

func LogStats(stats *StreamStats) {
	globalLogger.LogStats(stats)
}