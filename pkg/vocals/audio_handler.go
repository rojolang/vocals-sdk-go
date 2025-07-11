package vocals

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"
	
	"github.com/gordonklaus/portaudio"
)

// AudioHandler manages local audio storage and processing
type AudioHandler struct {
	outputDir       string
	audioBuffer     []AudioBufferEntry
	bufferMu        sync.RWMutex
	processFunc     AudioProcessFunc
	saveRawAudio    bool
	maxBufferSize   int
	totalAudioBytes int64
	totalSegments   int
}

// AudioBufferEntry represents a single audio segment in memory
type AudioBufferEntry struct {
	SegmentID       string
	Text            string
	AudioData       []byte // Decoded audio bytes
	SampleRate      int
	Format          string
	Timestamp       time.Time
	DurationSeconds float64
}

// AudioProcessFunc is called for real-time audio processing
type AudioProcessFunc func(entry AudioBufferEntry)

// NewAudioHandler creates a new audio handler with local storage
func NewAudioHandler(outputDir string, saveRawAudio bool, maxBufferSize int) *AudioHandler {
	// Create output directory if it doesn't exist
	if outputDir != "" {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			log.Printf("Failed to create audio output directory: %v", err)
		}
	}

	return &AudioHandler{
		outputDir:     outputDir,
		audioBuffer:   make([]AudioBufferEntry, 0),
		saveRawAudio:  saveRawAudio,
		maxBufferSize: maxBufferSize,
	}
}

// SetProcessFunc sets the real-time audio processing function
func (ah *AudioHandler) SetProcessFunc(fn AudioProcessFunc) {
	ah.processFunc = fn
}

// HandleTTSAudio processes incoming TTS audio messages
func (ah *AudioHandler) HandleTTSAudio(msg *WebSocketResponse) error {
	if msg.Type == nil || *msg.Type != "tts_audio" {
		return fmt.Errorf("not a TTS audio message")
	}

	data, ok := msg.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid TTS message format")
	}

	// Extract fields
	segmentID := getString(data, "segment_id")
	text := getString(data, "text")
	audioDataB64 := getString(data, "audio_data")
	sampleRate := getInt(data, "sample_rate")
	format := getString(data, "format")
	duration := getFloat64(data, "duration_seconds")

	if segmentID == "" || audioDataB64 == "" {
		return fmt.Errorf("missing required fields")
	}

	// Decode base64 audio data
	audioBytes, err := base64.StdEncoding.DecodeString(audioDataB64)
	if err != nil {
		return fmt.Errorf("failed to decode audio data: %v", err)
	}

	// Create buffer entry
	entry := AudioBufferEntry{
		SegmentID:       segmentID,
		Text:            text,
		AudioData:       audioBytes,
		SampleRate:      sampleRate,
		Format:          format,
		Timestamp:       time.Now(),
		DurationSeconds: duration,
	}

	// Add to buffer
	ah.addToBuffer(entry)

	// Save to file if enabled
	if ah.saveRawAudio && ah.outputDir != "" {
		if err := ah.saveAudioToFile(entry); err != nil {
			log.Printf("Failed to save audio segment %s: %v", segmentID, err)
		}
	}

	// Process in real-time if function is set
	if ah.processFunc != nil {
		go ah.processFunc(entry)
	}

	// Update stats
	ah.totalAudioBytes += int64(len(audioBytes))
	ah.totalSegments++

	return nil
}

// addToBuffer adds an entry to the circular buffer
func (ah *AudioHandler) addToBuffer(entry AudioBufferEntry) {
	ah.bufferMu.Lock()
	defer ah.bufferMu.Unlock()

	ah.audioBuffer = append(ah.audioBuffer, entry)

	// Maintain max buffer size (circular buffer behavior)
	if ah.maxBufferSize > 0 && len(ah.audioBuffer) > ah.maxBufferSize {
		// Remove oldest entries
		removeCount := len(ah.audioBuffer) - ah.maxBufferSize
		ah.audioBuffer = ah.audioBuffer[removeCount:]
	}
}

// saveAudioToFile saves audio data to a local file
func (ah *AudioHandler) saveAudioToFile(entry AudioBufferEntry) error {
	// Create filename with timestamp and segment ID
	timestamp := entry.Timestamp.Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s.wav", timestamp, entry.SegmentID)
	fullPath := filepath.Join(ah.outputDir, filename)

	// Write raw audio data
	if err := ioutil.WriteFile(fullPath, entry.AudioData, 0644); err != nil {
		return err
	}

	// Also save metadata
	metaFilename := fmt.Sprintf("%s_%s.txt", timestamp, entry.SegmentID)
	metaFullPath := filepath.Join(ah.outputDir, metaFilename)
	metadata := fmt.Sprintf("Segment: %s\nText: %s\nSample Rate: %d\nFormat: %s\nDuration: %.2fs\nTimestamp: %s\n",
		entry.SegmentID, entry.Text, entry.SampleRate, entry.Format, 
		entry.DurationSeconds, entry.Timestamp.Format(time.RFC3339))
	
	if err := ioutil.WriteFile(metaFullPath, []byte(metadata), 0644); err != nil {
		log.Printf("Failed to save metadata: %v", err)
	}

	return nil
}

// GetBuffer returns a copy of the current audio buffer
func (ah *AudioHandler) GetBuffer() []AudioBufferEntry {
	ah.bufferMu.RLock()
	defer ah.bufferMu.RUnlock()

	// Return a copy to avoid race conditions
	buffer := make([]AudioBufferEntry, len(ah.audioBuffer))
	copy(buffer, ah.audioBuffer)
	return buffer
}

// GetBufferSize returns the current buffer size
func (ah *AudioHandler) GetBufferSize() int {
	ah.bufferMu.RLock()
	defer ah.bufferMu.RUnlock()
	return len(ah.audioBuffer)
}

// GetLatestEntry returns the most recent audio entry
func (ah *AudioHandler) GetLatestEntry() *AudioBufferEntry {
	ah.bufferMu.RLock()
	defer ah.bufferMu.RUnlock()
	
	if len(ah.audioBuffer) == 0 {
		return nil
	}
	
	// Return a copy to avoid race conditions
	latest := ah.audioBuffer[len(ah.audioBuffer)-1]
	return &latest
}

// ClearBuffer clears the audio buffer
func (ah *AudioHandler) ClearBuffer() {
	ah.bufferMu.Lock()
	defer ah.bufferMu.Unlock()
	ah.audioBuffer = make([]AudioBufferEntry, 0)
}

// GetStats returns statistics about audio handling
func (ah *AudioHandler) GetStats() AudioHandlerStats {
	ah.bufferMu.RLock()
	defer ah.bufferMu.RUnlock()

	totalDuration := 0.0
	for _, entry := range ah.audioBuffer {
		totalDuration += entry.DurationSeconds
	}

	return AudioHandlerStats{
		TotalSegments:   ah.totalSegments,
		BufferedSegments: len(ah.audioBuffer),
		TotalBytes:      ah.totalAudioBytes,
		BufferDuration:  totalDuration,
		OutputDirectory: ah.outputDir,
	}
}

// AudioHandlerStats contains statistics about audio handling
type AudioHandlerStats struct {
	TotalSegments    int
	BufferedSegments int
	TotalBytes       int64
	BufferDuration   float64
	OutputDirectory  string
}

// ConvertToFloat32Samples converts raw audio bytes to float32 samples
func ConvertToFloat32Samples(audioData []byte, format string) ([]float32, error) {
	if len(audioData) == 0 {
		return nil, fmt.Errorf("empty audio data")
	}

	// For now, assume PCM float32 format (can extend for other formats)
	if format != "pcm_f32le" && format != "" {
		log.Printf("Warning: Unsupported format %s, assuming pcm_f32le", format)
	}

	// Convert bytes to float32 samples
	numSamples := len(audioData) / 4
	samples := make([]float32, numSamples)
	
	for i := 0; i < numSamples; i++ {
		offset := i * 4
		if offset+4 > len(audioData) {
			break
		}
		bits := uint32(audioData[offset]) |
			uint32(audioData[offset+1])<<8 |
			uint32(audioData[offset+2])<<16 |
			uint32(audioData[offset+3])<<24
		samples[i] = float32(bits)
	}

	return samples, nil
}

// MergeAudioBuffers merges multiple audio buffers into one
func MergeAudioBuffers(entries []AudioBufferEntry) ([]byte, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("no entries to merge")
	}

	var merged []byte
	for _, entry := range entries {
		merged = append(merged, entry.AudioData...)
	}

	return merged, nil
}

// PlayAudioEntry plays an audio entry through the speakers
func (ah *AudioHandler) PlayAudioEntry(entry AudioBufferEntry) error {
	// Convert bytes to float32 samples
	samples, err := ConvertBytesToFloat32(entry.AudioData)
	if err != nil {
		return fmt.Errorf("failed to convert audio data: %v", err)
	}

	// Initialize PortAudio if needed
	if err := portaudio.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize audio: %v", err)
	}
	defer portaudio.Terminate()

	// Create output stream
	stream, err := portaudio.OpenDefaultStream(
		0,                    // no input channels
		1,                    // mono output
		float64(entry.SampleRate), // sample rate
		len(samples)/4,       // frames per buffer
		&samples,             // audio data
	)
	if err != nil {
		return fmt.Errorf("failed to open audio stream: %v", err)
	}
	defer stream.Close()

	// Start playback
	if err := stream.Start(); err != nil {
		return fmt.Errorf("failed to start audio stream: %v", err)
	}

	// Wait for playback to complete
	time.Sleep(time.Duration(entry.DurationSeconds * float64(time.Second)))
	
	// Stop the stream
	if err := stream.Stop(); err != nil {
		log.Printf("Warning: failed to stop audio stream: %v", err)
	}

	return nil
}

// ConvertBytesToFloat32 converts raw audio bytes to float32 samples
func ConvertBytesToFloat32(audioData []byte) ([]float32, error) {
	if len(audioData)%4 != 0 {
		return nil, fmt.Errorf("invalid audio data length: %d", len(audioData))
	}

	numSamples := len(audioData) / 4
	samples := make([]float32, numSamples)
	
	for i := 0; i < numSamples; i++ {
		offset := i * 4
		bits := binary.LittleEndian.Uint32(audioData[offset:offset+4])
		samples[i] = math.Float32frombits(bits)
	}

	return samples, nil
}