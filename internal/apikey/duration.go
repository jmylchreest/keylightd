package apikey

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const day = 24 * time.Hour

// ParseExpiryDuration parses API key expiry strings.
// It accepts standard time.ParseDuration units plus a trailing "d" for days.
// An empty string or "0" returns zero (no expiry). Negative durations are rejected.
func ParseExpiryDuration(input string) (time.Duration, error) {
	input = strings.TrimSpace(input)
	if input == "" || input == "0" {
		return 0, nil
	}

	var d time.Duration
	var err error

	if strings.HasSuffix(input, "d") {
		daysPart := strings.TrimSuffix(input, "d")
		days, parseErr := strconv.ParseFloat(daysPart, 64)
		if parseErr != nil {
			return 0, fmt.Errorf("invalid day duration %q", input)
		}
		d = time.Duration(days * float64(day))
	} else {
		d, err = time.ParseDuration(input)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q: %w", input, err)
		}
	}

	if d < 0 {
		return 0, fmt.Errorf("duration must not be negative: %q", input)
	}
	return d, nil
}
