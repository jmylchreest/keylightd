package keylight

import (
	"context"
	"fmt"
	"net"
	"time"

	"log/slog"

	"github.com/grandcat/zeroconf"
)

const (
	serviceName = "_elg._tcp"
	domain      = "local."
)

// validProductNames contains all valid Elgato Key Light product names
var validProductNames = []string{"Elgato Key Light"}

func isValidProductName(name string) bool {
	for _, valid := range validProductNames {
		if name == valid {
			return true
		}
	}
	return false
}

// ServiceEntry is a minimal struct for passing discovery info to validateLight
type ServiceEntry struct {
	Name   string
	AddrV4 net.IP
	Port   int
	Info   string
}

// DiscoverLights discovers Key Light devices on the network periodically.
// The interval must be at least 5 seconds. If a shorter interval is provided,
// it will be automatically increased to 5 seconds and a warning will be logged.
// Each discovery run will last (interval - 1) seconds to ensure a 1-second gap
// between discovery runs.
func (m *Manager) DiscoverLights(ctx context.Context, interval time.Duration) error {
	if interval < 5*time.Second {
		interval = 5 * time.Second
		m.logger.Warn("Discovery interval too short, using minimum of 5 seconds")
	}

	// Create a ticker for periodic discovery
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	discover := func() error {
		discoverCtx, cancel := context.WithTimeout(ctx, interval-time.Second)
		defer cancel()

		entries := make(chan *zeroconf.ServiceEntry, 10)
		resolver, err := zeroconf.NewResolver(nil)
		if err != nil {
			return fmt.Errorf("failed to create zeroconf resolver: %w", err)
		}

		go func() {
			for entry := range entries {
				m.logger.Debug("zeroconf: received entry", "instance", entry.Instance, "service", entry.Service, "addrIPv4", entry.AddrIPv4, "addrIPv6", entry.AddrIPv6, "port", entry.Port, "text", entry.Text)
				if entry.Service != serviceName {
					continue // Only process _elg._tcp
				}
				// Convert zeroconf.ServiceEntry to mdns.ServiceEntry-like for validateLight
				var ipv4 net.IP
				if len(entry.AddrIPv4) > 0 {
					ipv4 = entry.AddrIPv4[0]
				}
				localEntry := &ServiceEntry{
					Name:   entry.Instance + "." + entry.Service + "." + entry.Domain,
					AddrV4: ipv4,
					Port:   entry.Port,
					Info:   fmt.Sprint(entry.Text),
				}
				light, valid := validateLight(localEntry, m.logger)
				if !valid {
					m.logger.Debug("zeroconf: entry did not validate as key light", "instance", entry.Instance, "addrIPv4", entry.AddrIPv4, "port", entry.Port)
					continue
				}
				m.logger.Info("light: validated Light", "name", light.Name, "id", light.ID, "addr", light.IP, "port", light.Port)
				m.AddLight(light)
			}
		}()

		err = resolver.Browse(discoverCtx, serviceName, domain, entries)
		if err != nil {
			return fmt.Errorf("zeroconf browse failed: %w", err)
		}

		<-discoverCtx.Done()
		return nil
	}

	if err := discover(); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := discover(); err != nil {
				m.logger.Info("light: stopping discovery", "reason", err)
			}
		}
	}
}

// validateLight checks if the mDNS entry is a valid Elgato Key Light by querying /elgato/accessory-info
func validateLight(entry *ServiceEntry, logger *slog.Logger) (Light, bool) {
	if entry == nil || entry.AddrV4 == nil || entry.Port == 0 {
		if logger != nil {
			logger.Debug("validateLight: skipping invalid service entry", "name", entry.Name, "addr", entry.AddrV4, "port", entry.Port)
		}
		return Light{}, false
	}

	client := NewKeyLightClient(entry.AddrV4.String(), entry.Port, logger)
	info, err := client.GetAccessoryInfo()
	if err != nil {
		if logger != nil {
			logger.Debug("validateLight: failed to get accessory info", "ip", entry.AddrV4, "port", entry.Port, "error", err)
		}
		return Light{}, false
	}
	if !isValidProductName(info.ProductName) {
		if logger != nil {
			logger.Debug("validateLight: discovered device is not a valid Elgato Key Light", "productName", info.ProductName, "name", entry.Name, "addr", entry.AddrV4)
		}
		return Light{}, false
	}
	// Build the Light struct with info
	light := Light{
		ID:                UnescapeRFC6763Label(entry.Name),
		IP:                entry.AddrV4,
		Port:              entry.Port,
		ProductName:       info.ProductName,
		HardwareBoardType: info.HardwareBoardType,
		FirmwareVersion:   info.FirmwareVersion,
		FirmwareBuild:     info.FirmwareBuildNumber,
		SerialNumber:      info.SerialNumber,
		Name:              UnescapeRFC6763Label(info.DisplayName),
	}
	return light, true
}
