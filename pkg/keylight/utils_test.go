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

func TestTemperatureConversionLimits(t *testing.T) {
	for _, tt := range []struct {
		kelvin int
		mireds int
	}{
		{-1, 344}, {0, 344}, {2899, 344}, {2900, 344},
		{4000, 250}, {6500, 153}, {7000, 143}, {7001, 143},
	} {
		if got := convertTemperatureToDevice(tt.kelvin); got != tt.mireds {
			t.Errorf("convertTemperatureToDevice(%d) = %d, want %d", tt.kelvin, got, tt.mireds)
		}
	}
	for _, tt := range []struct {
		mireds int
		kelvin int
	}{
		{-1, 6993}, {0, 6993}, {142, 6993}, {143, 6993},
		{250, 4000}, {344, 2906}, {345, 2906},
	} {
		if got := ConvertDeviceToTemperature(tt.mireds); got != tt.kelvin {
			t.Errorf("ConvertDeviceToTemperature(%d) = %d, want %d", tt.mireds, got, tt.kelvin)
		}
	}
}
