package keylight

import (
	"strconv"
	"strings"
)

// boolToInt converts a bool to int (true=1, false=0)
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// convertTemperatureToDevice converts Kelvin to device mireds
func convertTemperatureToDevice(kelvin int) int {
	if kelvin < 2900 {
		kelvin = 2900
	} else if kelvin > 7000 {
		kelvin = 7000
	}
	mireds := 1000000 / kelvin
	if mireds > 344 {
		mireds = 344
	} else if mireds < 143 {
		mireds = 143
	}
	return mireds
}

// convertDeviceToTemperature converts device mireds to Kelvin
func ConvertDeviceToTemperature(mireds int) int {
	if mireds < 143 {
		mireds = 143
	} else if mireds > 344 {
		mireds = 344
	}
	return 1000000 / mireds
}

// UnescapeRFC6763Label unescapes a DNS-SD label per RFC 6763 section 6.4
func UnescapeRFC6763Label(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			// Check for \DDD decimal escape
			if i+3 < len(s) && isDigit(s[i+1]) && isDigit(s[i+2]) && isDigit(s[i+3]) {
				val, err := strconv.Atoi(s[i+1 : i+4])
				if err == nil {
					b.WriteByte(byte(val))
					i += 3
					continue
				}
			}
			// Otherwise, just use the next character as-is
			i++
			b.WriteByte(s[i])
		} else {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}
