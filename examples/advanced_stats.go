package main

import (
	"fmt"
	"log"
	"time"

	"github.com/rojolang/vocals-sdk-go/pkg/vocals"
)

func main() {
	fmt.Println("Vocals SDK Go - Advanced Statistics Example")
	
	// Create configuration
	config := vocals.NewVocalsConfig()
	audioConfig := vocals.NewAudioConfig()
	
	// Create client
	client := vocals.NewVocalsClient(config, audioConfig, nil, []string{})
	
	// Add handlers
	client.AddMessageHandler(vocals.CreateLoggingMessageHandler(false))
	client.AddErrorHandler(vocals.CreateErrorLoggingHandler("AdvancedStats"))
	
	// Custom statistics callback
	statsCallback := func(stats *vocals.StreamStats) {
		fmt.Printf("\n=== Real-time Statistics ===\n")
		fmt.Printf("Duration: %v\n", stats.Duration)
		fmt.Printf("Samples: %d\n", stats.TotalSamples)
		fmt.Printf("Sample Rate: %.0f Hz\n", stats.GetSampleRate())
		fmt.Printf("Avg Amplitude: %.4f\n", stats.AverageAmplitude)
		fmt.Printf("Max Amplitude: %.4f\n", stats.MaxAmplitude)
		fmt.Printf("Voice Activity: %.1f%%\n", stats.GetVoiceActivityPercentage())
		fmt.Printf("Quality Score: %.2f\n", stats.GetQualityScore())
		fmt.Printf("Is Healthy: %v\n", stats.IsHealthy())
	}
	
	// Audio level callback
	audioLevelCallback := func(avgLevel, maxLevel float32) {
		// Print audio levels every few callbacks to avoid spam
		if time.Now().UnixNano()%1000000000 < 100000000 { // Roughly every second
			fmt.Printf("Audio Levels - Avg: %.4f, Max: %.4f\n", avgLevel, maxLevel)
		}
	}
	
	// Silence detection callback
	silenceCallback := func(duration time.Duration) {
		fmt.Printf("Silence detected for: %v\n", duration)
	}
	
	// Record with comprehensive statistics
	fmt.Println("Starting 10-second recording with statistics...")
	stats, err := client.StreamMicrophoneWithStats(
		10.0,               // duration
		statsCallback,      // real-time stats
		audioLevelCallback, // audio levels
		0.01,               // silence threshold
		silenceCallback,    // silence detection
	)
	
	if err != nil {
		log.Fatalf("Recording failed: %v", err)
	}
	
	// Final statistics
	fmt.Printf("\n=== Final Statistics ===\n")
	fmt.Printf("Total Duration: %v\n", stats.Duration)
	fmt.Printf("Total Samples: %d\n", stats.TotalSamples)
	fmt.Printf("Total Bytes: %d\n", stats.TotalBytes)
	fmt.Printf("Sample Rate: %.0f Hz\n", stats.GetSampleRate())
	fmt.Printf("Bytes/Second: %.0f\n", stats.GetBytesPerSecond())
	fmt.Printf("Average Amplitude: %.4f\n", stats.AverageAmplitude)
	fmt.Printf("Maximum Amplitude: %.4f\n", stats.MaxAmplitude)
	fmt.Printf("Minimum Amplitude: %.4f\n", stats.MinAmplitude)
	fmt.Printf("RMS Amplitude: %.4f\n", stats.RMSAmplitude)
	fmt.Printf("Voice Activity: %.1f%%\n", stats.GetVoiceActivityPercentage())
	fmt.Printf("Silence Duration: %v\n", stats.SilenceDuration)
	fmt.Printf("Quality Score: %.2f\n", stats.GetQualityScore())
	fmt.Printf("Stream Health: %v\n", stats.IsHealthy())
	
	// Cleanup
	client.Cleanup()
	
	fmt.Println("Advanced statistics example completed!")
}