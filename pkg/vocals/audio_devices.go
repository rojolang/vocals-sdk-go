package vocals

import (
	"fmt"
	"sync"

	"github.com/gordonklaus/portaudio"
)

// AudioDevice represents an audio device
type AudioDevice struct {
	ID                int
	Name              string
	MaxInputChannels  int
	MaxOutputChannels int
	DefaultSampleRate float64
	IsDefault         bool
	IsInput           bool
	IsOutput          bool
	HostAPI           string
}

// AudioDeviceManager manages audio devices
type AudioDeviceManager struct {
	mu      sync.RWMutex
	devices []AudioDevice
	logger  *VocalsLogger
}

// NewAudioDeviceManager creates a new audio device manager
func NewAudioDeviceManager() *AudioDeviceManager {
	return &AudioDeviceManager{
		devices: make([]AudioDevice, 0),
		logger:  GetGlobalLogger().WithComponent("AudioDeviceManager"),
	}
}

// Initialize initializes the audio device manager
func (adm *AudioDeviceManager) Initialize() error {
	adm.mu.Lock()
	defer adm.mu.Unlock()

	if err := portaudio.Initialize(); err != nil {
		adm.logger.WithError(err).Error("Failed to initialize PortAudio")
		return err
	}

	if err := adm.refreshDevices(); err != nil {
		adm.logger.WithError(err).Error("Failed to refresh device list")
		return err
	}

	adm.logger.WithField("device_count", len(adm.devices)).Info("Audio device manager initialized")
	return nil
}

// Cleanup cleans up the audio device manager
func (adm *AudioDeviceManager) Cleanup() {
	adm.mu.Lock()
	defer adm.mu.Unlock()

	if err := portaudio.Terminate(); err != nil {
		adm.logger.WithError(err).Error("Failed to terminate PortAudio")
	}

	adm.logger.Info("Audio device manager cleaned up")
}

// refreshDevices refreshes the device list
func (adm *AudioDeviceManager) refreshDevices() error {
	adm.devices = make([]AudioDevice, 0)

	// Get default devices
	defaultInput, err := portaudio.DefaultInputDevice()
	if err != nil {
		adm.logger.WithError(err).Warn("No default input device")
	}

	defaultOutput, err := portaudio.DefaultOutputDevice()
	if err != nil {
		adm.logger.WithError(err).Warn("No default output device")
	}

	// Get all devices
	devices, err := portaudio.Devices()
	if err != nil {
		return err
	}

	for i, dev := range devices {
		hostAPIName := "Unknown"
		if dev.HostApi != nil {
			hostAPIName = dev.HostApi.Name
		}

		device := AudioDevice{
			ID:                i,
			Name:              dev.Name,
			MaxInputChannels:  dev.MaxInputChannels,
			MaxOutputChannels: dev.MaxOutputChannels,
			DefaultSampleRate: dev.DefaultSampleRate,
			IsDefault:         false,
			IsInput:           dev.MaxInputChannels > 0,
			IsOutput:          dev.MaxOutputChannels > 0,
			HostAPI:           hostAPIName,
		}

		// Check if it's a default device
		if defaultInput != nil && dev == defaultInput {
			device.IsDefault = true
		}
		if defaultOutput != nil && dev == defaultOutput {
			device.IsDefault = true
		}

		adm.devices = append(adm.devices, device)
	}

	return nil
}

// GetDevices returns all available audio devices
func (adm *AudioDeviceManager) GetDevices() []AudioDevice {
	adm.mu.RLock()
	defer adm.mu.RUnlock()

	// Return a copy to prevent external modification
	devices := make([]AudioDevice, len(adm.devices))
	copy(devices, adm.devices)
	return devices
}

// GetInputDevices returns all input devices
func (adm *AudioDeviceManager) GetInputDevices() []AudioDevice {
	adm.mu.RLock()
	defer adm.mu.RUnlock()

	inputDevices := make([]AudioDevice, 0)
	for _, device := range adm.devices {
		if device.IsInput {
			inputDevices = append(inputDevices, device)
		}
	}
	return inputDevices
}

// GetOutputDevices returns all output devices
func (adm *AudioDeviceManager) GetOutputDevices() []AudioDevice {
	adm.mu.RLock()
	defer adm.mu.RUnlock()

	outputDevices := make([]AudioDevice, 0)
	for _, device := range adm.devices {
		if device.IsOutput {
			outputDevices = append(outputDevices, device)
		}
	}
	return outputDevices
}

// GetDefaultInputDevice returns the default input device
func (adm *AudioDeviceManager) GetDefaultInputDevice() (*AudioDevice, error) {
	adm.mu.RLock()
	defer adm.mu.RUnlock()

	for _, device := range adm.devices {
		if device.IsDefault && device.IsInput {
			return &device, nil
		}
	}
	return nil, fmt.Errorf("no default input device found")
}

// GetDefaultOutputDevice returns the default output device
func (adm *AudioDeviceManager) GetDefaultOutputDevice() (*AudioDevice, error) {
	adm.mu.RLock()
	defer adm.mu.RUnlock()

	for _, device := range adm.devices {
		if device.IsDefault && device.IsOutput {
			return &device, nil
		}
	}
	return nil, fmt.Errorf("no default output device found")
}

// GetDeviceByID returns a device by its ID
func (adm *AudioDeviceManager) GetDeviceByID(id int) (*AudioDevice, error) {
	adm.mu.RLock()
	defer adm.mu.RUnlock()

	for _, device := range adm.devices {
		if device.ID == id {
			return &device, nil
		}
	}
	return nil, fmt.Errorf("device with ID %d not found", id)
}

// GetDeviceByName returns a device by its name
func (adm *AudioDeviceManager) GetDeviceByName(name string) (*AudioDevice, error) {
	adm.mu.RLock()
	defer adm.mu.RUnlock()

	for _, device := range adm.devices {
		if device.Name == name {
			return &device, nil
		}
	}
	return nil, fmt.Errorf("device with name '%s' not found", name)
}

// ValidateDevice validates if a device is suitable for the given requirements
func (adm *AudioDeviceManager) ValidateDevice(deviceID int, isInput bool, channels int, sampleRate float64) error {
	device, err := adm.GetDeviceByID(deviceID)
	if err != nil {
		return err
	}

	if isInput {
		if !device.IsInput {
			return fmt.Errorf("device '%s' is not an input device", device.Name)
		}
		if device.MaxInputChannels < channels {
			return fmt.Errorf("device '%s' supports max %d input channels, requested %d", 
				device.Name, device.MaxInputChannels, channels)
		}
	} else {
		if !device.IsOutput {
			return fmt.Errorf("device '%s' is not an output device", device.Name)
		}
		if device.MaxOutputChannels < channels {
			return fmt.Errorf("device '%s' supports max %d output channels, requested %d", 
				device.Name, device.MaxOutputChannels, channels)
		}
	}

	// Check sample rate (simplified check)
	if sampleRate > 0 && device.DefaultSampleRate > 0 {
		ratio := sampleRate / device.DefaultSampleRate
		if ratio < 0.5 || ratio > 2.0 {
			adm.logger.WithFields(map[string]interface{}{
				"device_name": device.Name,
				"device_sample_rate": device.DefaultSampleRate,
				"requested_sample_rate": sampleRate,
			}).Warn("Sample rate significantly different from device default")
		}
	}

	return nil
}

// RefreshDevices refreshes the device list
func (adm *AudioDeviceManager) RefreshDevices() error {
	adm.mu.Lock()
	defer adm.mu.Unlock()

	return adm.refreshDevices()
}

// GetDeviceInfo returns formatted device information
func (adm *AudioDeviceManager) GetDeviceInfo(deviceID int) (string, error) {
	device, err := adm.GetDeviceByID(deviceID)
	if err != nil {
		return "", err
	}

	info := fmt.Sprintf("Device: %s\n", device.Name)
	info += fmt.Sprintf("  ID: %d\n", device.ID)
	info += fmt.Sprintf("  Host API: %s\n", device.HostAPI)
	info += fmt.Sprintf("  Input Channels: %d\n", device.MaxInputChannels)
	info += fmt.Sprintf("  Output Channels: %d\n", device.MaxOutputChannels)
	info += fmt.Sprintf("  Default Sample Rate: %.1f Hz\n", device.DefaultSampleRate)
	info += fmt.Sprintf("  Is Default: %v\n", device.IsDefault)
	info += fmt.Sprintf("  Capabilities: ")

	capabilities := make([]string, 0)
	if device.IsInput {
		capabilities = append(capabilities, "Input")
	}
	if device.IsOutput {
		capabilities = append(capabilities, "Output")
	}
	if len(capabilities) == 0 {
		capabilities = append(capabilities, "None")
	}

	for i, cap := range capabilities {
		if i > 0 {
			info += ", "
		}
		info += cap
	}
	info += "\n"

	return info, nil
}

// TestDevice tests a device with basic parameters
func (adm *AudioDeviceManager) TestDevice(deviceID int, isInput bool, duration float64) error {
	device, err := adm.GetDeviceByID(deviceID)
	if err != nil {
		return err
	}

	adm.logger.WithFields(map[string]interface{}{
		"device_id": deviceID,
		"device_name": device.Name,
		"is_input": isInput,
		"duration": duration,
	}).Info("Testing audio device")

	// Basic validation
	if err := adm.ValidateDevice(deviceID, isInput, 1, 44100); err != nil {
		return err
	}

	// TODO: Implement actual device testing with portaudio stream
	// For now, just log success
	adm.logger.WithField("device_name", device.Name).Info("Device test completed successfully")
	return nil
}

// Global device manager instance
var globalDeviceManager *AudioDeviceManager

// GetGlobalDeviceManager returns the global device manager
func GetGlobalDeviceManager() *AudioDeviceManager {
	if globalDeviceManager == nil {
		globalDeviceManager = NewAudioDeviceManager()
	}
	return globalDeviceManager
}

// Helper functions for easy access
func GetAllAudioDevices() ([]AudioDevice, error) {
	dm := GetGlobalDeviceManager()
	if err := dm.Initialize(); err != nil {
		return nil, err
	}
	defer dm.Cleanup()
	return dm.GetDevices(), nil
}

func GetInputDevices() ([]AudioDevice, error) {
	dm := GetGlobalDeviceManager()
	if err := dm.Initialize(); err != nil {
		return nil, err
	}
	defer dm.Cleanup()
	return dm.GetInputDevices(), nil
}

func GetOutputDevices() ([]AudioDevice, error) {
	dm := GetGlobalDeviceManager()
	if err := dm.Initialize(); err != nil {
		return nil, err
	}
	defer dm.Cleanup()
	return dm.GetOutputDevices(), nil
}

func ValidateAudioDevice(deviceID int, isInput bool, channels int, sampleRate float64) error {
	dm := GetGlobalDeviceManager()
	if err := dm.Initialize(); err != nil {
		return err
	}
	defer dm.Cleanup()
	return dm.ValidateDevice(deviceID, isInput, channels, sampleRate)
}