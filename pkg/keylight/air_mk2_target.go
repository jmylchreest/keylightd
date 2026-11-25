package keylight

import (
	"bytes"
	"encoding/hex"
	"net"
	"slices"
	"strings"
)

func airMK2AdvertisedMAC(text []string) string {
	if !slices.Contains(text, airMK2TLSText) || !slices.Contains(text, "dt=214") || !slices.Contains(text, "mf=Elgato") {
		return ""
	}
	var mac string
	for _, field := range text {
		if value, ok := strings.CutPrefix(field, "id="); ok {
			if mac != "" {
				return ""
			}
			mac = canonicalAirMK2MAC(value)
			if mac == "" {
				return ""
			}
		}
	}
	return mac
}

func canonicalAirMK2MAC(value string) string {
	mac, err := net.ParseMAC(value)
	if err != nil && len(value) == 12 {
		mac, err = hex.DecodeString(value)
	}
	if err != nil || len(mac) != 6 || mac[0]&1 != 0 || bytes.Equal(mac, make([]byte, 6)) {
		return ""
	}
	return mac.String()
}

// AirMK2BluetoothIdentityMatches compares the mDNS identity with the identity
// returned by the peripheral, independently of its Bluetooth address.
func AirMK2BluetoothIdentityMatches(expected string, reported []byte) bool {
	if expected == "" {
		return false
	}
	if len(reported) == 6 {
		return expected == net.HardwareAddr(reported).String()
	}
	return expected == canonicalAirMK2MAC(strings.TrimRight(string(reported), "\x00"))
}
