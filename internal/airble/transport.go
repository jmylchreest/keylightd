//go:build linux || (darwin && cgo)

// Package airble provides the native GATT transport for Air MK.2 setup.
package airble

import (
	"errors"
	"fmt"
	"io"

	"tinygo.org/x/bluetooth"
)

var pairingService = bluetooth.NewUUID([16]byte{0x0d, 0x27, 0xfa, 0x90, 0xf0, 0xd4, 0x46, 0x9d, 0xaf, 0xd3, 0x60, 0x5a, 0x6e, 0xbb, 0xdb, 0x13})

// Link is an open connection to the pairing service of a selected light.
type Link struct {
	device  bluetooth.Device
	service bluetooth.DeviceService
}

// Connect opens an explicitly selected peripheral and verifies its pairing service.
func Connect(adapter *bluetooth.Adapter, address bluetooth.Address) (*Link, error) {
	if adapter == nil {
		return nil, errors.New("a Bluetooth adapter is required")
	}
	device, err := adapter.Connect(address, bluetooth.ConnectionParams{})
	if err != nil {
		return nil, fmt.Errorf("connect Bluetooth peripheral: %w", err)
	}
	services, err := device.DiscoverServices([]bluetooth.UUID{pairingService})
	if err != nil || len(services) != 1 {
		_ = device.Disconnect()
		if err == nil {
			err = errors.New("pairing service not found")
		}
		return nil, fmt.Errorf("discover Air MK.2 pairing service: %w", err)
	}
	return &Link{device: device, service: services[0]}, nil
}

// Characteristic opens a characteristic of the verified pairing service.
func (l *Link) Characteristic(uuid bluetooth.UUID) (*Characteristic, error) {
	found, err := l.service.DiscoverCharacteristics([]bluetooth.UUID{uuid})
	if err != nil {
		return nil, err
	}
	if len(found) != 1 {
		return nil, errors.New("pairing characteristic not found")
	}
	return &Characteristic{native: found[0]}, nil
}

// Close disconnects the peripheral.
func (l *Link) Close() error { return l.device.Disconnect() }

// Characteristic wraps an Air MK.2 pairing characteristic.
type Characteristic struct {
	native bluetooth.DeviceCharacteristic
}

// Write sends an acknowledged GATT write.
func (c *Characteristic) Write(data []byte) error {
	n, err := c.native.Write(data)
	if err == nil && n != len(data) {
		return io.ErrShortWrite
	}
	return err
}

// Read reads a pairing characteristic into the supplied buffer.
func (c *Characteristic) Read(data []byte) (int, error) { return c.native.Read(data) }

// Notifications installs a callback with a copy of each notification value.
// Pass nil to unsubscribe.
func (c *Characteristic) Notifications(receive func([]byte)) error {
	if receive == nil {
		return c.native.EnableNotifications(nil)
	}
	return c.native.EnableNotifications(func(data []byte) { receive(append([]byte(nil), data...)) })
}
