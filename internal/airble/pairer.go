//go:build linux || (darwin && cgo)

package airble

import (
	"context"
	"errors"
	"fmt"

	"tinygo.org/x/bluetooth"

	"github.com/jmylchreest/keylightd/pkg/keylight"
)

// Only one session uses the default adapter at a time.
var adapterSlot = make(chan struct{}, 1)

type pairer struct {
	address bluetooth.Address
	frame   []byte
}

// NewPairer prepares an explicitly selected peripheral for setup.
func NewPairer(options keylight.AirMK2SetupOptions) (keylight.AirMK2Pairer, error) {
	address, err := parseAddress(options.Peripheral)
	if err != nil {
		return nil, errors.New("invalid Bluetooth peripheral address")
	}
	frame, err := wifiFrame(options.Security, options.SSID, options.Password)
	if err != nil {
		return nil, err
	}
	return &pairer{address: address, frame: frame}, nil
}

func (p *pairer) Provision(ctx context.Context, material keylight.AirMK2Provisioning) (string, int, error) {
	defer clear(p.frame)
	select {
	case <-ctx.Done():
		return "", 0, ctx.Err()
	case adapterSlot <- struct{}{}:
	}
	defer func() { <-adapterSlot }()

	adapter := bluetooth.DefaultAdapter
	if err := enableAdapter(adapter); err != nil {
		return "", 0, fmt.Errorf("%w: enable adapter: %w", keylight.SetupBluetoothFailed, err)
	}
	link, err := Connect(adapter, p.address)
	if err != nil {
		return "", 0, fmt.Errorf("%w: %w", keylight.SetupBluetoothFailed, err)
	}
	defer link.Close() //nolint:errcheck // The setup result depends on the protocol, even if the device disconnects first.

	characteristics := make([]*Characteristic, 0, 7)
	for _, id := range []byte{0x9c, 0x9b, 0x95, 0x93, 0x9e, 0x9d, 0x9f} {
		uuid := bluetooth.NewUUID([16]byte{0x0d, 0x27, 0xfb, id, 0xf0, 0xd4, 0x46, 0x9d, 0xaf, 0xd3, 0x60, 0x5a, 0x6e, 0xbb, 0xdb, 0x13})
		characteristic, err := link.Characteristic(uuid)
		if err != nil {
			return "", 0, fmt.Errorf("%w: discover pairing characteristic %s: %w", keylight.SetupBluetoothFailed, uuid, err)
		}
		characteristics = append(characteristics, characteristic)
	}
	if material.Confirm == nil {
		return "", 0, errors.New("physical light confirmation is required")
	}
	identify := func() error {
		if err := characteristics[6].Write([]byte{1}); err != nil {
			return fmt.Errorf("%w: identify Bluetooth peripheral: %w", keylight.SetupBluetoothFailed, err)
		}
		return nil
	}
	if err := identify(); err != nil {
		return "", 0, err
	}
	if err := material.Confirm(ctx, identify); err != nil {
		return "", 0, err
	}
	if err := transferCertificates(ctx, characteristics[0], characteristics[1], material.CA, material.ServerCertificate, material.ServerKey); err != nil {
		return "", 0, fmt.Errorf("%w: %w", keylight.SetupCertificateInstallFailed, err)
	}
	if err := characteristics[5].Write([]byte{0}); err != nil {
		return "", 0, fmt.Errorf("%w: set Wi-Fi connection option: %w", keylight.SetupBluetoothFailed, err)
	}
	host, err := joinWiFi(ctx, characteristics[2], characteristics[3], characteristics[4], p.frame)
	return host, 0, err
}
