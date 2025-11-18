package errors

import (
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestSentinelErrors(t *testing.T) {
	// Verify sentinel errors exist and have expected messages
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"ErrNotFound", ErrNotFound, "resource not found"},
		{"ErrInvalidInput", ErrInvalidInput, "invalid input"},
		{"ErrDeviceUnavailable", ErrDeviceUnavailable, "device unavailable"},
		{"ErrInternal", ErrInternal, "internal error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.expected {
				t.Errorf("%s.Error() = %q, want %q", tt.name, tt.err.Error(), tt.expected)
			}
		})
	}
}

func TestLogErrorAndReturn(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	t.Run("returns nil for nil error", func(t *testing.T) {
		result := LogErrorAndReturn(logger, nil, "test message")
		if result != nil {
			t.Errorf("LogErrorAndReturn(nil) = %v, want nil", result)
		}
	})

	t.Run("returns the same error", func(t *testing.T) {
		err := errors.New("test error")
		result := LogErrorAndReturn(logger, err, "test message", "key", "value")
		if result != err {
			t.Errorf("LogErrorAndReturn returned different error")
		}
	})
}

func TestWrapErrorf(t *testing.T) {
	t.Run("returns nil for nil error", func(t *testing.T) {
		result := WrapErrorf(nil, "context %s", "value")
		if result != nil {
			t.Errorf("WrapErrorf(nil) = %v, want nil", result)
		}
	})

	t.Run("wraps error with context", func(t *testing.T) {
		original := errors.New("original error")
		wrapped := WrapErrorf(original, "context %s", "value")

		if !strings.Contains(wrapped.Error(), "context value") {
			t.Errorf("wrapped error should contain context: %v", wrapped)
		}
		if !errors.Is(wrapped, original) {
			t.Error("wrapped error should unwrap to original")
		}
	})
}

func TestIsNotFound(t *testing.T) {
	t.Run("returns true for ErrNotFound", func(t *testing.T) {
		if !IsNotFound(ErrNotFound) {
			t.Error("IsNotFound(ErrNotFound) = false, want true")
		}
	})

	t.Run("returns true for wrapped ErrNotFound", func(t *testing.T) {
		wrapped := NotFoundf("user %s", "123")
		if !IsNotFound(wrapped) {
			t.Error("IsNotFound(wrapped) = false, want true")
		}
	})

	t.Run("returns false for other errors", func(t *testing.T) {
		if IsNotFound(ErrInvalidInput) {
			t.Error("IsNotFound(ErrInvalidInput) = true, want false")
		}
	})
}

func TestIsInvalidInput(t *testing.T) {
	t.Run("returns true for ErrInvalidInput", func(t *testing.T) {
		if !IsInvalidInput(ErrInvalidInput) {
			t.Error("IsInvalidInput(ErrInvalidInput) = false, want true")
		}
	})

	t.Run("returns true for wrapped ErrInvalidInput", func(t *testing.T) {
		wrapped := InvalidInputf("field %s", "name")
		if !IsInvalidInput(wrapped) {
			t.Error("IsInvalidInput(wrapped) = false, want true")
		}
	})

	t.Run("returns false for other errors", func(t *testing.T) {
		if IsInvalidInput(ErrNotFound) {
			t.Error("IsInvalidInput(ErrNotFound) = true, want false")
		}
	})
}

func TestIsDeviceUnavailable(t *testing.T) {
	t.Run("returns true for ErrDeviceUnavailable", func(t *testing.T) {
		if !IsDeviceUnavailable(ErrDeviceUnavailable) {
			t.Error("IsDeviceUnavailable(ErrDeviceUnavailable) = false, want true")
		}
	})

	t.Run("returns true for wrapped ErrDeviceUnavailable", func(t *testing.T) {
		wrapped := DeviceUnavailablef("device %s", "light1")
		if !IsDeviceUnavailable(wrapped) {
			t.Error("IsDeviceUnavailable(wrapped) = false, want true")
		}
	})

	t.Run("returns false for other errors", func(t *testing.T) {
		if IsDeviceUnavailable(ErrNotFound) {
			t.Error("IsDeviceUnavailable(ErrNotFound) = true, want false")
		}
	})
}

func TestNotFoundf(t *testing.T) {
	err := NotFoundf("user %d not found", 123)

	if !strings.Contains(err.Error(), "user 123 not found") {
		t.Errorf("NotFoundf error message incorrect: %v", err)
	}
	if !errors.Is(err, ErrNotFound) {
		t.Error("NotFoundf should wrap ErrNotFound")
	}
}

func TestInvalidInputf(t *testing.T) {
	err := InvalidInputf("field %s is required", "name")

	if !strings.Contains(err.Error(), "field name is required") {
		t.Errorf("InvalidInputf error message incorrect: %v", err)
	}
	if !errors.Is(err, ErrInvalidInput) {
		t.Error("InvalidInputf should wrap ErrInvalidInput")
	}
}

func TestDeviceUnavailablef(t *testing.T) {
	err := DeviceUnavailablef("device %s timeout", "light1")

	if !strings.Contains(err.Error(), "device light1 timeout") {
		t.Errorf("DeviceUnavailablef error message incorrect: %v", err)
	}
	if !errors.Is(err, ErrDeviceUnavailable) {
		t.Error("DeviceUnavailablef should wrap ErrDeviceUnavailable")
	}
}

func TestInternalf(t *testing.T) {
	err := Internalf("unexpected state: %s", "nil pointer")

	if !strings.Contains(err.Error(), "unexpected state: nil pointer") {
		t.Errorf("Internalf error message incorrect: %v", err)
	}
	if !errors.Is(err, ErrInternal) {
		t.Error("Internalf should wrap ErrInternal")
	}
}
