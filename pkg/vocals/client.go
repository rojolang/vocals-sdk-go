package vocals

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
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
		logger:             GetGlobalLogger().WithComponent("VocalsClient"),
	}

	client.setupInternalHandlers()

	if len(modes) == 0 {
		client.setupDefaultHandlers()
	}

	if config.AutoConnect {
		go func() {
			if err := client.Connect(); err != nil {
				client.logger.WithError(err).Error("Auto-connect failed")
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
					c.logger.WithFields(map[string]interface{}{
						"segment_id": segmentID,
						"audio_data_empty": audioData == "",
					}).Error("Invalid TTS message: missing required fields")
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
				c.logger.WithField("data_type", fmt.Sprintf("%T", msg.Data)).Error("Invalid TTS message format: expected map[string]interface{}")
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
			c.logger.WithError(err).Error("Error sending audio data")
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
	var statsCallback StreamStatsCallback
	var audioLevelCallback AudioLevelCallback
	var silenceDetectionCallback SilenceDetectionCallback

	if verbose {
		statsCallback = func(stats *StreamStats) {
			c.logger.WithFields(map[string]interface{}{
				"duration": stats.Duration.Seconds(),
				"samples": stats.TotalSamples,
				"avg_amplitude": stats.AverageAmplitude,
				"max_amplitude": stats.MaxAmplitude,
				"voice_activity": stats.VoiceActivityRatio * 100,
			}).Info("Stream statistics update")
		}

		audioLevelCallback = func(avgLevel, maxLevel float32) {
			c.logger.WithFields(map[string]interface{}{
				"avg_level": avgLevel,
				"max_level": maxLevel,
			}).Debug("Audio level update")
		}

		silenceDetectionCallback = func(duration time.Duration) {
			c.logger.WithField("silence_duration", duration).Debug("Silence detected")
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
	c.logger.Info("Vocals client cleaned up")
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
		GetGlobalLogger().WithFields(map[string]interface{}{
			"key": key,
			"type": fmt.Sprintf("%T", val),
		}).Warn("Type assertion failed for key: expected string")
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
		GetGlobalLogger().WithFields(map[string]interface{}{
			"key": key,
			"type": fmt.Sprintf("%T", val),
		}).Warn("Type assertion failed for key: expected int or float64")
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
		GetGlobalLogger().WithFields(map[string]interface{}{
			"key": key,
			"type": fmt.Sprintf("%T", val),
		}).Warn("Type assertion failed for key: expected float64 or int")
	}
	return 0.0
}

// Helper function for safe boolean extraction
func getBool(data map[string]interface{}, key string) bool {
	if val, ok := data[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
		GetGlobalLogger().WithFields(map[string]interface{}{
			"key": key,
			"type": fmt.Sprintf("%T", val),
		}).Warn("Type assertion failed for key: expected bool")
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
