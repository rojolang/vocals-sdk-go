package vocals

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"io/ioutil"
	"log"
	"math"
	"sync"
	"time"
)

func LoadAudioFile(filePath string) []float32 {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Printf("Error loading audio file %s: %v", filePath, err)
		return nil
	}

	// Simple WAV parser assuming 44-byte header, mono, 32-bit float
	if len(data) < 44 {
		log.Println("Invalid WAV file")
		return nil
	}
	samples := make([]float32, (len(data)-44)/4)
	for i := 0; i < len(samples); i++ {
		offset := 44 + i*4
		bits := binary.LittleEndian.Uint32(data[offset : offset+4])
		samples[i] = math.Float32frombits(bits)
	}
	return samples
}

func CreateDefaultMessageHandler(verbose bool) MessageHandler {
	return func(message *WebSocketResponse) {
		if verbose {
			msgType := "unknown"
			if message.Type != nil {
				msgType = *message.Type
			}
			log.Printf("Default handler received message of type %s: %v", msgType, message.Data)
		}
		
		// Basic logging for different types
		if message.Type != nil {
			switch *message.Type {
			case "transcription":
				if data, ok := message.Data.(map[string]interface{}); ok {
					if text := getString(data, "text"); text != "" {
						log.Printf("Transcription: %s", text)
					}
				}
			case "response":
				if data, ok := message.Data.(map[string]interface{}); ok {
					if text := getString(data, "text"); text != "" {
						log.Printf("AI Response: %s", text)
					}
				}
			case "tts_audio":
				if data, ok := message.Data.(map[string]interface{}); ok {
					segmentID := getString(data, "segment_id")
					sentenceNumber := getInt(data, "sentence_number")
					log.Printf("TTS Audio: segment %s-%d", segmentID, sentenceNumber)
				}
			}
		}
	}
}

func CreateEnhancedMessageHandler() MessageHandler {
	return func(message *WebSocketResponse) {
		// More advanced handling, e.g., with formatting or callbacks
		log.Printf("Enhanced handler: %+v", message)
	}
}

type ConversationTracker struct {
	transcriptions []string
	responses      []string
	mu             sync.Mutex
}

func NewConversationTracker() *ConversationTracker {
	return &ConversationTracker{
		transcriptions: []string{},
		responses:      []string{},
	}
}

func (ct *ConversationTracker) AddTranscription(text string) {
	ct.mu.Lock()
	ct.transcriptions = append(ct.transcriptions, text)
	ct.mu.Unlock()
}

func (ct *ConversationTracker) AddResponse(text string) {
	ct.mu.Lock()
	ct.responses = append(ct.responses, text)
	ct.mu.Unlock()
}

func (ct *ConversationTracker) GetHistory() ([]string, []string) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	// Return copies to avoid race conditions
	transcriptions := make([]string, len(ct.transcriptions))
	responses := make([]string, len(ct.responses))
	copy(transcriptions, ct.transcriptions)
	copy(responses, ct.responses)
	return transcriptions, responses
}

func (ct *ConversationTracker) Clear() {
	ct.mu.Lock()
	ct.transcriptions = []string{}
	ct.responses = []string{}
	ct.mu.Unlock()
}

func (ct *ConversationTracker) GetTranscriptionCount() int {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	return len(ct.transcriptions)
}

func (ct *ConversationTracker) GetResponseCount() int {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	return len(ct.responses)
}

func (ct *ConversationTracker) GetLastTranscription() string {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	if len(ct.transcriptions) > 0 {
		return ct.transcriptions[len(ct.transcriptions)-1]
	}
	return ""
}

func (ct *ConversationTracker) GetLastResponse() string {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	if len(ct.responses) > 0 {
		return ct.responses[len(ct.responses)-1]
	}
	return ""
}

// Audio encoding helpers
func EncodeAudioToBase64(samples []float32) string {
	buf := new(bytes.Buffer)
	for _, sample := range samples {
		bits := math.Float32bits(sample)
		binary.Write(buf, binary.LittleEndian, bits)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func DecodeAudioFromBase64(encoded string) ([]float32, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	
	samples := make([]float32, len(data)/4)
	for i := 0; i < len(samples); i++ {
		bits := binary.LittleEndian.Uint32(data[i*4 : (i+1)*4])
		samples[i] = math.Float32frombits(bits)
	}
	return samples, nil
}

// Token utilities
func IsTokenExpiredUtil(token *WSToken) bool {
	return time.Now().UnixMilli() > token.ExpiresAt
}

func GetTokenTimeLeft(token *WSToken) time.Duration {
	remaining := token.ExpiresAt - time.Now().UnixMilli()
	if remaining <= 0 {
		return 0
	}
	return time.Duration(remaining) * time.Millisecond
}

// Message creation helpers
func CreateAudioMessage(audioData []float32, sampleRate int, format string) *WebSocketMessage {
	// Convert float32 samples to raw bytes
	buf := new(bytes.Buffer)
	for _, sample := range audioData {
		bits := math.Float32bits(sample)
		binary.Write(buf, binary.LittleEndian, bits)
	}
	
	sampleRatePtr := &sampleRate
	formatPtr := &format
	return &WebSocketMessage{
		Event:      "media",
		Data:       buf.Bytes(),  // Raw []byte - JSON marshaler will auto-base64 it
		Format:     formatPtr,
		SampleRate: sampleRatePtr,
	}
}

// CreateRawAudioBytes converts float32 samples to raw PCM bytes
func CreateRawAudioBytes(audioData []float32) []byte {
	buf := new(bytes.Buffer)
	for _, sample := range audioData {
		bits := math.Float32bits(sample)
		binary.Write(buf, binary.LittleEndian, bits)
	}
	return buf.Bytes()
}

func CreateTextMessage(text string) *WebSocketMessage {
	return &WebSocketMessage{
		Event: "text_input",
		Data: map[string]interface{}{
			"text": text,
		},
	}
}

func CreateControlMessage(command string, params map[string]interface{}) *WebSocketMessage {
	data := map[string]interface{}{
		"command": command,
	}
	for k, v := range params {
		data[k] = v
	}
	return &WebSocketMessage{
		Event: "control",
		Data:  data,
	}
}

// Utility functions for message handling
func ExtractMessageType(message *WebSocketResponse) string {
	if message.Type != nil {
		return *message.Type
	}
	return "unknown"
}

func ExtractMessageData(message *WebSocketResponse) map[string]interface{} {
	if data, ok := message.Data.(map[string]interface{}); ok {
		return data
	}
	return make(map[string]interface{})
}

// Audio processing utilities
func NormalizeAudio(samples []float32) []float32 {
	if len(samples) == 0 {
		return samples
	}
	
	// Find max amplitude
	maxAmp := float32(0)
	for _, sample := range samples {
		if abs := math.Abs(float64(sample)); abs > float64(maxAmp) {
			maxAmp = float32(abs)
		}
	}
	
	if maxAmp == 0 {
		return samples
	}
	
	// Normalize to prevent clipping
	scale := float32(0.95) / maxAmp
	normalized := make([]float32, len(samples))
	for i, sample := range samples {
		normalized[i] = sample * scale
	}
	
	return normalized
}

func CalculateRMS(samples []float32) float32 {
	if len(samples) == 0 {
		return 0
	}
	
	sum := float32(0)
	for _, sample := range samples {
		sum += sample * sample
	}
	
	return float32(math.Sqrt(float64(sum / float32(len(samples)))))
}

func ApplyGain(samples []float32, gainDb float32) []float32 {
	if len(samples) == 0 {
		return samples
	}
	
	// Convert dB to linear scale
	gain := float32(math.Pow(10, float64(gainDb)/20))
	
	result := make([]float32, len(samples))
	for i, sample := range samples {
		result[i] = sample * gain
	}
	
	return result
}

// Validation utilities
func ValidateAudioConfig(config *AudioConfig) error {
	if config.SampleRate <= 0 {
		return NewVocalsError("Invalid sample rate", "INVALID_SAMPLE_RATE")
	}
	if config.Channels <= 0 {
		return NewVocalsError("Invalid channel count", "INVALID_CHANNELS")
	}
	if config.BufferSize <= 0 {
		return NewVocalsError("Invalid buffer size", "INVALID_BUFFER_SIZE")
	}
	return nil
}

func ValidateVocalsConfig(config *VocalsConfig) error {
	if config.MaxReconnectAttempts < 0 {
		return NewVocalsError("Invalid max reconnect attempts", "INVALID_RECONNECT_ATTEMPTS")
	}
	if config.ReconnectDelay < 0 {
		return NewVocalsError("Invalid reconnect delay", "INVALID_RECONNECT_DELAY")
	}
	if config.TokenRefreshBuffer < 0 {
		return NewVocalsError("Invalid token refresh buffer", "INVALID_TOKEN_REFRESH_BUFFER")
	}
	return nil
}

// Type conversion utilities (duplicated here for completeness)
func getStringUtil(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getIntUtil(data map[string]interface{}, key string) int {
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

func getFloat64Util(data map[string]interface{}, key string) float64 {
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

func getBoolUtil(data map[string]interface{}, key string) bool {
	if val, ok := data[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}