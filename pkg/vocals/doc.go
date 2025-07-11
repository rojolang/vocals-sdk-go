// Package vocals provides a comprehensive Go SDK for real-time audio processing
// and WebSocket communication with the Vocals API.
//
// # Overview
//
// The Vocals SDK Go provides a complete solution for:
//   - Real-time audio recording and playback
//   - WebSocket communication with auto-reconnection
//   - Audio device management and validation
//   - Comprehensive statistics and monitoring
//   - Structured logging with Zerolog
//   - Type-safe message handling
//
// # Quick Start
//
// Basic usage example:
//
//	config := vocals.NewVocalsConfig()
//	audioConfig := vocals.NewAudioConfig()
//	client := vocals.NewVocalsClient(config, audioConfig, nil, []string{})
//
//	// Add handlers
//	client.AddMessageHandler(vocals.CreateLoggingMessageHandler(true))
//	client.AddErrorHandler(vocals.CreateErrorLoggingHandler("Main"))
//
//	// Stream microphone for 5 seconds
//	err := client.StreamMicrophone(5.0)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	client.Cleanup()
//
// # Configuration
//
// The SDK uses two main configuration structures:
//
// VocalsConfig for general client settings:
//
//	config := vocals.NewVocalsConfig()
//	config.AutoConnect = true
//	config.MaxReconnectAttempts = 5
//	config.ReconnectDelay = 2.0
//
// AudioConfig for audio-specific settings:
//
//	audioConfig := vocals.NewAudioConfig()
//	audioConfig.SampleRate = 44100
//	audioConfig.Channels = 2
//	audioConfig.BufferSize = 2048
//
// # Audio Device Management
//
// The SDK provides comprehensive audio device management:
//
//	// List all devices
//	devices, err := vocals.GetAllAudioDevices()
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Validate a device
//	err = vocals.ValidateAudioDevice(deviceID, true, 1, 44100)
//	if err != nil {
//		log.Printf("Device validation failed: %v", err)
//	}
//
// # Message Handlers
//
// The SDK provides built-in handlers for common message types:
//
//	// Transcription handler
//	client.AddMessageHandler(vocals.CreateTranscriptionHandler(func(text string, isFinal bool) {
//		fmt.Printf("Transcription: %s (final: %v)\n", text, isFinal)
//	}))
//
//	// TTS handler
//	client.AddMessageHandler(vocals.CreateTTSHandler(func(segment vocals.TTSAudioSegment) {
//		fmt.Printf("TTS Audio: %s\n", segment.Text)
//	}))
//
//	// Custom handler
//	client.AddMessageHandler(func(msg *vocals.WebSocketResponse) {
//		// Handle custom messages
//	})
//
// # Statistics and Monitoring
//
// Advanced streaming with real-time statistics:
//
//	stats, err := client.StreamMicrophoneWithStats(
//		10.0,                    // duration
//		func(stats *vocals.StreamStats) {
//			fmt.Printf("Real-time stats: %+v\n", stats)
//		},
//		func(avgLevel, maxLevel float32) {
//			fmt.Printf("Audio levels: avg=%.3f, max=%.3f\n", avgLevel, maxLevel)
//		},
//		0.01,                    // silence threshold
//		func(duration time.Duration) {
//			fmt.Printf("Silence detected: %v\n", duration)
//		},
//	)
//
// # Structured Logging
//
// The SDK uses structured logging throughout:
//
//	// Global logger
//	vocals.Info("Application started")
//	vocals.LogAudioEvent("recording_started", map[string]interface{}{
//		"duration": 5.0,
//		"sample_rate": 44100,
//	})
//
//	// Custom logger
//	logConfig := vocals.DefaultLogConfig()
//	logConfig.Level = vocals.DebugLevel
//	logger := vocals.NewVocalsLogger(logConfig)
//	logger.WithComponent("MyComponent").Info("Component initialized")
//
// # Error Handling
//
// The SDK provides enhanced error handling with stack traces:
//
//	err := vocals.NewVocalsError("Connection failed", "CONNECTION_FAILED")
//	err.AddDetail("endpoint", "ws://localhost:8080")
//	err.AddDetail("attempts", 3)
//
//	// Check error types
//	if vocals.IsRetryableError(err) {
//		// Retry the operation
//	}
//
//	if vocals.IsCriticalError(err) {
//		// Handle critical error
//	}
//
// # CLI Tool
//
// The SDK includes a comprehensive CLI tool:
//
//	# List available devices
//	./vocals devices list
//
//	# Test a device
//	./vocals devices test 1
//
//	# Demo recording
//	./vocals demo record --duration 10
//
//	# Demo with statistics
//	./vocals demo stats --duration 15 --verbose
//
// # Thread Safety
//
// The SDK is designed to be thread-safe:
//   - All client operations are protected by mutexes
//   - Handlers are executed in separate goroutines
//   - Statistics collection is thread-safe
//   - Audio processing uses proper synchronization
//
// # Performance Considerations
//
// For optimal performance:
//   - Use appropriate buffer sizes for your use case
//   - Monitor memory usage with statistics
//   - Handle errors gracefully to avoid connection drops
//   - Use appropriate log levels in production
//
// # Dependencies
//
// The SDK depends on:
//   - github.com/gordonklaus/portaudio: Audio I/O
//   - github.com/gorilla/websocket: WebSocket client
//   - github.com/rs/zerolog: Structured logging
//   - github.com/spf13/cobra: CLI framework
//   - github.com/golang-jwt/jwt/v4: JWT handling
//   - github.com/joho/godotenv: Environment variables
package vocals