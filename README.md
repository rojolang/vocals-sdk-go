# Vocals SDK for Go

[![Go Version](https://img.shields.io/badge/go-1.24-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen.svg)](https://github.com/rojolang/vocals-sdk-go)

A production-ready Go implementation of a real-time voice AI client, supporting WebSocket streaming, audio processing, conversation management, and comprehensive API integration. Designed for seamless integration into applications requiring voice transcription, TTS playback, AI responses, and intelligent interruption handling.

## Features

### Core Capabilities
- **WebSocket Client**: Reliable connection with token authentication, automatic reconnection, and comprehensive message handling
- **Audio Processing**: High-performance recording from microphone, TTS audio playback with intelligent queueing, and real-time amplitude detection
- **Conversation Management**: Advanced conversation tracking with history, customizable prompts, and automatic interruption based on audio input
- **API Integration**: Full-featured HTTP client for token generation, user management, and conversation operations
- **Error Handling**: Comprehensive error system with detailed logging, stack traces, and retry logic

### Advanced Features
- **Handler System**: Flexible, chainable handlers for messages, connections, errors, and audio data
- **Configuration Management**: Environment-based configuration with validation and default values
- **Audio Utilities**: Built-in audio processing including normalization, RMS calculation, and format conversion
- **Conversation Tracking**: Persistent conversation history with import/export capabilities
- **Graceful Shutdown**: Proper resource cleanup and connection management

## Installation

### System Requirements
- Go 1.24 or higher
- PortAudio system libraries (for audio processing)

### Install System Dependencies

**macOS:**
```bash
brew install portaudio
```

**Ubuntu/Debian:**
```bash
sudo apt-get install libportaudio2 libportaudio-dev
```

**Windows:**
- Download PortAudio from [portaudio.com](http://portaudio.com/)
- Follow the installation instructions for your system

### Install Go Module
```bash
go get github.com/rojolang/vocals-sdk-go
```

## Quick Start

### Basic Client Setup

```go
package main

import (
    "log"
    "time"

    "github.com/rojolang/vocals-sdk-go/pkg/vocals"
)

func main() {
    // Create configuration
    config := vocals.NewVocalsConfig()
    config.WsEndpoint = ptr("ws://localhost:8000/v1/stream/conversation")
    config.UseTokenAuth = true
    config.AutoConnect = true

    audioConfig := vocals.NewAudioConfig()
    audioConfig.SampleRate = 24000
    audioConfig.Channels = 1

    // Create client
    client := vocals.NewVocalsClient(config, audioConfig, ptr("user123"), []string{})

    // Add handlers
    client.AddMessageHandler(vocals.CreateLoggingMessageHandler(true))
    client.AddConnectionHandler(vocals.CreateConnectionStatusHandler(nil))
    client.AddErrorHandler(vocals.CreateErrorLoggingHandler("Client"))

    // Wait for connection
    time.Sleep(2 * time.Second)

    // Start recording
    if err := client.StartRecording(); err != nil {
        log.Fatal(err)
    }

    // Record for 10 seconds
    time.Sleep(10 * time.Second)

    // Cleanup
    client.StopRecording()
    client.Disconnect()
    client.Cleanup()
}

func ptr[T any](v T) *T { return &v }
```

### Conversation Mode

```go
// Create conversation configuration
convConfig := vocals.NewConversationConfig()
convConfig.Prompt = "You are a helpful AI assistant."
convConfig.Language = "en-US"
convConfig.AutoInterrupt = true
convConfig.InterruptThreshold = 0.3

// Create conversation
conv := vocals.NewConversation(
    client.GetWebSocketClient(),
    client.GetAudioProcessor(),
    convConfig,
)

// Send text message
if err := conv.SendText("Hello, how are you?"); err != nil {
    log.Printf("Failed to send text: %v", err)
}

// Export conversation history
if err := conv.ExportHistory("conversation.json"); err != nil {
    log.Printf("Failed to export history: %v", err)
}

// Cleanup
defer conv.Cleanup()
```

### API Usage

```go
// Create API client
apiClient := vocals.NewAPIClient("https://api.vocals.example.com", ptr("your-api-key"))

// Generate WebSocket token
tokenResult := apiClient.GenerateWsToken()
if tokenResult.Success {
    log.Printf("Token: %s", tokenResult.Data.Token)
} else {
    log.Printf("Error: %v", tokenResult.Error)
}

// Create user
userResult := apiClient.AddUser("user@example.com", "password123")
if userResult.Success {
    log.Printf("Created user: %s", userResult.Data.ID)
} else {
    log.Printf("Error: %v", userResult.Error)
}

// Health check
healthResult := apiClient.HealthCheck()
if healthResult.Success {
    log.Println("API is healthy")
} else {
    log.Printf("API health check failed: %v", healthResult.Error)
}
```

### Custom Handlers

```go
// Transcription handler
transcriptionHandler := vocals.CreateTranscriptionHandler(func(text string, isFinal bool) {
    if isFinal {
        log.Printf("Final transcription: %s", text)
    } else {
        log.Printf("Partial transcription: %s", text)
    }
})

// TTS handler
ttsHandler := vocals.CreateTTSHandler(func(segment vocals.TTSAudioSegment) {
    log.Printf("Received TTS segment: %s (%.2fs)", segment.SegmentID, segment.DurationSeconds)
})

// Audio visualizer
audioHandler := vocals.CreateAudioVisualizerHandler(func(amplitude float32) {
    log.Printf("Audio level: %.3f", amplitude)
})

// Add handlers to client
client.AddMessageHandler(transcriptionHandler)
client.AddMessageHandler(ttsHandler)
client.AddAudioDataHandler(audioHandler)
```

## Configuration

### Environment Variables

```bash
# WebSocket Configuration
export VOCALS_WS_ENDPOINT="ws://localhost:8000/v1/stream/conversation"
export VOCALS_USE_TOKEN_AUTH=true
export VOCALS_AUTO_CONNECT=true

# API Configuration
export VOCALS_DEV_API_KEY="vdev_your_api_key_here"
export VOCALS_API_BASE_URL="https://api.vocals.example.com"

# Audio Configuration
export VOCALS_AUDIO_DEVICE_ID=0
export VOCALS_SAMPLE_RATE=24000

# Debug Configuration
export VOCALS_DEBUG_LEVEL=INFO
export VOCALS_DEBUG_WEBSOCKET=true
export VOCALS_DEBUG_AUDIO=true

# Connection Configuration
export VOCALS_MAX_RECONNECT_ATTEMPTS=5
export VOCALS_RECONNECT_DELAY=2.0
export VOCALS_TOKEN_REFRESH_BUFFER=60.0
```

### Configuration Validation

```go
config := vocals.NewVocalsConfig()
if issues := config.Validate(); len(issues) > 0 {
    log.Printf("Configuration issues:")
    for _, issue := range issues {
        log.Printf("  - %s", issue)
    }
}
```

## Audio Processing

### List Audio Devices

```go
devices := vocals.ListAudioDevices()
for _, device := range devices {
    log.Printf("Device %v: %s (%d channels, %.0f Hz)",
        device["id"], device["name"], device["channels"], device["sample_rate"])
}
```

### Audio Utilities

```go
// Load audio file
samples := vocals.LoadAudioFile("audio.wav")

// Normalize audio
normalized := vocals.NormalizeAudio(samples)

// Calculate RMS
rms := vocals.CalculateRMS(samples)

// Apply gain
gained := vocals.ApplyGain(samples, 6.0) // +6dB

// Encode to base64
encoded := vocals.EncodeAudioToBase64(samples)

// Decode from base64
decoded, err := vocals.DecodeAudioFromBase64(encoded)
if err != nil {
    log.Printf("Decode error: %v", err)
}
```

## Running the Example

Build and run the comprehensive example:

```bash
# Build the example
go build -o vocals-demo examples/main.go

# Run with default settings
./vocals-demo

# Run with custom settings
./vocals-demo \
    --ws-endpoint="ws://your-server:8000/v1/stream/conversation" \
    --user-id="your-user-id" \
    --modes="conversation,streaming" \
    --duration=60 \
    --verbose=true
```

### Example Command Line Options

```bash
Usage of ./vocals-demo:
  -api-key string
        API key for HTTP requests
  -auto-connect
        Automatically connect on start (default true)
  -duration int
        Recording duration in seconds (default 30)
  -modes string
        Comma-separated modes (e.g., conversation,streaming)
  -sample-rate int
        Audio sample rate (default 24000)
  -token-auth
        Use token authentication (default true)
  -user-id string
        User ID for token generation
  -verbose
        Enable verbose logging (default true)
  -ws-endpoint string
        WebSocket endpoint (default "ws://localhost:8000/v1/stream/conversation")
```

## Error Handling

### Error Types

```go
// Check error type
if vocals.IsErrorCode(err, vocals.ErrCodeConnectionFailed) {
    log.Println("Connection failed, retrying...")
}

// Check if error is retryable
if vocals.IsRetryableError(err) {
    log.Println("Error is retryable")
}

// Check if error is critical
if vocals.IsCriticalError(err) {
    log.Println("Critical error, stopping...")
}
```

### Error Codes

- `CONNECTION_FAILED`: WebSocket connection issues
- `RECONNECT_FAILED`: Automatic reconnection failed
- `TOKEN_EXPIRED`: Authentication token expired
- `AUDIO_DEVICE_ERROR`: Audio device problems
- `PLAYBACK_ERROR`: Audio playback issues
- `WEBSOCKET_ERROR`: WebSocket communication errors
- `TRANSCRIPTION_FAILED`: Speech recognition errors
- `RESPONSE_FAILED`: AI response generation errors
- `INTERRUPT_FAILED`: Interruption handling errors
- `CONFIG_INVALID`: Configuration validation errors
- `JSON_PARSE_ERROR`: JSON parsing errors
- `TIMEOUT_ERROR`: Operation timeout errors
- `AUTH_FAILED`: Authentication errors

## Architecture

### Core Components

1. **VocalsClient**: Main client orchestrating all components
2. **WebSocketClient**: Handles WebSocket connections and messaging
3. **AudioProcessor**: Manages audio recording, playback, and processing
4. **Conversation**: Manages conversation state and AI interactions
5. **APIClient**: Handles HTTP API operations
6. **TokenManager**: Manages authentication tokens
7. **ErrorHandler**: Comprehensive error handling and logging

### Data Flow

```
Audio Input → AudioProcessor → WebSocketClient → Server
                     ↓
User Interface ← MessageHandlers ← WebSocketClient ← Server Response
                     ↓
Audio Output ← AudioProcessor ← TTS Audio Data
```

## Testing

### Unit Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific test
go test -run TestWebSocketClient ./pkg/vocals
```

### Integration Tests

```bash
# Run integration tests (requires running server)
go test -tags=integration ./...
```

## Performance Considerations

### Optimization Tips

1. **Audio Buffer Size**: Adjust `AudioConfig.BufferSize` based on latency requirements
2. **Reconnection Settings**: Tune `MaxReconnectAttempts` and `ReconnectDelay` for your network
3. **Handler Concurrency**: Use goroutines in handlers for non-blocking processing
4. **Memory Management**: Call `Cleanup()` methods to prevent memory leaks

### Resource Management

```go
// Always clean up resources
defer client.Cleanup()
defer conv.Cleanup()

// Monitor connection state
client.AddConnectionHandler(func(state vocals.ConnectionState) {
    if state == vocals.Connected {
        log.Println("Connected - start processing")
    } else if state == vocals.Disconnected {
        log.Println("Disconnected - pause processing")
    }
})
```

## Production Deployment

### Security Considerations

1. **Use WSS**: Always use secure WebSocket connections in production
2. **Token Management**: Implement secure token storage and rotation
3. **API Keys**: Store API keys securely using environment variables or secret management
4. **Input Validation**: Validate all user inputs and configuration values

### Monitoring and Logging

```go
// Add metrics handler
metricsHandler := vocals.CreateMetricsHandler(func(msgType string, metrics map[string]interface{}) {
    // Send metrics to your monitoring system
    log.Printf("Metrics: %s - %+v", msgType, metrics)
})
client.AddMessageHandler(metricsHandler)

// Add debug handler
debugHandler := vocals.CreateDebugHandler("Production")
client.AddMessageHandler(debugHandler)
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

- **GitHub Issues**: [Report bugs or request features](https://github.com/rojolang/vocals-sdk-go/issues)
- **Documentation**: [Full API documentation](https://pkg.go.dev/github.com/rojolang/vocals-sdk-go)
- **Examples**: See the `examples/` directory for more use cases

## Changelog

### v1.0.0
- Initial release with complete Go implementation
- WebSocket client with reconnection
- Audio processing with PortAudio
- Conversation management
- Comprehensive API client
- Full error handling and logging
- Production-ready examples and documentation