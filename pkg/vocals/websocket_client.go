package vocals

import (
	"context"

	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type WebSocketClient struct {
	config             *VocalsConfig
	userID             *string
	tokenManager       *TokenManager
	conn               *websocket.Conn
	state              ConnectionState
	messageHandlers    []MessageHandler
	connectionHandlers []ConnectionHandler
	errorHandlers      []ErrorHandler
	reconnectAttempts  int
	shouldReconnect    bool
	ctx                context.Context
	cancel             context.CancelFunc
	mu                 sync.Mutex
}

func NewWebSocketClient(config *VocalsConfig, userID *string) *WebSocketClient {
	ctx, cancel := context.WithCancel(context.Background())

	var tokenManager *TokenManager
	if config.TokenEndpoint != nil {
		tokenManager = NewTokenManager(*config.TokenEndpoint, config.Headers, config.TokenRefreshBuffer)
	}

	return &WebSocketClient{
		config:             config,
		userID:             userID,
		tokenManager:       tokenManager,
		state:              Disconnected,
		messageHandlers:    []MessageHandler{},
		connectionHandlers: []ConnectionHandler{},
		errorHandlers:      []ErrorHandler{},
		shouldReconnect:    true,
		ctx:                ctx,
		cancel:             cancel,
	}
}

func (wsc *WebSocketClient) Connect() error {
	wsc.mu.Lock()
	defer wsc.mu.Unlock()

	if wsc.state == Connected || wsc.state == Connecting {
		return fmt.Errorf("already connected or connecting")
	}

	wsc.setState(Connecting)
	wsc.reconnectAttempts = 0

	return wsc.connectWithRetry()
}

func (wsc *WebSocketClient) connectWithRetry() error {
	for wsc.reconnectAttempts < wsc.config.MaxReconnectAttempts {
		if err := wsc.performConnection(); err != nil {
			wsc.reconnectAttempts++
			if wsc.reconnectAttempts >= wsc.config.MaxReconnectAttempts {
				wsc.setState(ErrorState)
				wsc.handleError(NewVocalsError(fmt.Sprintf("Max reconnect attempts reached: %v", err), "CONNECTION_FAILED"))
				return err
			}

			if wsc.config.DebugWebsocket {
				log.Printf("Connection attempt %d failed, retrying in %.1fs: %v", wsc.reconnectAttempts, wsc.config.ReconnectDelay, err)
			}

			time.Sleep(time.Duration(wsc.config.ReconnectDelay * float64(time.Second)))
			continue
		}

		wsc.setState(Connected)
		wsc.reconnectAttempts = 0
		go wsc.messageLoop()
		return nil
	}

	return fmt.Errorf("failed to connect after %d attempts", wsc.config.MaxReconnectAttempts)
}

func (wsc *WebSocketClient) performConnection() error {
	var token string
	var err error

	if wsc.config.UseTokenAuth {
		if wsc.tokenManager != nil {
			token, err = wsc.tokenManager.GetToken()
			if err != nil {
				return fmt.Errorf("failed to get token: %v", err)
			}
		} else {
			// Generate token from API key
			var tokenResult Result[*WSToken]
			if wsc.userID != nil {
				tokenResult = GenerateWsTokenWithUserId(*wsc.userID)
			} else {
				tokenResult = GenerateWsToken()
			}

			if !tokenResult.Success {
				return fmt.Errorf("failed to generate token: %v", tokenResult.Error.Message)
			}
			token = tokenResult.Data.Token
		}
	}

	header := make(http.Header)
	if token != "" {
		header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range wsc.config.Headers {
		header.Set(k, v)
	}

	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(*wsc.config.WsEndpoint, header)
	if err != nil {
		return err
	}

	wsc.conn = conn
	return nil
}

func (wsc *WebSocketClient) messageLoop() {
	defer func() {
		if wsc.conn != nil {
			wsc.conn.Close()
		}
	}()

	for {
		select {
		case <-wsc.ctx.Done():
			return
		default:
			var message WebSocketResponse
			if err := wsc.conn.ReadJSON(&message); err != nil {
				if wsc.config.DebugWebsocket {
					log.Printf("WebSocket read error: %v", err)
				}

				if wsc.shouldReconnect && wsc.state == Connected {
					wsc.setState(Reconnecting)
					go wsc.handleReconnect()
				}
				return
			}

			if wsc.config.DebugWebsocket {
				log.Printf("Received message: %+v", message)
			}

			wsc.handleMessage(&message)
		}
	}
}

func (wsc *WebSocketClient) handleReconnect() {
	wsc.mu.Lock()
	defer wsc.mu.Unlock()

	if wsc.state != Reconnecting {
		return
	}

	if err := wsc.connectWithRetry(); err != nil {
		wsc.setState(ErrorState)
		wsc.handleError(NewVocalsError(fmt.Sprintf("Reconnection failed: %v", err), "RECONNECTION_FAILED"))
	}
}

func (wsc *WebSocketClient) handleMessage(message *WebSocketResponse) {
	for _, handler := range wsc.messageHandlers {
		go handler(message)
	}
}

func (wsc *WebSocketClient) SendMessage(message *WebSocketMessage) error {
	wsc.mu.Lock()
	defer wsc.mu.Unlock()

	if wsc.state != Connected {
		return fmt.Errorf("not connected")
	}

	if wsc.config.DebugWebsocket {
		log.Printf("Sending message: %+v", message)
	}

	return wsc.conn.WriteJSON(message)
}

func (wsc *WebSocketClient) Disconnect() {
	wsc.mu.Lock()
	defer wsc.mu.Unlock()

	wsc.shouldReconnect = false
	wsc.cancel()

	if wsc.conn != nil {
		wsc.conn.Close()
		wsc.conn = nil
	}

	wsc.setState(Disconnected)
}

func (wsc *WebSocketClient) setState(state ConnectionState) {
	if wsc.state != state {
		wsc.state = state
		for _, handler := range wsc.connectionHandlers {
			go handler(state)
		}
	}
}

func (wsc *WebSocketClient) handleError(err *VocalsError) {
	log.Printf("WebSocket error: %s (%s)", err.Message, err.Code)
	for _, handler := range wsc.errorHandlers {
		go handler(err)
	}
}

func (wsc *WebSocketClient) AddMessageHandler(handler MessageHandler) func() {
	wsc.mu.Lock()
	wsc.messageHandlers = append(wsc.messageHandlers, handler)
	wsc.mu.Unlock()

	return func() {
		wsc.mu.Lock()
		for i, h := range wsc.messageHandlers {
			if &h == &handler {
				wsc.messageHandlers = append(wsc.messageHandlers[:i], wsc.messageHandlers[i+1:]...)
				break
			}
		}
		wsc.mu.Unlock()
	}
}

func (wsc *WebSocketClient) AddConnectionHandler(handler ConnectionHandler) func() {
	wsc.mu.Lock()
	wsc.connectionHandlers = append(wsc.connectionHandlers, handler)
	wsc.mu.Unlock()

	return func() {
		wsc.mu.Lock()
		for i, h := range wsc.connectionHandlers {
			if &h == &handler {
				wsc.connectionHandlers = append(wsc.connectionHandlers[:i], wsc.connectionHandlers[i+1:]...)
				break
			}
		}
		wsc.mu.Unlock()
	}
}

func (wsc *WebSocketClient) AddErrorHandler(handler ErrorHandler) func() {
	wsc.mu.Lock()
	wsc.errorHandlers = append(wsc.errorHandlers, handler)
	wsc.mu.Unlock()

	return func() {
		wsc.mu.Lock()
		for i, h := range wsc.errorHandlers {
			if &h == &handler {
				wsc.errorHandlers = append(wsc.errorHandlers[:i], wsc.errorHandlers[i+1:]...)
				break
			}
		}
		wsc.mu.Unlock()
	}
}

func (wsc *WebSocketClient) GetState() ConnectionState {
	wsc.mu.Lock()
	defer wsc.mu.Unlock()
	return wsc.state
}

func (wsc *WebSocketClient) IsConnected() bool {
	wsc.mu.Lock()
	defer wsc.mu.Unlock()
	return wsc.state == Connected
}
