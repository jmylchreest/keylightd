package airble

import "tinygo.org/x/bluetooth"

func parseAddress(value string) (bluetooth.Address, error) {
	mac, err := bluetooth.ParseMAC(value)
	return bluetooth.Address{MACAddress: bluetooth.MACAddress{MAC: mac}}, err
}

func enableAdapter(adapter *bluetooth.Adapter) error {
	return adapter.Enable()
}
