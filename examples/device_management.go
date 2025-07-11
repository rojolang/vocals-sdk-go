package main

import (
	"fmt"
	"log"

	"github.com/rojolang/vocals-sdk-go/pkg/vocals"
)

func main() {
	fmt.Println("Vocals SDK Go - Device Management Example")
	
	// List all available audio devices
	fmt.Println("\n=== All Audio Devices ===")
	devices, err := vocals.GetAllAudioDevices()
	if err != nil {
		log.Fatalf("Failed to list devices: %v", err)
	}
	
	for _, device := range devices {
		fmt.Printf("Device %d: %s\n", device.ID, device.Name)
		fmt.Printf("  Host API: %s\n", device.HostAPI)
		fmt.Printf("  Input Channels: %d\n", device.MaxInputChannels)
		fmt.Printf("  Output Channels: %d\n", device.MaxOutputChannels)
		fmt.Printf("  Sample Rate: %.0f Hz\n", device.DefaultSampleRate)
		fmt.Printf("  Default: %v\n", device.IsDefault)
		fmt.Printf("  Capabilities: ")
		
		capabilities := []string{}
		if device.IsInput {
			capabilities = append(capabilities, "Input")
		}
		if device.IsOutput {
			capabilities = append(capabilities, "Output")
		}
		if len(capabilities) == 0 {
			fmt.Printf("None")
		} else {
			for i, cap := range capabilities {
				if i > 0 {
					fmt.Printf(", ")
				}
				fmt.Printf("%s", cap)
			}
		}
		fmt.Printf("\n\n")
	}
	
	// List input devices only
	fmt.Println("=== Input Devices ===")
	inputDevices, err := vocals.GetInputDevices()
	if err != nil {
		log.Fatalf("Failed to list input devices: %v", err)
	}
	
	for _, device := range inputDevices {
		marker := ""
		if device.IsDefault {
			marker = " (Default)"
		}
		fmt.Printf("  %d: %s%s - %d channels\n", 
			device.ID, device.Name, marker, device.MaxInputChannels)
	}
	
	// List output devices only
	fmt.Println("\n=== Output Devices ===")
	outputDevices, err := vocals.GetOutputDevices()
	if err != nil {
		log.Fatalf("Failed to list output devices: %v", err)
	}
	
	for _, device := range outputDevices {
		marker := ""
		if device.IsDefault {
			marker = " (Default)"
		}
		fmt.Printf("  %d: %s%s - %d channels\n", 
			device.ID, device.Name, marker, device.MaxOutputChannels)
	}
	
	// Demonstrate device validation
	fmt.Println("\n=== Device Validation ===")
	if len(inputDevices) > 0 {
		deviceID := inputDevices[0].ID
		fmt.Printf("Validating device %d for recording...\n", deviceID)
		
		// Validate device for mono recording at 44100 Hz
		err = vocals.ValidateAudioDevice(deviceID, true, 1, 44100)
		if err != nil {
			fmt.Printf("Validation failed: %v\n", err)
		} else {
			fmt.Printf("Device %d is valid for mono recording at 44100 Hz\n", deviceID)
		}
		
		// Try validating for stereo recording
		err = vocals.ValidateAudioDevice(deviceID, true, 2, 44100)
		if err != nil {
			fmt.Printf("Stereo validation failed: %v\n", err)
		} else {
			fmt.Printf("Device %d is valid for stereo recording at 44100 Hz\n", deviceID)
		}
	}
	
	// Device information example
	fmt.Println("\n=== Device Information ===")
	if len(devices) > 0 {
		deviceID := devices[0].ID
		
		// Get device manager
		dm := vocals.GetGlobalDeviceManager()
		if err := dm.Initialize(); err != nil {
			log.Fatalf("Failed to initialize device manager: %v", err)
		}
		defer dm.Cleanup()
		
		// Get detailed device information
		info, err := dm.GetDeviceInfo(deviceID)
		if err != nil {
			log.Printf("Failed to get device info: %v", err)
		} else {
			fmt.Printf("Detailed information for device %d:\n%s", deviceID, info)
		}
		
		// Test the device (basic validation)
		fmt.Printf("Testing device %d...\n", deviceID)
		if err := dm.TestDevice(deviceID, true, 3.0); err != nil {
			fmt.Printf("Device test failed: %v\n", err)
		} else {
			fmt.Printf("Device test completed successfully!\n")
		}
	}
	
	fmt.Println("Device management example completed!")
}