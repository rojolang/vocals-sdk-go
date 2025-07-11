package vocals

import (
	"bytes"
	"context"
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
	logger             *VocalsLogger
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
				// Validate required fields before processing
				segmentID := getString(data, "segment_id")
				audioData := getString(data, "audio_data")

				if segmentID == "" || audioData == "" {
					log.Printf("Invalid TTS message: missing required fields (segment_id: %s, audio_data: %s)", segmentID, audioData)
					return
				}

				segment := TTSAudioSegment{
					SegmentID:      segmentID,
					SentenceNumber: getInt(data, "sentence_number"),
					AudioData:      audioData,
					SampleRate:     getInt(data, "sample_rate"),
					Text:           getString(data, "text"),
					Format:         getString(data, "format"),
				}
				c.audioProcessor.AddToQueue(segment)
			} else {
				log.Printf("Invalid TTS message format: expected map[string]interface{}, got %T", msg.Data)
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

// EnsureConnected ensures the WebSocket connection is established before proceeding
func (c *VocalsClient) EnsureConnected() error {
	if c.websocketClient.IsConnected() {
		log.Printf("Already connected (state: %v)", c.websocketClient.GetState())
		return nil
	}
	
	log.Printf("Connection state: %v - attempting to connect...", c.websocketClient.GetState())
	
	// Try to connect
	if err := c.Connect(); err != nil {
		return fmt.Errorf("connection failed: %v", err)
	}
	
	// Wait for connection to be established (with timeout)
	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()
	
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-timeout.C:
			return fmt.Errorf("connection timeout after 10 seconds (state: %v)", c.websocketClient.GetState())
		case <-ticker.C:
			if c.websocketClient.IsConnected() {
				log.Printf("Successfully connected (state: %v)", c.websocketClient.GetState())
				
				// Send settings event after connection is established
				if err := c.sendSettingsEvent(); err != nil {
					log.Printf("Failed to send settings event: %v", err)
				}
				
				return nil
			}
			state := c.websocketClient.GetState()
			if state == ErrorState {
				return fmt.Errorf("connection failed - WebSocket in error state")
			}
			log.Printf("Waiting for connection... (state: %v)", state)
		}
	}
}

// sendSettingsEvent sends the audio settings to the server
func (c *VocalsClient) sendSettingsEvent() error {
	settingsMsg := &WebSocketMessage{
		Event: "settings",
		Data: map[string]interface{}{
			"format":     c.audioConfig.Format,
			"sampleRate": c.audioConfig.SampleRate,
			"channels":   c.audioConfig.Channels,
		},
	}
	
	log.Printf("Sending settings event: format=%s, sampleRate=%d, channels=%d", 
		c.audioConfig.Format, c.audioConfig.SampleRate, c.audioConfig.Channels)
	
	return c.websocketClient.SendMessage(settingsMsg)
}

func (c *VocalsClient) StartRecording() error {
	return c.audioProcessor.StartRecording(func(data []float32) {
		// Check if we're connected before trying to send data
		if !c.websocketClient.IsConnected() {
			if c.config.DebugWebsocket {
				log.Printf("Skipping audio data - not connected (state: %v)", c.websocketClient.GetState())
			}
			return
		}

		// Convert PCM float32 to raw bytes
		buf := new(bytes.Buffer)
		for _, sample := range data {
			bits := math.Float32bits(sample)
			binary.Write(buf, binary.LittleEndian, bits)
		}
		
		// Send as JSON with raw bytes - Go's JSON marshaler will auto-base64 it
		format := c.audioConfig.Format
		sampleRate := c.audioConfig.SampleRate
		msg := &WebSocketMessage{
			Event:      "media",
			Data:       buf.Bytes(),  // Raw []byte - JSON marshaler will auto-base64 it
			Format:     &format,
			SampleRate: &sampleRate,
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
	// Ensure we're connected before starting
	if err := c.EnsureConnected(); err != nil {
		return fmt.Errorf("failed to establish connection: %v", err)
	}
	
	if err := c.StartRecording(); err != nil {
		return err
	}
	time.Sleep(time.Duration(duration * float64(time.Second)))
	return c.StopRecording()
}

// StreamMicrophoneWithStats provides enhanced streaming with comprehensive monitoring
func (c *VocalsClient) StreamMicrophoneWithStats(
	duration float64,
	statsCallback StreamStatsCallback,
	audioLevelCallback AudioLevelCallback,
	silenceThreshold float32,
	silenceDetectionCallback SilenceDetectionCallback,
) (*StreamStats, error) {
	// Initialize statistics
	stats := &StreamStats{
		StartTime:          time.Now(),
		MinAmplitude:       float32(math.Inf(1)), // Initialize to positive infinity
		VoiceActivityRatio: 0.0,
	}

	// Setup audio level monitoring
	var totalAmplitude float64
	var silenceStartTime time.Time
	var totalSilenceDuration time.Duration
	var voiceActivityDuration time.Duration
	var mu sync.Mutex

	// Create audio data handler for statistics
	audioHandler := func(data []float32) {
		mu.Lock()
		defer mu.Unlock()

		if len(data) == 0 {
			return
		}

		// Update sample and byte counts
		stats.TotalSamples += int64(len(data))
		stats.TotalBytes += int64(len(data) * 4) // float32 = 4 bytes

		// Calculate amplitude metrics
		var sum float64
		var maxAmp float32
		var minAmp float32 = float32(math.Inf(1))

		for _, sample := range data {
			abs := float32(math.Abs(float64(sample)))
			sum += float64(abs)

			if abs > maxAmp {
				maxAmp = abs
			}
			if abs < minAmp {
				minAmp = abs
			}
		}

		avgAmp := float32(sum / float64(len(data)))
		rms := float32(math.Sqrt(sum / float64(len(data))))

		// Update global statistics
		totalAmplitude += sum
		if maxAmp > stats.MaxAmplitude {
			stats.MaxAmplitude = maxAmp
		}
		if minAmp < stats.MinAmplitude {
			stats.MinAmplitude = minAmp
		}
		stats.RMSAmplitude = rms

		// Voice activity detection
		if avgAmp > silenceThreshold {
			if !silenceStartTime.IsZero() {
				silenceDuration := time.Since(silenceStartTime)
				totalSilenceDuration += silenceDuration
				if silenceDetectionCallback != nil {
					go silenceDetectionCallback(silenceDuration)
				}
				silenceStartTime = time.Time{}
			}
			voiceActivityDuration += time.Duration(float64(len(data)) / float64(c.audioConfig.SampleRate) * float64(time.Second))
		} else {
			if silenceStartTime.IsZero() {
				silenceStartTime = time.Now()
			}
		}

		// Call real-time audio level callback
		if audioLevelCallback != nil {
			go audioLevelCallback(avgAmp, maxAmp)
		}

		// Update and call stats callback
		stats.Duration = time.Since(stats.StartTime)
		if stats.TotalSamples > 0 {
			stats.AverageAmplitude = float32(totalAmplitude / float64(stats.TotalSamples))
		}
		stats.SilenceDuration = totalSilenceDuration
		if stats.Duration > 0 {
			stats.VoiceActivityRatio = float32(voiceActivityDuration.Seconds() / stats.Duration.Seconds())
		}

		if statsCallback != nil {
			go statsCallback(stats)
		}
	}

	// Add the audio handler
	removeHandler := c.AddAudioDataHandler(audioHandler)
	defer removeHandler()

	// Start recording
	if err := c.StartRecording(); err != nil {
		return nil, err
	}

	// Monitor for the specified duration
	time.Sleep(time.Duration(duration * float64(time.Second)))

	// Stop recording
	if err := c.StopRecording(); err != nil {
		return stats, err
	}

	// Finalize statistics
	mu.Lock()
	stats.EndTime = time.Now()
	stats.Duration = stats.EndTime.Sub(stats.StartTime)

	// Handle final silence period
	if !silenceStartTime.IsZero() {
		finalSilenceDuration := time.Since(silenceStartTime)
		stats.SilenceDuration += finalSilenceDuration
	}

	// Calculate final voice activity ratio
	if stats.Duration > 0 {
		stats.VoiceActivityRatio = float32(voiceActivityDuration.Seconds() / stats.Duration.Seconds())
	}
	mu.Unlock()

	return stats, nil
}

// StreamMicrophoneWithBasicStats provides enhanced streaming with basic statistics and logging
func (c *VocalsClient) StreamMicrophoneWithBasicStats(duration float64, silenceThreshold float32, verbose bool) (*StreamStats, error) {
	// Ensure we're connected before starting
	if err := c.EnsureConnected(); err != nil {
		return nil, fmt.Errorf("failed to establish connection: %v", err)
	}
	
	var statsCallback StreamStatsCallback
	var audioLevelCallback AudioLevelCallback
	var silenceDetectionCallback SilenceDetectionCallback

	if verbose {
		statsCallback = func(stats *StreamStats) {
			log.Printf("Stream Stats: Duration=%.2fs, Samples=%d, AvgAmp=%.4f, MaxAmp=%.4f, VoiceActivity=%.2f%%",
				stats.Duration.Seconds(), stats.TotalSamples, stats.AverageAmplitude, stats.MaxAmplitude, stats.VoiceActivityRatio*100)
		}

		audioLevelCallback = func(avgLevel, maxLevel float32) {
			log.Printf("Audio Level: Avg=%.4f, Max=%.4f", avgLevel, maxLevel)
		}

		silenceDetectionCallback = func(duration time.Duration) {
			log.Printf("Silence detected for: %v", duration)
		}
	}

	return c.StreamMicrophoneWithStats(duration, statsCallback, audioLevelCallback, silenceThreshold, silenceDetectionCallback)
}

func (c *VocalsClient) StreamAudioFile(filePath string) error {
	samples := LoadAudioFile(filePath)
	if samples == nil {
		return fmt.Errorf("failed to load audio file")
	}

	chunkSize := c.audioConfig.BufferSize
	for i := 0; i < len(samples); i += chunkSize {
		// Check if we're still connected
		if !c.websocketClient.IsConnected() {
			return fmt.Errorf("connection lost during streaming")
		}

		end := i + chunkSize
		if end > len(samples) {
			end = len(samples)
		}
		chunk := samples[i:end]

		// Convert chunk to raw bytes
		buf := new(bytes.Buffer)
		for _, sample := range chunk {
			bits := math.Float32bits(sample)
			binary.Write(buf, binary.LittleEndian, bits)
		}
		
		// Send as JSON with raw bytes - Go's JSON marshaler will auto-base64 it
		format := c.audioConfig.Format
		sampleRate := c.audioConfig.SampleRate
		msg := &WebSocketMessage{
			Event:      "media",
			Data:       buf.Bytes(),  // Raw []byte - JSON marshaler will auto-base64 it
			Format:     &format,
			SampleRate: &sampleRate,
		}
		err := c.websocketClient.SendMessage(msg)
		if err != nil {
			return fmt.Errorf("failed to send audio data: %v", err)
		}

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

// Helper functions for safe type assertions with error logging
func getString(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
		log.Printf("Type assertion failed for key '%s': expected string, got %T", key, val)
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
		log.Printf("Type assertion failed for key '%s': expected int or float64, got %T", key, val)
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
		log.Printf("Type assertion failed for key '%s': expected float64 or int, got %T", key, val)
	}
	return 0.0
}

// Helper function for safe boolean extraction
func getBool(data map[string]interface{}, key string) bool {
	if val, ok := data[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
		log.Printf("Type assertion failed for key '%s': expected bool, got %T", key, val)
	}
	return false
}

// Helper methods to access internal components
func (c *VocalsClient) GetWebSocketClient() *WebSocketClient {
	return c.websocketClient
}

func (c *VocalsClient) GetAudioProcessor() *AudioProcessor {
	return c.audioProcessor
}
