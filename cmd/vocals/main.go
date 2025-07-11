package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/rojolang/vocals-sdk-go/pkg/vocals"
	"github.com/joho/godotenv"
)

var (
	verbose   bool
	duration  float64
	apiKey    string
	endpoint  string
	userID    string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "vocals",
		Short: "Vocals SDK Go CLI",
		Long:  "A command-line interface for the Vocals SDK Go library",
	}

	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key for authentication")
	rootCmd.PersistentFlags().StringVar(&endpoint, "endpoint", "", "WebSocket endpoint URL")
	rootCmd.PersistentFlags().StringVar(&userID, "user-id", "", "User ID for the session")

	// Add subcommands
	rootCmd.AddCommand(demoCmd())
	rootCmd.AddCommand(setupCmd())
	rootCmd.AddCommand(devicesCmd())

	if err := rootCmd.Execute(); err != nil {
		vocals.GetGlobalLogger().WithError(err).Fatal("CLI execution failed")
	}
}

func demoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "demo",
		Short: "Run demo commands",
		Long:  "Run various demo commands to test the Vocals SDK functionality",
	}

	cmd.AddCommand(demoRecordCmd())
	cmd.AddCommand(demoPlaybackCmd())
	cmd.AddCommand(demoStatsCmd())

	return cmd
}

func demoRecordCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "record",
		Short: "Demo audio recording",
		Long:  "Record audio from the microphone for a specified duration",
		Run: func(cmd *cobra.Command, args []string) {
			if duration == 0 {
				duration = 5.0 // Default 5 seconds
			}

			config := vocals.NewVocalsConfig()
			audioConfig := vocals.NewAudioConfig()
			
			// Enable WebSocket debugging for better troubleshooting
			config.DebugWebsocket = true
			
			if apiKey != "" {
				// Set API key if provided
				vocals.GetGlobalLogger().WithField("api_key_prefix", apiKey[:min(len(apiKey), 8)]).Info("Using API key")
			}
			
			if endpoint != "" {
				config.WsEndpoint = &endpoint
			}

			var userIDPtr *string
			if userID != "" {
				userIDPtr = &userID
			}

			client := vocals.NewVocalsClient(config, audioConfig, userIDPtr, []string{})
			
			// Add basic handlers
			client.AddMessageHandler(vocals.CreateLoggingMessageHandler(verbose))
			client.AddErrorHandler(vocals.CreateErrorLoggingHandler("Demo"))
			client.AddConnectionHandler(vocals.CreateConnectionStatusHandler(nil))

			fmt.Printf("Recording for %.1f seconds...\n", duration)
			
			if err := client.StreamMicrophone(duration); err != nil {
				vocals.GetGlobalLogger().WithError(err).Fatal("Recording failed")
			}
			
			fmt.Println("Recording completed successfully!")
			client.Cleanup()
		},
	}

	cmd.Flags().Float64VarP(&duration, "duration", "d", 5.0, "Recording duration in seconds")
	return cmd
}


func demoStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Demo audio recording with statistics",
		Long:  "Record audio from the microphone with enhanced statistics and debugging",
		Run: func(cmd *cobra.Command, args []string) {
			if duration == 0 {
				duration = 5.0 // Default 5 seconds
			}

			// Load environment variables
			_ = godotenv.Load()

			config := vocals.NewVocalsConfig()
			audioConfig := vocals.NewAudioConfig()
			
			// Disable verbose WebSocket debugging for cleaner output
			config.DebugWebsocket = false
			
			// Set API key from env if not provided via flag
			if apiKey == "" {
				apiKey = os.Getenv("VOCALS_DEV_API_KEY")
			}
			
			if apiKey != "" {
				// Set API key in the vocals module
				os.Setenv("VOCALS_DEV_API_KEY", apiKey)
				vocals.GetGlobalLogger().WithField("api_key_prefix", apiKey[:min(len(apiKey), 8)]).Info("Using API key")
			}
			
			if endpoint != "" {
				config.WsEndpoint = &endpoint
			}

			var userIDPtr *string
			if userID != "" {
				userIDPtr = &userID
			}

			// Disable verbose WebSocket debugging
			config.DebugWebsocket = false
			config.AutoConnect = false // We'll connect manually to track the process

			client := vocals.NewVocalsClient(config, audioConfig, userIDPtr, []string{})
			
			// Create audio handler for local storage and processing
			audioOutputDir := "./audio_output"
			audioHandler := vocals.NewAudioHandler(audioOutputDir, true, 100) // Save files, buffer last 100 segments
			
			// Set up real-time audio processing and LIVE PLAYBACK
			audioHandler.SetProcessFunc(func(entry vocals.AudioBufferEntry) {
				// Real-time processing - PLAY THE AUDIO LIVE!
				fmt.Printf("\n[AUDIO PLAYBACK] 🔊 Playing segment %s (%.1fs, %d bytes)\n", 
					entry.SegmentID, entry.DurationSeconds, len(entry.AudioData))
				
				// Play audio in background so we don't block processing
				go func() {
					if err := audioHandler.PlayAudioEntry(entry); err != nil {
						fmt.Printf("  ⚠️  Playback error: %v\n", err)
					}
				}()
			})
			
			// Add comprehensive handlers
			client.AddMessageHandler(func(msg *vocals.WebSocketResponse) {
				// Only show meaningful responses, not status messages
				if msg.Type == nil || *msg.Type == "status" {
					return
				}
				
				msgType := *msg.Type
				data, ok := msg.Data.(map[string]interface{})
				if !ok {
					fmt.Printf("\n[%s] %v\n", msgType, msg.Data)
					return
				}
				
				// Handle different message types with clean formatting
				switch msgType {
				case "transcription":
					text := getString(data, "text")
					isPartial := getBool(data, "is_partial")
					segmentID := getString(data, "segment_id")
					
					if isPartial {
						fmt.Printf("\r[TRANSCRIPTION] (partial): %s", text)
					} else {
						fmt.Printf("\n[TRANSCRIPTION] %s (segment: %s)\n", text, segmentID)
					}
					
				case "llm_response_streaming":
					token := getString(data, "token")
					isComplete := getBool(data, "is_complete")
					segmentID := getString(data, "segment_id")
					
					if !isComplete {
						// For streaming, just print the token without newline
						fmt.Printf("%s", token)
					} else {
						// Complete response, add newline and segment info
						fmt.Printf("\n[LLM COMPLETE] (segment: %s)\n", segmentID)
					}
					
				case "tts_audio":
					text := getString(data, "text")
					audioData := getString(data, "audio_data")
					sampleRate := getInt(data, "sample_rate")
					segmentID := getString(data, "segment_id")
					duration := getFloat64(data, "duration_seconds")
					
					audioSize := len(audioData)
					fmt.Printf("\n[TTS AUDIO] \"%s\"\n", text)
					fmt.Printf("  → Audio: %d bytes, %.1fs @ %dHz (segment: %s)\n", 
						audioSize, duration, sampleRate, segmentID)
					
					// Process and save audio locally
					if err := audioHandler.HandleTTSAudio(msg); err != nil {
						fmt.Printf("  ⚠️  Failed to handle audio: %v\n", err)
					}
					
				case "speech_interruption":
					fmt.Printf("\n[INTERRUPTION] Speech detected - stopping current response\n")
					
				default:
					// For any other message types, show a clean summary
					fmt.Printf("\n[%s]\n", msgType)
					for key, value := range data {
						// Special handling for audio data - don't print the full base64
						if key == "audio_data" || key == "data" {
							if str, ok := value.(string); ok && len(str) > 100 {
								fmt.Printf("  %s: [%d bytes]\n", key, len(str))
								continue
							}
						}
						fmt.Printf("  %s: %v\n", key, value)
					}
				}
			})
			
			client.AddConnectionHandler(func(state vocals.ConnectionState) {
				fmt.Printf("[CONNECTION] State changed to: %s\n", state)
			})
			
			client.AddErrorHandler(func(err *vocals.VocalsError) {
				fmt.Printf("[ERROR] %s: %s\n", err.Code, err.Message)
			})

			fmt.Println("=== Starting Enhanced Recording Demo ===")
			fmt.Printf("Duration: %.1f seconds\n", duration)
			fmt.Printf("Sample Rate: %d Hz\n", audioConfig.SampleRate)
			fmt.Printf("Format: %s\n", audioConfig.Format)
			fmt.Printf("Channels: %d\n", audioConfig.Channels)
			fmt.Println("=====================================")
			
			// Use the enhanced stats method
			stats, err := client.StreamMicrophoneWithBasicStats(duration, 0.001, true)
			if err != nil {
				vocals.GetGlobalLogger().WithError(err).Fatal("Recording failed")
			}
			
			// Print final statistics
			fmt.Println("\n=== Final Statistics ===")
			fmt.Printf("Total Duration: %.2fs\n", stats.Duration.Seconds())
			fmt.Printf("Total Samples: %d\n", stats.TotalSamples)
			fmt.Printf("Total Bytes: %d\n", stats.TotalBytes)
			fmt.Printf("Average Amplitude: %.4f\n", stats.AverageAmplitude)
			fmt.Printf("Max Amplitude: %.4f\n", stats.MaxAmplitude)
			fmt.Printf("Min Amplitude: %.4f\n", stats.MinAmplitude)
			fmt.Printf("RMS Amplitude: %.4f\n", stats.RMSAmplitude)
			fmt.Printf("Voice Activity: %.2f%%\n", stats.GetVoiceActivityPercentage())
			fmt.Printf("Silence: %.2f%%\n", stats.GetSilencePercentage())
			fmt.Printf("Quality Score: %.2f\n", stats.GetQualityScore())
			fmt.Printf("Is Healthy: %v\n", stats.IsHealthy())
			fmt.Println("=======================")
			
			// Print audio handler statistics
			audioStats := audioHandler.GetStats()
			fmt.Println("\n=== Audio Storage Stats ===")
			fmt.Printf("Total TTS Segments: %d\n", audioStats.TotalSegments)
			fmt.Printf("Buffered Segments: %d\n", audioStats.BufferedSegments)
			fmt.Printf("Total Audio Bytes: %d\n", audioStats.TotalBytes)
			fmt.Printf("Buffer Duration: %.2fs\n", audioStats.BufferDuration)
			fmt.Printf("Output Directory: %s\n", audioStats.OutputDirectory)
			fmt.Println("==========================")
			
			client.Cleanup()
		},
	}

	cmd.Flags().Float64VarP(&duration, "duration", "d", 5.0, "Recording duration in seconds")
	return cmd
}

func demoPlaybackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "playback [audio-file]",
		Short: "Demo audio playback",
		Long:  "Play back an audio file through the Vocals SDK",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			audioFile := args[0]
			
			config := vocals.NewVocalsConfig()
			audioConfig := vocals.NewAudioConfig()
			
			if apiKey != "" {
				vocals.GetGlobalLogger().WithField("api_key_prefix", apiKey[:min(len(apiKey), 8)]).Info("Using API key")
			}
			
			if endpoint != "" {
				config.WsEndpoint = &endpoint
			}

			var userIDPtr *string
			if userID != "" {
				userIDPtr = &userID
			}

			client := vocals.NewVocalsClient(config, audioConfig, userIDPtr, []string{})
			
			// Add basic handlers
			client.AddMessageHandler(vocals.CreateLoggingMessageHandler(verbose))
			client.AddErrorHandler(vocals.CreateErrorLoggingHandler("Demo"))

			fmt.Printf("Playing back audio file: %s\n", audioFile)
			
			if err := client.StreamAudioFile(audioFile); err != nil {
				vocals.GetGlobalLogger().WithError(err).Fatal("Playback failed")
			}
			
			fmt.Println("Playback completed successfully!")
			client.Cleanup()
		},
	}

	return cmd
}

func setupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Setup and configuration commands",
		Long:  "Commands for setting up and configuring the Vocals SDK",
	}

	cmd.AddCommand(setupTestCmd())
	cmd.AddCommand(setupConfigCmd())

	return cmd
}

func setupTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test system configuration",
		Long:  "Test audio devices and system configuration",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Testing system configuration...")
			
			// Test audio configuration
			audioConfig := vocals.NewAudioConfig()
			fmt.Printf("Audio Config - Sample Rate: %d, Channels: %d, Buffer Size: %d\n",
				audioConfig.SampleRate, audioConfig.Channels, audioConfig.BufferSize)
			
			// Test basic config
			config := vocals.NewVocalsConfig()
			fmt.Printf("Vocals Config - Auto Connect: %v, Max Reconnect: %d\n",
				config.AutoConnect, config.MaxReconnectAttempts)
			
			// Test audio processor initialization
			processor := vocals.NewAudioProcessor(audioConfig)
			if processor != nil {
				fmt.Println("✓ Audio processor initialized successfully")
			} else {
				fmt.Println("✗ Audio processor initialization failed")
			}
			
			fmt.Println("System configuration test completed!")
		},
	}

	return cmd
}

func setupConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show current configuration",
		Long:  "Display current configuration settings",
		Run: func(cmd *cobra.Command, args []string) {
			// Load environment variables
			_ = godotenv.Load()
			
			// Get actual values from environment or flags (flags take precedence)
			actualApiKey := apiKey
			if actualApiKey == "" {
				if result := vocals.GetVocalsApiKey(); result.Success {
					actualApiKey = result.Data
				} else {
					// Try direct environment variable access
					actualApiKey = os.Getenv("VOCALS_DEV_API_KEY")
				}
			}
			
			actualEndpoint := endpoint
			if actualEndpoint == "" {
				actualEndpoint = vocals.GetWsEndpoint()
			}
			
			actualUserID := userID
			if actualUserID == "" {
				actualUserID = os.Getenv("USER_ID")
			}
			
			fmt.Println("Current Configuration:")
			fmt.Printf("API Key: %s\n", maskString(actualApiKey))
			fmt.Printf("Endpoint: %s\n", actualEndpoint)
			fmt.Printf("User ID: %s\n", actualUserID)
			fmt.Printf("Verbose: %v\n", verbose)
			
			// Show default configs
			config := vocals.NewVocalsConfig()
			audioConfig := vocals.NewAudioConfig()
			
			fmt.Println("\nDefault Vocals Config:")
			fmt.Printf("  Auto Connect: %v\n", config.AutoConnect)
			fmt.Printf("  Max Reconnect Attempts: %d\n", config.MaxReconnectAttempts)
			fmt.Printf("  Reconnect Delay: %.1fs\n", config.ReconnectDelay)
			
			fmt.Println("\nDefault Audio Config:")
			fmt.Printf("  Sample Rate: %d Hz\n", audioConfig.SampleRate)
			fmt.Printf("  Channels: %d\n", audioConfig.Channels)
			fmt.Printf("  Buffer Size: %d samples\n", audioConfig.BufferSize)
			fmt.Printf("  Format: %s\n", audioConfig.Format)
		},
	}

	return cmd
}

func devicesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "devices",
		Short: "Audio device management",
		Long:  "Commands for managing and listing audio devices",
	}

	cmd.AddCommand(devicesListCmd())
	cmd.AddCommand(devicesTestCmd())

	return cmd
}

func devicesListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available audio devices",
		Long:  "List all available audio input and output devices",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Available Audio Devices:")
			
			devices, err := vocals.GetAllAudioDevices()
			if err != nil {
				vocals.GetGlobalLogger().WithError(err).Error("Failed to list audio devices")
				fmt.Printf("Error listing devices: %v\n", err)
				return
			}
			
			fmt.Println("\nAll Devices:")
			for _, device := range devices {
				marker := ""
				if device.IsDefault {
					marker = " (Default)"
				}
				
				capabilities := ""
				if device.IsInput && device.IsOutput {
					capabilities = "Input/Output"
				} else if device.IsInput {
					capabilities = "Input"
				} else if device.IsOutput {
					capabilities = "Output"
				}
				
				fmt.Printf("  %d: %s%s - %s (%.0f Hz)\n", 
					device.ID, device.Name, marker, capabilities, device.DefaultSampleRate)
			}
			
			// Show input devices separately
			inputDevices, _ := vocals.GetInputDevices()
			if len(inputDevices) > 0 {
				fmt.Println("\nInput Devices:")
				for _, device := range inputDevices {
					marker := ""
					if device.IsDefault {
						marker = " (Default)"
					}
					fmt.Printf("  %d: %s%s - %d channels\n", 
						device.ID, device.Name, marker, device.MaxInputChannels)
				}
			}
			
			// Show output devices separately
			outputDevices, _ := vocals.GetOutputDevices()
			if len(outputDevices) > 0 {
				fmt.Println("\nOutput Devices:")
				for _, device := range outputDevices {
					marker := ""
					if device.IsDefault {
						marker = " (Default)"
					}
					fmt.Printf("  %d: %s%s - %d channels\n", 
						device.ID, device.Name, marker, device.MaxOutputChannels)
				}
			}
		},
	}

	return cmd
}

func devicesTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test [device-id]",
		Short: "Test a specific audio device",
		Long:  "Test recording from a specific audio device",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			deviceID := 0
			if len(args) > 0 {
				fmt.Sscanf(args[0], "%d", &deviceID)
			}
			
			fmt.Printf("Testing audio device ID: %d\n", deviceID)
			
			// Validate device first
			if err := vocals.ValidateAudioDevice(deviceID, true, 1, 44100); err != nil {
				vocals.GetGlobalLogger().WithError(err).Error("Device validation failed")
				fmt.Printf("Device validation failed: %v\n", err)
				return
			}
			
			// Get device info
			dm := vocals.GetGlobalDeviceManager()
			if err := dm.Initialize(); err != nil {
				vocals.GetGlobalLogger().WithError(err).Error("Failed to initialize device manager")
				fmt.Printf("Failed to initialize device manager: %v\n", err)
				return
			}
			defer dm.Cleanup()
			
			deviceInfo, err := dm.GetDeviceInfo(deviceID)
			if err != nil {
				vocals.GetGlobalLogger().WithError(err).Error("Failed to get device info")
				fmt.Printf("Failed to get device info: %v\n", err)
				return
			}
			
			fmt.Printf("\nDevice Information:\n%s\n", deviceInfo)
			
			// Test the device
			fmt.Println("Starting 3-second device test...")
			if err := dm.TestDevice(deviceID, true, 3.0); err != nil {
				vocals.GetGlobalLogger().WithError(err).Error("Device test failed")
				fmt.Printf("Device test failed: %v\n", err)
				return
			}
			
			fmt.Println("Device test completed successfully!")
		},
	}

	return cmd
}

// Helper function to mask sensitive strings
func maskString(s string) string {
	if s == "" {
		return "<not set>"
	}
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Helper functions for safe type extraction from map[string]interface{}
func getString(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getBool(data map[string]interface{}, key string) bool {
	if val, ok := data[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
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
		if num, ok := val.(float32); ok {
			return float64(num)
		}
	}
	return 0.0
}