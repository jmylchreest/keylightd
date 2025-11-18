package main

import (
	"testing"
)

func TestFormatCount(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{5, "5"},
		{9, "9"},
		{10, "10"},
		{15, "15"},
		{99, "99"},
		{100, "100"},
		{123, "123"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatCount(tt.input)
			if result != tt.expected {
				t.Errorf("formatCount(%d) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}
