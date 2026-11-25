//go:build !linux && (!darwin || !cgo)

package airble

import (
	"errors"

	"github.com/jmylchreest/keylightd/pkg/keylight"
)

// NewPairer reports that this build has no native BLE transport.
func NewPairer(keylight.AirMK2SetupOptions) (keylight.AirMK2Pairer, error) {
	return nil, errors.New("Bluetooth pairing is unavailable on this build")
}
