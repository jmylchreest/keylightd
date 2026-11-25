//go:build cgo

package airble

import "tinygo.org/x/bluetooth"

func parseAddress(value string) (bluetooth.Address, error) {
	uuid, err := bluetooth.ParseUUID(value)
	return bluetooth.Address{UUID: uuid}, err
}

var adapterEnabled bool

func enableAdapter(adapter *bluetooth.Adapter) error {
	if adapterEnabled {
		return nil
	}
	if err := adapter.Enable(); err != nil {
		return err
	}
	adapterEnabled = true
	return nil
}
