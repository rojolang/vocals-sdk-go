package vocals

import (
	"log"
	"math"
	"sync"
	"time"
)

// Factory functions for common handlers
func CreateLoggingMessageHandler(verbose bool) MessageHandler {
	return func(msg *WebSocketResponse) {
		msgType := "unknown"
		if msg.Type != nil {
			msgType = *msg.Type
		}

		if verbose {
			log.Printf("Verbose: Received %s - Data: %+v - Timestamp: %s", msgType, msg.Data, time.Now().Format(time.RFC3339))
		} else {
			log.Printf("Received %s at %s", msgType, time.Now().Format(time.RFC3339))
		}
	}
}

func CreateTranscriptionHandler(callback func(string, bool)) MessageHandler {
	return func(msg *WebSocketResponse) {
		if msg.Type == nil {
			return
		}

		if *msg.Type != "transcription" && *msg.Type != "partial_transcription" {
			return
		}

		data, ok := msg.Data.(map[string]interface{})
		if !ok {
			log.Println("Invalid transcription data")
			return
		}

		text := getString(data, "text")
		if text == "" {
			log.Println("Empty transcription text")
			return
		}

		isFinal := false
		if _, exists := data["is_final"]; exists {
			isFinal = getBoolUtil(data, "is_final")
		}

		callback(text, isFinal)
	}
}

func CreateTTSHandler(callback func(TTSAudioSegment)) MessageHandler {
	return func(msg *WebSocketResponse) {
		if msg.Type == nil || *msg.Type != "tts_audio" {
			return
		}

		data, ok := msg.Data.(map[string]interface{})
		if !ok {
			log.Println("Invalid TTS data")
			return
		}

		segmentID := getString(data, "segment_id")
		if segmentID == "" {
			log.Println("Invalid TTS segment ID")
			return
		}

		sentenceNumber := getInt(data, "sentence_number")
		audioData := getString(data, "audio_data")
		if audioData == "" {
			log.Println("Invalid TTS audio data")
			return
		}

		sampleRate := getInt(data, "sample_rate")
		if sampleRate == 0 {
			sampleRate = 24000 // Default
		}

		segment := TTSAudioSegment{
			SegmentID:        segmentID,
			SentenceNumber:   sentenceNumber,
			AudioData:        audioData,
			SampleRate:       sampleRate,
			Text:             getString(data, "text"),
			Format:           getString(data, "format"),
			DurationSeconds:  getFloat64(data, "duration_seconds"),
			GenerationTimeMs: getInt(data, "generation_time_ms"),
		}

		callback(segment)
	}
}

func CreateResponseHandler(callback func(string)) MessageHandler {
	return func(msg *WebSocketResponse) {
		if msg.Type == nil || *msg.Type != "response" {
			return
		}

		data, ok := msg.Data.(map[string]interface{})
		if !ok {
			log.Println("Invalid response data")
			return
		}

		text := getString(data, "text")
		if text != "" {
			callback(text)
		} else {
			log.Println("Empty response text")
		}
	}
}

func CreateInterruptionHandler(callback func()) MessageHandler {
	return func(msg *WebSocketResponse) {
		if msg.Type != nil && *msg.Type == "interruption" {
			callback()
		}
	}
}

func CreateErrorLoggingHandler(prefix string) ErrorHandler {
	return func(err *VocalsError) {
		if err != nil {
			log.Printf("%s Error: %v", prefix, err.Error())
		}
	}
}

func CreateConnectionStatusHandler(callback func(ConnectionState)) ConnectionHandler {
	return func(state ConnectionState) {
		log.Printf("Connection state changed to: %s at %s", state, time.Now().Format(time.RFC3339))
		if callback != nil {
			callback(state)
		}
	}
}

func CreateAudioVisualizerHandler(callback func(float32)) AudioDataHandler {
	return func(data []float32) {
		if len(data) == 0 {
			return
		}

		var sum float64
		for _, v := range data {
			sum += float64(v * v)
		}
		rms := float32(math.Sqrt(sum / float64(len(data))))

		if callback != nil {
			callback(rms)
		} else {
			log.Printf("Audio RMS amplitude: %f", rms)
		}
	}
}

func CreateAudioSilenceDetector(threshold float32, silenceDuration time.Duration, callback func()) AudioDataHandler {
	var mu sync.Mutex
	var silenceStart time.Time

	return func(data []float32) {
		mu.Lock()
		defer mu.Unlock()

		if len(data) == 0 {
			return
		}

		amplitude := float32(0)
		for _, v := range data {
			amplitude += float32(math.Abs(float64(v)))
		}
		amplitude /= float32(len(data))

		if amplitude < threshold {
			if silenceStart.IsZero() {
				silenceStart = time.Now()
			} else if time.Since(silenceStart) >= silenceDuration {
				callback()
				silenceStart = time.Time{}
			}
		} else {
			silenceStart = time.Time{}
		}
	}
}

func CreateAudioLevelMonitor(callback func(float32, float32)) AudioDataHandler {
	var mu sync.Mutex
	var maxLevel float32
	var avgLevel float32

	return func(data []float32) {
		mu.Lock()
		defer mu.Unlock()

		if len(data) == 0 {
			return
		}

		var sum float64
		var max float32

		for _, v := range data {
			abs := float32(math.Abs(float64(v)))
			sum += float64(abs)
			if abs > max {
				max = abs
			}
		}

		avgLevel = float32(sum / float64(len(data)))
		maxLevel = max

		if callback != nil {
			callback(avgLevel, maxLevel)
		}
	}
}

func CreateMessageTypeFilter(messageType string, handler MessageHandler) MessageHandler {
	return func(msg *WebSocketResponse) {
		if msg.Type != nil && *msg.Type == messageType {
			handler(msg)
		}
	}
}

func CreateConditionalHandler(condition func(*WebSocketResponse) bool, handler MessageHandler) MessageHandler {
	return func(msg *WebSocketResponse) {
		if condition(msg) {
			handler(msg)
		}
	}
}

func CreateBufferedHandler(bufferSize int, handler MessageHandler) MessageHandler {
	msgChan := make(chan *WebSocketResponse, bufferSize)

	go func() {
		for msg := range msgChan {
			handler(msg)
		}
	}()

	return func(msg *WebSocketResponse) {
		select {
		case msgChan <- msg:
		default:
			log.Println("Message buffer full, dropping message")
		}
	}
}

// Composability functions
func ChainMessageHandlers(handlers ...MessageHandler) MessageHandler {
	return func(msg *WebSocketResponse) {
		for _, h := range handlers {
			if h != nil {
				go h(msg) // Non-blocking chain
			}
		}
	}
}

func ChainErrorHandlers(handlers ...ErrorHandler) ErrorHandler {
	return func(err *VocalsError) {
		for _, h := range handlers {
			if h != nil {
				go h(err)
			}
		}
	}
}

func ChainConnectionHandlers(handlers ...ConnectionHandler) ConnectionHandler {
	return func(state ConnectionState) {
		for _, h := range handlers {
			if h != nil {
				go h(state)
			}
		}
	}
}

func ChainAudioDataHandlers(handlers ...AudioDataHandler) AudioDataHandler {
	return func(data []float32) {
		for _, h := range handlers {
			if h != nil {
				go h(data)
			}
		}
	}
}

func SequentialMessageHandlers(handlers ...MessageHandler) MessageHandler {
	return func(msg *WebSocketResponse) {
		for _, h := range handlers {
			if h != nil {
				h(msg) // Sequential execution
			}
		}
	}
}

func SequentialErrorHandlers(handlers ...ErrorHandler) ErrorHandler {
	return func(err *VocalsError) {
		for _, h := range handlers {
			if h != nil {
				h(err)
			}
		}
	}
}

// Utility handler for debugging
func CreateDebugHandler(label string) MessageHandler {
	return func(msg *WebSocketResponse) {
		msgType := "unknown"
		if msg.Type != nil {
			msgType = *msg.Type
		}
		log.Printf("[DEBUG-%s] Message: %s, Data: %+v", label, msgType, msg.Data)
	}
}

// Utility handler for metrics
func CreateMetricsHandler(callback func(string, map[string]interface{})) MessageHandler {
	return func(msg *WebSocketResponse) {
		msgType := "unknown"
		if msg.Type != nil {
			msgType = *msg.Type
		}

		metrics := map[string]interface{}{
			"timestamp": time.Now().Unix(),
			"type":      msgType,
		}

		if data, ok := msg.Data.(map[string]interface{}); ok {
			// Add specific metrics based on message type
			switch msgType {
			case "transcription":
				if text := getString(data, "text"); text != "" {
					metrics["text_length"] = len(text)
				}
			case "tts_audio":
				if segmentID := getString(data, "segment_id"); segmentID != "" {
					metrics["segment_id"] = segmentID
				}
				if sampleRate := getInt(data, "sample_rate"); sampleRate > 0 {
					metrics["sample_rate"] = sampleRate
				}
			}
		}

		if callback != nil {
			callback(msgType, metrics)
		}
	}
}
