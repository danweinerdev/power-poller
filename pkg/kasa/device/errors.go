package device

import "errors"

var (
	// ErrNotConnected is returned when trying to send commands without a connection.
	ErrNotConnected = errors.New("device not connected")

	// ErrNotSupported is returned when a feature is not supported by the device.
	ErrNotSupported = errors.New("feature not supported by this device")

	// ErrNoEmeter is returned when trying to access emeter on a device without it.
	ErrNoEmeter = errors.New("device does not support energy monitoring")

	// ErrInvalidBrightness is returned for out-of-range brightness values.
	ErrInvalidBrightness = errors.New("brightness must be 0-100")

	// ErrInvalidHue is returned for out-of-range hue values.
	ErrInvalidHue = errors.New("hue must be 0-360")

	// ErrInvalidSaturation is returned for out-of-range saturation values.
	ErrInvalidSaturation = errors.New("saturation must be 0-100")

	// ErrInvalidColorTemp is returned for out-of-range color temperature values.
	ErrInvalidColorTemp = errors.New("color temperature out of range for this device")

	// ErrChildNotFound is returned when a child device index is invalid.
	ErrChildNotFound = errors.New("child device not found")
)
