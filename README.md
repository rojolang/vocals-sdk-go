# Vocals SDK Go

A comprehensive Go SDK for real-time audio processing and WebSocket communication with the Vocals API.

## Features

- **Real-time Audio Processing**: Record, process, and stream audio with PortAudio integration
- **WebSocket Client**: Robust WebSocket client with auto-reconnection and error handling
- **Structured Logging**: Built-in structured logging with Zerolog
- **Audio Device Management**: Complete audio device listing, validation, and testing
- **Statistics & Monitoring**: Comprehensive audio streaming statistics and monitoring
- **CLI Tool**: Command-line interface for testing and development
- **Type Safety**: Comprehensive type checking and error handling

## Installation

```bash
go get github.com/rojolang/vocals-sdk-go
```

## Quick Start

### Basic Usage

```go
package main

import (
    "github.com/rojolang/vocals-sdk-go/pkg/vocals"
    "time"
)

func main() {
    // Create configuration
    config := vocals.NewVocalsConfig()
    audioConfig := vocals.NewAudioConfig()
    
    // Create client
    client := vocals.NewVocalsClient(config, audioConfig, nil, []string{})
    
    // Add handlers
    client.AddMessageHandler(vocals.CreateLoggingMessageHandler(true))
    client.AddErrorHandler(vocals.CreateErrorLoggingHandler("Main"))
    
    // Stream microphone for 5 seconds
    if err := client.StreamMicrophone(5.0); err != nil {
        panic(err)
    }
    
    // Cleanup
    client.Cleanup()
}
```

### Advanced Usage with Statistics

```go
package main

import (
    "fmt"
    "github.com/rojolang/vocals-sdk-go/pkg/vocals"
)

func main() {
    config := vocals.NewVocalsConfig()
    audioConfig := vocals.NewAudioConfig()
    client := vocals.NewVocalsClient(config, audioConfig, nil, []string{})
    
    // Stream with statistics
    stats, err := client.StreamMicrophoneWithBasicStats(10.0, 0.01, true)
    if err != nil {
        panic(err)
    }
    
    // Display statistics
    fmt.Printf("Duration: %v\n", stats.Duration)
    fmt.Printf("Total Samples: %d\n", stats.TotalSamples)
    fmt.Printf("Voice Activity: %.1f%%\n", stats.GetVoiceActivityPercentage())
    fmt.Printf("Quality Score: %.2f\n", stats.GetQualityScore())
    
    client.Cleanup()
}
```

## Configuration

### VocalsConfig

```go
config := vocals.NewVocalsConfig()
config.AutoConnect = true
config.MaxReconnectAttempts = 5
config.ReconnectDelay = 2.0
config.DebugWebsocket = true
```

### AudioConfig

```go
audioConfig := vocals.NewAudioConfig()
audioConfig.SampleRate = 44100
audioConfig.Channels = 2
audioConfig.BufferSize = 2048
audioConfig.Format = "pcm_f32le"
```

## Audio Device Management

### List Audio Devices

```go
devices, err := vocals.GetAllAudioDevices()
if err != nil {
    panic(err)
}

for _, device := range devices {
    fmt.Printf("Device: %s (ID: %d)\n", device.Name, device.ID)
    fmt.Printf("  Input Channels: %d\n", device.MaxInputChannels)
    fmt.Printf("  Output Channels: %d\n", device.MaxOutputChannels)
    fmt.Printf("  Sample Rate: %.0f Hz\n", device.DefaultSampleRate)
}
```

### Validate Device

```go
err := vocals.ValidateAudioDevice(deviceID, true, 1, 44100)
if err != nil {
    fmt.Printf("Device validation failed: %v\n", err)
}
```

## Message Handlers

### Built-in Handlers

```go
client := vocals.NewVocalsClient(config, audioConfig, nil, []string{})

// Logging handler
client.AddMessageHandler(vocals.CreateLoggingMessageHandler(true))

// Transcription handler
client.AddMessageHandler(vocals.CreateTranscriptionHandler(func(text string, isFinal bool) {
    fmt.Printf("Transcription: %s (final: %v)\n", text, isFinal)
}))

// TTS handler
client.AddMessageHandler(vocals.CreateTTSHandler(func(segment vocals.TTSAudioSegment) {
    fmt.Printf("TTS Audio: %s\n", segment.Text)
}))

// Error handler
client.AddErrorHandler(vocals.CreateErrorLoggingHandler("MyApp"))
```

### Custom Handlers

```go
client.AddMessageHandler(func(msg *vocals.WebSocketResponse) {
    if msg.Type != nil && *msg.Type == "custom_event" {
        fmt.Printf("Custom event received: %+v\n", msg.Data)
    }
})
```

## Structured Logging

### Global Logger

```go
vocals.Info("Application started")
vocals.Error("Something went wrong")
vocals.LogAudioEvent("recording_started", map[string]interface{}{
    "duration": 5.0,
    "sample_rate": 44100,
})
```

### Custom Logger

```go
logConfig := vocals.DefaultLogConfig()
logConfig.Level = vocals.DebugLevel
logConfig.Pretty = true

logger := vocals.NewVocalsLogger(logConfig)
logger.WithComponent("MyComponent").Info("Component initialized")
```

## Error Handling

### VocalsError

```go
err := vocals.NewVocalsError("Connection failed", "CONNECTION_FAILED")
err.AddDetail("endpoint", "ws://localhost:8080")
err.AddDetail("attempts", 3)

// Error includes stack trace and structured details
fmt.Printf("Error: %v\n", err)
```

### Error Checking

```go
if vocals.IsRetryableError(err) {
    // Retry the operation
}

if vocals.IsCriticalError(err) {
    // Handle critical error
}
```

## Statistics and Monitoring

### Stream Statistics

```go
stats, err := client.StreamMicrophoneWithStats(
    10.0,                    // duration
    func(stats *vocals.StreamStats) {
        fmt.Printf("Real-time stats: %+v\n", stats)
    },
    func(avgLevel, maxLevel float32) {
        fmt.Printf("Audio levels: avg=%.3f, max=%.3f\n", avgLevel, maxLevel)
    },
    0.01,                    // silence threshold
    func(duration time.Duration) {
        fmt.Printf("Silence detected: %v\n", duration)
    },
)
```

### Statistics Methods

```go
fmt.Printf("Sample Rate: %.0f Hz\n", stats.GetSampleRate())
fmt.Printf("Bytes/Second: %.0f\n", stats.GetBytesPerSecond())
fmt.Printf("Voice Activity: %.1f%%\n", stats.GetVoiceActivityPercentage())
fmt.Printf("Quality Score: %.2f\n", stats.GetQualityScore())
fmt.Printf("Is Healthy: %v\n", stats.IsHealthy())
```

## CLI Tool

### Installation

```bash
go build -o vocals ./cmd/vocals
```

### Usage

```bash
# List available devices
./vocals devices list

# Test a device
./vocals devices test 1

# Demo recording
./vocals demo record --duration 10

# Demo with statistics
./vocals demo stats --duration 15 --verbose

# Show configuration
./vocals setup config

# Test system
./vocals setup test
```

### CLI Options

```bash
# Global flags
--verbose, -v      Enable verbose output
--api-key string   API key for authentication
--endpoint string  WebSocket endpoint URL
--user-id string   User ID for the session
```

## API Reference

### Core Types

- `VocalsClient`: Main client for audio processing and WebSocket communication
- `VocalsConfig`: Configuration for the Vocals client
- `AudioConfig`: Audio-specific configuration
- `StreamStats`: Comprehensive streaming statistics
- `AudioDevice`: Audio device information
- `VocalsError`: Enhanced error with stack traces and details

### Handler Types

- `MessageHandler`: Handles WebSocket messages
- `ErrorHandler`: Handles errors
- `ConnectionHandler`: Handles connection state changes
- `AudioDataHandler`: Handles raw audio data
- `StreamStatsCallback`: Handles streaming statistics updates

### Connection States

- `Disconnected`: Not connected
- `Connecting`: Connection in progress
- `Connected`: Successfully connected
- `Reconnecting`: Attempting to reconnect
- `ErrorState`: Connection error occurred

### Recording/Playback States

- `IdleRecording`/`IdlePlayback`: Not active
- `Recording`/`PlayingPlayback`: Active
- `ProcessingRecording`: Processing audio
- `PausedPlayback`: Paused
- `ErrorRecording`/`ErrorPlayback`: Error occurred

## Examples

See the `examples/` directory for complete examples:

- `basic_recording.go`: Simple audio recording
- `advanced_stats.go`: Recording with statistics
- `custom_handlers.go`: Custom message handlers
- `device_management.go`: Audio device operations
- `error_handling.go`: Error handling patterns

## Dependencies

- `github.com/gordonklaus/portaudio`: Audio I/O
- `github.com/gorilla/websocket`: WebSocket client
- `github.com/rs/zerolog`: Structured logging
- `github.com/spf13/cobra`: CLI framework
- `github.com/golang-jwt/jwt/v4`: JWT handling
- `github.com/joho/godotenv`: Environment variables

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

MIT License - see LICENSE file for details

## Support

For support, please open an issue on GitHub or contact the maintainers.