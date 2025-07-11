package vocals

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/gordonklaus/portaudio"
)

type AudioConfig struct {
	SampleRate int
	Channels   int
	Format     string
	BufferSize int
	DeviceID   *int
}

func NewAudioConfig() *AudioConfig {
	return &AudioConfig{
		SampleRate: 24000,
		Channels:   1,
		Format:     "pcm_f32le",
		BufferSize: 1024,
	}
}

type AudioProcessor struct {
	config            *AudioConfig
	recordingState    RecordingState
	playbackState     PlaybackState
	isRecording       bool
	currentAmplitude  float32
	isPlaying         bool
	audioQueue        []TTSAudioSegment
	currentSegment    *TTSAudioSegment
	audioDataHandlers []AudioDataHandler
	errorHandlers     []ErrorHandler
	autoPlayback      bool
	stream            *portaudio.Stream
	mu                sync.Mutex
}

func NewAudioProcessor(config *AudioConfig) *AudioProcessor {
	portaudio.Initialize()
	return &AudioProcessor{
		config:            config,
		recordingState:    IdleRecording,
		playbackState:     IdlePlayback,
		autoPlayback:      true,
		audioQueue:        make([]TTSAudioSegment, 0),
		audioDataHandlers: []AudioDataHandler{},
		errorHandlers:     []ErrorHandler{},
	}
}

func (ap *AudioProcessor) StartRecording(handler func([]float32)) error {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	if ap.isRecording {
		return fmt.Errorf("already recording")
	}
	ap.recordingState = Recording
	ap.isRecording = true

	var err error
	ap.stream, err = portaudio.OpenDefaultStream(ap.config.Channels, 0, float64(ap.config.SampleRate), ap.config.BufferSize, func(in []float32) {
		ap.currentAmplitude = 0
		for _, v := range in {
			ap.currentAmplitude += float32(math.Abs(float64(v)))
		}
		ap.currentAmplitude /= float32(len(in))

		if handler != nil {
			handler(in)
		}
		for _, h := range ap.audioDataHandlers {
			go h(in) // Non-blocking
		}
	})
	if err != nil {
		ap.recordingState = ErrorRecording
		ap.isRecording = false
		ap.handleError(NewVocalsError(err.Error(), "RECORDING_OPEN_ERROR"))
		return err
	}

	if err := ap.stream.Start(); err != nil {
		ap.recordingState = ErrorRecording
		ap.isRecording = false
		ap.handleError(NewVocalsError(err.Error(), "RECORDING_START_ERROR"))
		return err
	}

	log.Println("Recording started")
	return nil
}

func (ap *AudioProcessor) StopRecording() error {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	if !ap.isRecording {
		return nil
	}

	ap.isRecording = false
	ap.recordingState = IdleRecording

	if ap.stream != nil {
		if err := ap.stream.Stop(); err != nil {
			ap.handleError(NewVocalsError(err.Error(), "RECORDING_STOP_ERROR"))
		}
		if err := ap.stream.Close(); err != nil {
			ap.handleError(NewVocalsError(err.Error(), "RECORDING_CLOSE_ERROR"))
		}
		ap.stream = nil
	}

	log.Println("Recording stopped")
	return nil
}

func (ap *AudioProcessor) AddToQueue(segment TTSAudioSegment) {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	// Deduplicate by SegmentID and SentenceNumber
	for _, s := range ap.audioQueue {
		if s.SegmentID == segment.SegmentID && s.SentenceNumber == segment.SentenceNumber {
			log.Printf("Duplicate audio segment detected: %s-%d, skipping", segment.SegmentID, segment.SentenceNumber)
			return
		}
	}

	ap.audioQueue = append(ap.audioQueue, segment)
	log.Printf("Added audio segment to queue: %s-%d", segment.SegmentID, segment.SentenceNumber)

	if ap.autoPlayback && ap.playbackState != PlayingPlayback {
		go ap.playNextSegment()
	}
}

func (ap *AudioProcessor) playNextSegment() {
	ap.mu.Lock()
	if len(ap.audioQueue) == 0 || ap.playbackState == PlayingPlayback {
		ap.mu.Unlock()
		return
	}
	segment := ap.audioQueue[0]
	ap.audioQueue = ap.audioQueue[1:]
	ap.currentSegment = &segment
	ap.playbackState = PlayingPlayback
	ap.mu.Unlock()

	log.Printf("Playing audio segment: %s-%d", segment.SegmentID, segment.SentenceNumber)

	audioData, err := base64.StdEncoding.DecodeString(segment.AudioData)
	if err != nil {
		ap.handleError(NewVocalsError(fmt.Sprintf("Failed to decode audio data: %v", err), "AUDIO_DECODE_ERROR"))
		ap.mu.Lock()
		ap.playbackState = ErrorPlayback
		ap.currentSegment = nil
		ap.mu.Unlock()
		go ap.playNextSegment() // Try next
		return
	}

	// Convert to float32 if needed (assuming input is pcm_f32le little-endian)
	samples := make([]float32, len(audioData)/4)
	for i := 0; i < len(samples); i++ {
		bits := binary.LittleEndian.Uint32(audioData[i*4 : (i+1)*4])
		samples[i] = math.Float32frombits(bits)
	}

	// Create a channel to signal when playback is complete
	done := make(chan bool, 1)
	sampleIndex := 0
	var mu sync.Mutex
	
	// Open playback stream with callback that feeds audio data
	stream, err := portaudio.OpenDefaultStream(0, ap.config.Channels, float64(segment.SampleRate), ap.config.BufferSize, func(out []float32) {
		mu.Lock()
		defer mu.Unlock()
		
		// Copy samples to output buffer
		for i := range out {
			if sampleIndex < len(samples) {
				out[i] = samples[sampleIndex]
				sampleIndex++
			} else {
				out[i] = 0.0 // Silence when no more samples
			}
		}
		
		// Signal completion when all samples have been output
		if sampleIndex >= len(samples) {
			select {
			case done <- true:
			default:
			}
		}
	})
	if err != nil {
		ap.handleError(NewVocalsError(fmt.Sprintf("Failed to open playback stream: %v", err), "PLAYBACK_OPEN_ERROR"))
		ap.mu.Lock()
		ap.playbackState = ErrorPlayback
		ap.currentSegment = nil
		ap.mu.Unlock()
		go ap.playNextSegment()
		return
	}
	defer stream.Close()

	if err := stream.Start(); err != nil {
		ap.handleError(NewVocalsError(fmt.Sprintf("Failed to start playback stream: %v", err), "PLAYBACK_START_ERROR"))
		ap.mu.Lock()
		ap.playbackState = ErrorPlayback
		ap.currentSegment = nil
		ap.mu.Unlock()
		go ap.playNextSegment()
		return
	}

	log.Printf("Playing audio segment with %d samples", len(samples))
	
	// Wait for playback to complete or timeout
	select {
	case <-done:
		log.Printf("Audio playback completed")
	case <-time.After(time.Duration(float64(len(samples))/float64(segment.SampleRate)*1.5) * time.Second):
		log.Printf("Audio playback timeout")
	}

	if err := stream.Stop(); err != nil {
		ap.handleError(NewVocalsError(fmt.Sprintf("Failed to stop playback stream: %v", err), "PLAYBACK_STOP_ERROR"))
	}

	ap.mu.Lock()
	ap.currentSegment = nil
	if len(ap.audioQueue) == 0 {
		ap.playbackState = IdlePlayback
	} else {
		ap.playbackState = QueuedPlayback
	}
	ap.mu.Unlock()

	go ap.playNextSegment() // Play next if queued
}

func (ap *AudioProcessor) ClearQueue() {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	ap.audioQueue = make([]TTSAudioSegment, 0)
	ap.currentSegment = nil
	ap.playbackState = IdlePlayback
	log.Println("Audio queue cleared")
}

func (ap *AudioProcessor) PausePlayback() error {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	if ap.playbackState != PlayingPlayback {
		return fmt.Errorf("not currently playing")
	}

	// Portaudio doesn't support pause directly; we'd need to stop and resume with position tracking.
	// For simplicity, stop and mark as paused.
	ap.playbackState = PausedPlayback
	// Implement actual pause logic if needed, e.g., close stream and save position.
	log.Println("Playback paused (simulated)")
	return nil
}

func (ap *AudioProcessor) ResumePlayback() error {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	if ap.playbackState != PausedPlayback {
		return fmt.Errorf("not paused")
	}

	ap.playbackState = PlayingPlayback
	// Resume from saved position if implemented.
	go ap.playNextSegment()
	log.Println("Playback resumed")
	return nil
}

func (ap *AudioProcessor) StopPlayback() error {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	if ap.playbackState == IdlePlayback {
		return nil
	}

	// Close stream if open, clear current.
	ap.currentSegment = nil
	ap.playbackState = IdlePlayback
	log.Println("Playback stopped")
	return nil
}

func (ap *AudioProcessor) FadeOutAudio(duration time.Duration) {
	// Implement fade out by gradually reducing volume in playback loop.
	// This would require modifying the write loop to apply volume ramp.
	log.Println("Fade out not fully implemented")
}

func (ap *AudioProcessor) handleError(err *VocalsError) {
	log.Printf("Audio error: %s (%s)", err.Message, err.Code)
	for _, handler := range ap.errorHandlers {
		go handler(err)
	}
}

func (ap *AudioProcessor) AddAudioDataHandler(handler AudioDataHandler) func() {
	ap.mu.Lock()
	ap.audioDataHandlers = append(ap.audioDataHandlers, handler)
	ap.mu.Unlock()

	return func() {
		ap.mu.Lock()
		for i, h := range ap.audioDataHandlers {
			if &h == &handler {
				ap.audioDataHandlers = append(ap.audioDataHandlers[:i], ap.audioDataHandlers[i+1:]...)
				break
			}
		}
		ap.mu.Unlock()
	}
}

func (ap *AudioProcessor) AddErrorHandler(handler ErrorHandler) func() {
	ap.mu.Lock()
	ap.errorHandlers = append(ap.errorHandlers, handler)
	ap.mu.Unlock()

	return func() {
		ap.mu.Lock()
		for i, h := range ap.errorHandlers {
			if &h == &handler {
				ap.errorHandlers = append(ap.errorHandlers[:i], ap.errorHandlers[i+1:]...)
				break
			}
		}
		ap.mu.Unlock()
	}
}

func (ap *AudioProcessor) Cleanup() {
	ap.StopRecording()
	ap.StopPlayback()
	ap.ClearQueue()
	portaudio.Terminate()
	log.Println("Audio processor cleaned up")
}

func (ap *AudioProcessor) GetRecordingState() RecordingState {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	return ap.recordingState
}

func (ap *AudioProcessor) GetPlaybackState() PlaybackState {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	return ap.playbackState
}

func (ap *AudioProcessor) IsRecording() bool {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	return ap.isRecording
}

func (ap *AudioProcessor) GetCurrentAmplitude() float32 {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	return ap.currentAmplitude
}

func (ap *AudioProcessor) ProcessQueue(callback func(TTSAudioSegment), consumeAll bool) int {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	processed := 0
	if consumeAll {
		for len(ap.audioQueue) > 0 {
			segment := ap.audioQueue[0]
			ap.audioQueue = ap.audioQueue[1:]
			callback(segment)
			processed++
		}
	} else if len(ap.audioQueue) > 0 {
		segment := ap.audioQueue[0]
		ap.audioQueue = ap.audioQueue[1:]
		callback(segment)
		processed = 1
	}
	return processed
}

func ListAudioDevices() []map[string]interface{} {
	if err := portaudio.Initialize(); err != nil {
		log.Printf("Failed to initialize portaudio: %v", err)
		return []map[string]interface{}{}
	}
	defer portaudio.Terminate()

	var devices []map[string]interface{}
	
	// Get default device info as a fallback
	devices = append(devices, map[string]interface{}{
		"id":          0,
		"name":        "Default",
		"channels":    1,
		"sample_rate": 44100,
	})
	
	return devices
}

func (ap *AudioProcessor) SetDeviceID(deviceID int) error {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	ap.config.DeviceID = &deviceID
	return nil
}

func (ap *AudioProcessor) GetDeviceID() *int {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	return ap.config.DeviceID
}
