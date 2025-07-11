package vocals

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

type VocalsClient struct {
	config             *VocalsConfig
	audioConfig        *AudioConfig
	modes              []string
	websocketClient    *WebSocketClient
	audioProcessor     *AudioProcessor
	messageHandlers    []MessageHandler
	connectionHandlers []ConnectionHandler
	errorHandlers      []ErrorHandler
	audioDataHandlers  []AudioDataHandler
	ctx                context.Context
	cancel             context.CancelFunc
	mu                 sync.Mutex
}

func NewVocalsClient(config *VocalsConfig, audioConfig *AudioConfig, userID *string, modes []string) *VocalsClient {
	if config == nil {
		config = NewVocalsConfig()
	}
	if audioConfig == nil {
		audioConfig = NewAudioConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	wsClient := NewWebSocketClient(config, userID)
	audioProc := NewAudioProcessor(audioConfig)

	client := &VocalsClient{
		config:             config,
		audioConfig:        audioConfig,
		modes:              modes,
		websocketClient:    wsClient,
		audioProcessor:     audioProc,
		messageHandlers:    []MessageHandler{},
		connectionHandlers: []ConnectionHandler{},
		errorHandlers:      []ErrorHandler{},
		audioDataHandlers:  []AudioDataHandler{},
		ctx:                ctx,
		cancel:             cancel,
	}

	client.setupInternalHandlers()

	if len(modes) == 0 {
		client.setupDefaultHandlers()
	}

	if config.AutoConnect {
		go func() {
			if err := client.Connect(); err != nil {
				log.Printf("Auto-connect failed: %v", err)
			}
		}()
	}

	return client
}

func (c *VocalsClient) setupInternalHandlers() {
	c.websocketClient.AddMessageHandler(func(msg *WebSocketResponse) {
		// Internal processing, e.g., add TTS to queue
		if msg.Type != nil && *msg.Type == "tts_audio" {
			if data, ok := msg.Data.(map[string]interface{}); ok {
				segment := TTSAudioSegment{
					SegmentID:      getString(data, "segment_id"),
					SentenceNumber: getInt(data, "sentence_number"),
					AudioData:      getString(data, "audio_data"),
					SampleRate:     getInt(data, "sample_rate"),
					Text:           getString(data, "text"),
					Format:         getString(data, "format"),
				}
				c.audioProcessor.AddToQueue(segment)
			}
		}
		// Handle other types internally
	})

	c.audioProcessor.AddErrorHandler(func(err *VocalsError) {
		for _, h := range c.errorHandlers {
			go h(err)
		}
	})
}

func (c *VocalsClient) setupDefaultHandlers() {
	// Add default message handler
	c.AddMessageHandler(CreateDefaultMessageHandler(true))
}

func (c *VocalsClient) Connect() error {
	return c.websocketClient.Connect()
}

func (c *VocalsClient) Disconnect() {
	c.websocketClient.Disconnect()
}

func (c *VocalsClient) StartRecording() error {
	return c.audioProcessor.StartRecording(func(data []float32) {
		// Send to websocket as base64
		buf := new(bytes.Buffer)
		for _, sample := range data {
			bits := math.Float32bits(sample)
			binary.Write(buf, binary.LittleEndian, bits)
		}
		encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
		msg := &WebSocketMessage{
			Event: "audio_data",
			Data: map[string]interface{}{
				"audio":       encoded,
				"sample_rate": c.audioConfig.SampleRate,
				"format":      c.audioConfig.Format,
			},
		}
		err := c.websocketClient.SendMessage(msg)
		if err != nil {
			log.Printf("Error sending audio data: %v", err)
			return
		}
	})
}

func (c *VocalsClient) StopRecording() error {
	return c.audioProcessor.StopRecording()
}

func (c *VocalsClient) StreamMicrophone(duration float64) error {
	if err := c.StartRecording(); err != nil {
		return err
	}
	time.Sleep(time.Duration(duration * float64(time.Second)))
	return c.StopRecording()
}

func (c *VocalsClient) StreamAudioFile(filePath string) error {
	samples := LoadAudioFile(filePath)
	if samples == nil {
		return fmt.Errorf("failed to load audio file")
	}

	chunkSize := c.audioConfig.BufferSize
	for i := 0; i < len(samples); i += chunkSize {
		end := i + chunkSize
		if end > len(samples) {
			end = len(samples)
		}
		chunk := samples[i:end]

		// Encode and send
		buf := new(bytes.Buffer)
		for _, sample := range chunk {
			bits := math.Float32bits(sample)
			binary.Write(buf, binary.LittleEndian, bits)
		}
		encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
		msg := &WebSocketMessage{
			Event: "audio_data",
			Data: map[string]interface{}{
				"audio":       encoded,
				"sample_rate": c.audioConfig.SampleRate,
				"format":      c.audioConfig.Format,
			},
		}
		c.websocketClient.SendMessage(msg)

		// Sleep to simulate real-time playback
		sleepDuration := time.Duration(float64(len(chunk))/float64(c.audioConfig.SampleRate)*1000) * time.Millisecond
		time.Sleep(sleepDuration)
	}
	return nil
}

func (c *VocalsClient) AddMessageHandler(handler MessageHandler) func() {
	c.mu.Lock()
	c.messageHandlers = append(c.messageHandlers, handler)
	c.mu.Unlock()
	return c.websocketClient.AddMessageHandler(handler)
}

func (c *VocalsClient) AddConnectionHandler(handler ConnectionHandler) func() {
	c.mu.Lock()
	c.connectionHandlers = append(c.connectionHandlers, handler)
	c.mu.Unlock()
	return c.websocketClient.AddConnectionHandler(handler)
}

func (c *VocalsClient) AddErrorHandler(handler ErrorHandler) func() {
	c.mu.Lock()
	c.errorHandlers = append(c.errorHandlers, handler)
	c.mu.Unlock()
	return c.websocketClient.AddErrorHandler(handler)
}

func (c *VocalsClient) AddAudioDataHandler(handler AudioDataHandler) func() {
	c.mu.Lock()
	c.audioDataHandlers = append(c.audioDataHandlers, handler)
	c.mu.Unlock()
	return c.audioProcessor.AddAudioDataHandler(handler)
}

func (c *VocalsClient) ConnectionState() ConnectionState {
	return c.websocketClient.GetState()
}

func (c *VocalsClient) Cleanup() {
	c.cancel()
	c.audioProcessor.Cleanup()
	c.websocketClient.Disconnect()
	log.Println("Vocals client cleaned up")
}

func (c *VocalsClient) SendMessage(msg *WebSocketMessage) error {
	return c.websocketClient.SendMessage(msg)
}

func (c *VocalsClient) GetRecordingState() RecordingState {
	return c.audioProcessor.GetRecordingState()
}

func (c *VocalsClient) GetPlaybackState() PlaybackState {
	return c.audioProcessor.GetPlaybackState()
}

func (c *VocalsClient) IsRecording() bool {
	return c.audioProcessor.IsRecording()
}

func (c *VocalsClient) GetCurrentAmplitude() float32 {
	return c.audioProcessor.GetCurrentAmplitude()
}

func (c *VocalsClient) ClearAudioQueue() {
	c.audioProcessor.ClearQueue()
}

func (c *VocalsClient) PausePlayback() error {
	return c.audioProcessor.PausePlayback()
}

func (c *VocalsClient) ResumePlayback() error {
	return c.audioProcessor.ResumePlayback()
}

func (c *VocalsClient) StopPlayback() error {
	return c.audioProcessor.StopPlayback()
}

// Helper functions for type assertions
func getString(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getInt(data map[string]interface{}, key string) int {
	if val, ok := data[key]; ok {
		if num, ok := val.(float64); ok {
			return int(num)
		}
		if num, ok := val.(int); ok {
			return num
		}
	}
	return 0
}

func getFloat64(data map[string]interface{}, key string) float64 {
	if val, ok := data[key]; ok {
		if num, ok := val.(float64); ok {
			return num
		}
		if num, ok := val.(int); ok {
			return float64(num)
		}
	}
	return 0.0
}

// Helper methods to access internal components
func (c *VocalsClient) GetWebSocketClient() *WebSocketClient {
	return c.websocketClient
}

func (c *VocalsClient) GetAudioProcessor() *AudioProcessor {
	return c.audioProcessor
}
