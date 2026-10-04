package keylight

import "testing"

func TestUnescapeRFC6763LabelByteRange(t *testing.T) {
	for _, tt := range []struct{ input, want string }{
		{`light\032name`, "light name"},
		{`\255`, string([]byte{255})},
		{`\256`, "256"},
		{`\999`, "999"},
	} {
		if got := UnescapeRFC6763Label(tt.input); got != tt.want {
			t.Errorf("UnescapeRFC6763Label(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
