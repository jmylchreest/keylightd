package errors

import (
	"errors"
	"fmt"
	"log/slog"
)

// ErrNotFound is returned when a requested resource doesn't exist
var ErrNotFound = errors.New("resource not found")

// ErrInvalidInput is returned when the provided input is invalid
var ErrInvalidInput = errors.New("invalid input")

// ErrDeviceUnavailable is returned when a device can't be reached or is not responding
var ErrDeviceUnavailable = errors.New("device unavailable")

// ErrInternal is returned for unexpected internal errors
var ErrInternal = errors.New("internal error")

// LogErrorAndReturn logs an error with structured context and returns it
func LogErrorAndReturn(logger *slog.Logger, err error, message string, args ...any) error {
	// Don't modify nil errors
	if err == nil {
		return nil
	}

	// Log the error with the provided context
	logger.Error(message, append([]any{"error", err}, args...)...)
	return err
}

// WrapErrorf wraps an error with additional context using fmt.Errorf
func WrapErrorf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf(format+": %w", append(args, err)...)
}

// IsNotFound returns true if the error is or wraps ErrNotFound
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsInvalidInput returns true if the error is or wraps ErrInvalidInput
func IsInvalidInput(err error) bool {
	return errors.Is(err, ErrInvalidInput)
}

// IsDeviceUnavailable returns true if the error is or wraps ErrDeviceUnavailable
func IsDeviceUnavailable(err error) bool {
	return errors.Is(err, ErrDeviceUnavailable)
}

// NotFoundf returns a formatted ErrNotFound error
func NotFoundf(format string, args ...any) error {
	return fmt.Errorf(format+": %w", append(args, ErrNotFound)...)
}

// InvalidInputf returns a formatted ErrInvalidInput error
func InvalidInputf(format string, args ...any) error {
	return fmt.Errorf(format+": %w", append(args, ErrInvalidInput)...)
}

// DeviceUnavailablef returns a formatted ErrDeviceUnavailable error
func DeviceUnavailablef(format string, args ...any) error {
	return fmt.Errorf(format+": %w", append(args, ErrDeviceUnavailable)...)
}

// Internalf returns a formatted ErrInternal error
func Internalf(format string, args ...any) error {
	return fmt.Errorf(format+": %w", append(args, ErrInternal)...)
}