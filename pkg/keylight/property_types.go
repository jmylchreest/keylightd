package keylight

import (
	"fmt"
	"github.com/jmylchreest/keylightd/internal/config"
)

// PropertyName represents valid key light property names
type PropertyName string

const (
	// PropertyOn represents the light on/off state
	PropertyOn PropertyName = "on"
	
	// PropertyBrightness represents the light brightness level
	PropertyBrightness PropertyName = "brightness"
	
	// PropertyTemperature represents the light color temperature
	PropertyTemperature PropertyName = "temperature"
)

// LightPropertyValue is an interface for all possible light property values
type LightPropertyValue interface {
	// PropertyName returns the name of the property this value is for
	PropertyName() PropertyName
	
	// Value returns the raw value
	Value() any
	
	// Validate checks if the value is valid for the property
	Validate() error
}

// OnValue represents an on/off state value
type OnValue bool

// PropertyName returns the name of the property
func (v OnValue) PropertyName() PropertyName {
	return PropertyOn
}

// Value returns the underlying bool value
func (v OnValue) Value() any {
	return bool(v)
}

// Validate always returns nil for OnValue as any bool is valid
func (v OnValue) Validate() error {
	return nil
}

// BrightnessValue represents a brightness level
type BrightnessValue int

// PropertyName returns the name of the property
func (v BrightnessValue) PropertyName() PropertyName {
	return PropertyBrightness
}

// Value returns the underlying int value
func (v BrightnessValue) Value() any {
	return int(v)
}

// Validate ensures the brightness is within valid range
func (v BrightnessValue) Validate() error {
	if v < BrightnessValue(config.MinBrightness) || v > BrightnessValue(config.MaxBrightness) {
		return fmt.Errorf("brightness must be between %d and %d, got %d",
			config.MinBrightness, config.MaxBrightness, v)
	}
	return nil
}

// TemperatureValue represents a color temperature value in Kelvin
type TemperatureValue int

// PropertyName returns the name of the property
func (v TemperatureValue) PropertyName() PropertyName {
	return PropertyTemperature
}

// Value returns the underlying int value
func (v TemperatureValue) Value() any {
	return int(v)
}

// Validate ensures the temperature is within valid range
func (v TemperatureValue) Validate() error {
	if v < TemperatureValue(config.MinTemperature) || v > TemperatureValue(config.MaxTemperature) {
		return fmt.Errorf("temperature must be between %d and %d, got %d",
			config.MinTemperature, config.MaxTemperature, v)
	}
	return nil
}

// ValidateProperty validates if the provided property name is valid
func ValidateProperty(property PropertyName) error {
	switch property {
	case PropertyOn, PropertyBrightness, PropertyTemperature:
		return nil
	default:
		return fmt.Errorf("unknown property: %s", property)
	}
}