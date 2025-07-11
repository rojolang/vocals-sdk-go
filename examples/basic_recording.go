package main

import (
	"fmt"
	"log"

	"github.com/rojolang/vocals-sdk-go/pkg/vocals"
)

func main() {
	fmt.Println("Vocals SDK Go - Basic Recording Example")
	
	// Create configuration
	config := vocals.NewVocalsConfig()
	config.AutoConnect = false // Manual connection for this example
	
	audioConfig := vocals.NewAudioConfig()
	audioConfig.SampleRate = 44100
	audioConfig.Channels = 1
	audioConfig.BufferSize = 1024
	
	// Create client
	client := vocals.NewVocalsClient(config, audioConfig, nil, []string{})
	
	// Add basic handlers
	client.AddMessageHandler(vocals.CreateLoggingMessageHandler(true))
	client.AddErrorHandler(vocals.CreateErrorLoggingHandler("BasicRecording"))
	client.AddConnectionHandler(vocals.CreateConnectionStatusHandler(nil))
	
	// Connect to WebSocket (if endpoint is configured)
	if config.WsEndpoint != nil {
		fmt.Println("Connecting to WebSocket...")
		if err := client.Connect(); err != nil {
			log.Printf("Connection failed: %v", err)
		}
	}
	
	// Record audio for 5 seconds
	fmt.Println("Starting 5-second recording...")
	if err := client.StreamMicrophone(5.0); err != nil {
		log.Fatalf("Recording failed: %v", err)
	}
	
	fmt.Println("Recording completed successfully!")
	
	// Cleanup
	client.Cleanup()
}