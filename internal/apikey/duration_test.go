package apikey

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseExpiryDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{name: "empty", input: "", want: 0},
		{name: "zero", input: "0", want: 0},
		{name: "hours", input: "24h", want: 24 * time.Hour},
		{name: "days", input: "30d", want: 30 * 24 * time.Hour},
		{name: "fractional days", input: "1.5d", want: 36 * time.Hour},
		{name: "negative days", input: "-5d", wantErr: true},
		{name: "negative hours", input: "-2h", wantErr: true},
		{name: "invalid", input: "nonsense", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseExpiryDuration(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
