package vocals

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

type ConversationConfig struct {
	Prompt             string
	MaxHistory         int
	AutoInterrupt      bool
	InterruptThreshold float64 // e.g., amplitude threshold for auto-interrupt
	Language           string  // e.g., "en-US"
	ResponseTimeout    time.Duration
	MaxTextLength      int
}

func NewConversationConfig() *ConversationConfig {
	return &ConversationConfig{
		Prompt:             "You are a helpful assistant.",
		MaxHistory:         20,
		AutoInterrupt:      true,
		InterruptThreshold: 0.5,
		Language:           "en-US",
		ResponseTimeout:    30 * time.Second,
		MaxTextLength:      1000,
	}
}

type Conversation struct {
	config         *ConversationConfig
	history        []map[string]string
	currentText    string
	tracker        *ConversationTracker
	wsClient       *WebSocketClient
	audioProcessor *AudioProcessor
	ctx            context.Context
	cancel         context.CancelFunc
	mu             sync.Mutex
	interruptTimer *time.Timer
	responseChan   chan string
}

func NewConversation(wsClient *WebSocketClient, audioProcessor *AudioProcessor, config *ConversationConfig) *Conversation {
	if config == nil {
		config = NewConversationConfig()
	}
	ctx, cancel := context.WithCancel(context.Background())
	conv := &Conversation{
		config:         config,
		history:        make([]map[string]string, 0),
		tracker:        NewConversationTracker(),
		wsClient:       wsClient,
		audioProcessor: audioProcessor,
		ctx:            ctx,
		cancel:         cancel,
		responseChan:   make(chan string, 10),
	}

	// Setup handlers
	wsClient.AddMessageHandler(conv.handleIncomingMessage)
	if config.AutoInterrupt {
		audioProcessor.AddAudioDataHandler(conv.handleAudioForInterrupt)
	}

	return conv
}

func (c *Conversation) handleIncomingMessage(msg *WebSocketResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()

	msgType := "unknown"
	if msg.Type != nil {
		msgType = *msg.Type
	}

	switch msgType {
	case "transcription":
		if data, ok := msg.Data.(map[string]interface{}); ok {
			text := getString(data, "text")
			isFinal := getBoolUtil(data, "is_final")
			
			if len(c.currentText)+len(text) > c.config.MaxTextLength {
				text = text[:c.config.MaxTextLength-len(c.currentText)]
			}
			c.currentText += text
			c.tracker.AddTranscription(text)
			
			if isFinal {
				c.addToHistory("user", c.currentText)
				c.currentText = ""
				go c.sendToAI()
			}
		}
	case "partial_transcription":
		if data, ok := msg.Data.(map[string]interface{}); ok {
			text := getString(data, "text")
			log.Printf("Partial transcription: %s", text)
		}
	case "response":
		if data, ok := msg.Data.(map[string]interface{}); ok {
			text := getString(data, "text")
			c.addToHistory("assistant", text)
			c.tracker.AddResponse(text)
			select {
			case c.responseChan <- text:
			default:
			}
		}
	case "interruption":
		log.Println("Interruption detected by server")
		if c.config.AutoInterrupt {
			if err := c.Interrupt(); err != nil {
				log.Printf("Interrupt failed: %v", err)
			}
		}
	case "error":
		if data, ok := msg.Data.(map[string]interface{}); ok {
			message := getString(data, "message")
			code := getString(data, "code")
			err := NewVocalsError(message, code)
			log.Printf("Conversation error: %v", err)
		}
	default:
		log.Printf("Unhandled message type: %s", msgType)
	}
}

func (c *Conversation) handleAudioForInterrupt(data []float32) {
	amplitude := c.audioProcessor.GetCurrentAmplitude()
	if amplitude > float32(c.config.InterruptThreshold) {
		if c.interruptTimer == nil {
			c.interruptTimer = time.AfterFunc(500*time.Millisecond, func() {
				if err := c.Interrupt(); err != nil {
					log.Printf("Auto-interrupt failed: %v", err)
				}
				c.mu.Lock()
				c.interruptTimer = nil
				c.mu.Unlock()
			})
		}
	} else {
		c.mu.Lock()
		if c.interruptTimer != nil {
			c.interruptTimer.Stop()
			c.interruptTimer = nil
		}
		c.mu.Unlock()
	}
}

func (c *Conversation) addToHistory(role, content string) {
	if content == "" {
		return
	}
	c.history = append(c.history, map[string]string{"role": role, "content": content})
	if len(c.history) > c.config.MaxHistory {
		c.history = c.history[len(c.history)-c.config.MaxHistory:]
	}
	contentPreview := content
	if len(content) > 50 {
		contentPreview = content[:50] + "..."
	}
	log.Printf("Added to history: %s - %s (History size: %d)", role, contentPreview, len(c.history))
}

func (c *Conversation) sendToAI() {
	c.mu.Lock()
	historyCopy := append([]map[string]string(nil), c.history...)
	language := c.config.Language
	prompt := c.config.Prompt
	c.mu.Unlock()

	if len(historyCopy) == 0 {
		return
	}

	var fullPrompt string
	if prompt != "" {
		fullPrompt = prompt + "\n"
	}
	for _, msg := range historyCopy {
		fullPrompt += fmt.Sprintf("%s: %s\n", msg["role"], msg["content"])
	}

	wsMsg := &WebSocketMessage{
		Event: "ai_prompt",
		Data: map[string]interface{}{
			"prompt":   fullPrompt,
			"language": language,
		},
	}
	if err := c.wsClient.SendMessage(wsMsg); err != nil {
		log.Printf("Failed to send AI prompt: %v", err)
		return
	}

	// Wait for response with timeout
	select {
	case <-time.After(c.config.ResponseTimeout):
		log.Println("AI response timeout")
	case response := <-c.responseChan:
		responsePreview := response
		if len(response) > 50 {
			responsePreview = response[:50] + "..."
		}
		log.Printf("Received AI response: %s", responsePreview)
	}
}

func (c *Conversation) SendText(text string) error {
	if text == "" {
		return fmt.Errorf("empty text")
	}
	c.mu.Lock()
	c.addToHistory("user", text)
	c.mu.Unlock()
	go c.sendToAI()
	return nil
}

func (c *Conversation) Interrupt() error {
	wsMsg := &WebSocketMessage{
		Event: "interrupt",
		Data: map[string]interface{}{
			"reason": "user_interrupt",
		},
	}
	if err := c.wsClient.SendMessage(wsMsg); err != nil {
		return NewVocalsError(err.Error(), "INTERRUPT_FAILED")
	}
	log.Println("Sent interruption signal")
	c.audioProcessor.FadeOutAudio(500 * time.Millisecond)
	c.mu.Lock()
	c.currentText = ""
	c.mu.Unlock()
	return nil
}

func (c *Conversation) ClearHistory() {
	c.mu.Lock()
	c.history = make([]map[string]string, 0)
	c.currentText = ""
	c.tracker.Clear()
	c.mu.Unlock()
	log.Println("Conversation history cleared")
}

func (c *Conversation) GetHistory() []map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]map[string]string(nil), c.history...)
}

func (c *Conversation) SetPrompt(prompt string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config.Prompt = prompt
	promptPreview := prompt
	if len(prompt) > 50 {
		promptPreview = prompt[:50] + "..."
	}
	log.Printf("Updated prompt to: %s", promptPreview)
}

func (c *Conversation) SetMaxHistory(max int) {
	if max <= 0 {
		max = 1
	}
	c.mu.Lock()
	c.config.MaxHistory = max
	if len(c.history) > max {
		c.history = c.history[len(c.history)-max:]
	}
	c.mu.Unlock()
	log.Printf("Updated max history to: %d", max)
}

func (c *Conversation) SetLanguage(lang string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if lang == "" {
		lang = "en-US"
	}
	c.config.Language = lang
	log.Printf("Updated language to: %s", lang)
}

func (c *Conversation) EnableAutoInterrupt(enable bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config.AutoInterrupt = enable
	log.Printf("Auto-interrupt set to: %t", enable)
}

func (c *Conversation) SetInterruptThreshold(threshold float64) {
	if threshold < 0 || threshold > 1 {
		threshold = 0.5
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config.InterruptThreshold = threshold
	log.Printf("Interrupt threshold set to: %f", threshold)
}

func (c *Conversation) SetResponseTimeout(timeout time.Duration) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config.ResponseTimeout = timeout
	log.Printf("Response timeout set to: %s", timeout)
}

func (c *Conversation) SetMaxTextLength(length int) {
	if length <= 0 {
		length = 1000
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config.MaxTextLength = length
	log.Printf("Max text length set to: %d", length)
}

func (c *Conversation) ExportHistory(filePath string) error {
	c.mu.Lock()
	data, err := json.MarshalIndent(c.history, "", "  ")
	c.mu.Unlock()
	if err != nil {
		return NewVocalsError(err.Error(), "JSON_PARSE_ERROR")
	}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return NewVocalsError(err.Error(), "UNKNOWN_ERROR")
	}
	log.Printf("Exported history to %s", filePath)
	return nil
}

func (c *Conversation) ImportHistory(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return NewVocalsError(err.Error(), "UNKNOWN_ERROR")
	}
	var imported []map[string]string
	if err := json.Unmarshal(data, &imported); err != nil {
		return NewVocalsError(err.Error(), "JSON_PARSE_ERROR")
	}
	c.mu.Lock()
	c.history = imported
	if len(c.history) > c.config.MaxHistory {
		c.history = c.history[len(c.history)-c.config.MaxHistory:]
	}
	c.mu.Unlock()
	log.Printf("Imported history from %s (Size: %d)", filePath, len(imported))
	return nil
}

func (c *Conversation) Cleanup() {
	c.cancel()
	c.ClearHistory()
	c.mu.Lock()
	if c.interruptTimer != nil {
		c.interruptTimer.Stop()
		c.interruptTimer = nil
	}
	close(c.responseChan)
	c.mu.Unlock()
	log.Println("Conversation cleaned up")
}