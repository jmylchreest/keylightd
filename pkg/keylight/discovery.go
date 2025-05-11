package keylight

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
	"time"

	"log/slog"

	"github.com/hashicorp/mdns"
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

// DiscoverLights discovers Key Light devices on the network periodically.
// The interval must be at least 5 seconds. If a shorter interval is provided,
// it will be automatically increased to 5 seconds and a warning will be logged.
// Each discovery run will last (interval - 1) seconds to ensure a 1-second gap
// between discovery runs.
func (m *Manager) DiscoverLights(ctx context.Context, interval time.Duration) error {
	// Enforce minimum interval of 5 seconds
	if interval < 5*time.Second {
		interval = 5 * time.Second
		m.logger.Warn("Discovery interval too short, using minimum of 5 seconds")
	}

	// Create a channel to receive discovered services
	entriesCh := make(chan *mdns.ServiceEntry, 10)
	defer close(entriesCh)

	// Create a ticker for periodic discovery
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Function to perform a single discovery run
	discover := func() error {
		// Create a context with timeout for this discovery run
		// Use interval - 1 second to ensure we don't overlap with next discovery
		discoverCtx, cancel := context.WithTimeout(ctx, interval-time.Second)
		defer cancel()

		// Bridge slog.Logger to log.Logger for mDNS
		var mdnsLogger *log.Logger
		if m.logger != nil {
			mdnsLogger = slogToStdLogger(m.logger)
		} else {
			mdnsLogger = log.New(os.Stderr, "mdns: ", log.LstdFlags)
		}
		params := mdns.DefaultParams(serviceName)
		params.Domain = domain
		params.Entries = entriesCh
		params.Logger = mdnsLogger

		// mDNS is a bit broken on windows apparently (at least with the hasicorp/mDNS library). This is a workaround that appears to work despite a few warnings.
		if runtime.GOOS == "windows" {
			// Attempt to find a suitable network interface for mDNS on Windows
			var selectedInterface *net.Interface
			interfaces, errInterfaces := net.Interfaces()
			if errInterfaces == nil {
				for _, iface := range interfaces {
					i := iface // Create a local copy for the pointer
					if (i.Flags&net.FlagUp) == 0 || (i.Flags&net.FlagLoopback) != 0 || (i.Flags&net.FlagMulticast) == 0 {
						continue
					}
					addrs, errAddrs := i.Addrs()
					if errAddrs != nil {
						continue
					}
					hasIPv4 := false
					for _, addr := range addrs {
						if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
							hasIPv4 = true
							break
						}
					}
					if hasIPv4 {
						selectedInterface = &i
						params.DisableIPv6 = true // Required to allow hasicorp/mDNS to work on Windows
						params.Interface = selectedInterface
						m.logger.Info("Disabling IPv6 and using specific network interface for mDNS on Windows", "interface", i.Name)
						break
					}
				}
			} else {
				m.logger.Warn("Failed to list network interfaces on Windows", "error", errInterfaces)
			}
		}

		// Start the discovery
		errQuery := mdns.Query(params)
		if errQuery != nil {
			return fmt.Errorf("failed to start discovery: %w", errQuery)
		}

		// Process discovered services
		for {
			select {
			case <-discoverCtx.Done():
				// Discovery timeout reached, this is normal
				return nil
			case entry, ok := <-entriesCh:
				if !ok {
					// Channel closed, discovery complete
					return nil
				}
				light, valid := validateLight(entry, m.logger)
				if !valid {
					continue
				}
				m.logger.Debug("Validated Elgato Key Light", "name", light.Name, "id", light.ID, "addr", light.IP, "port", light.Port)
				m.AddLight(light)
			}
		}
	}

	// Run initial discovery
	if err := discover(); err != nil {
		return err
	}

	// Run periodic discovery
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := discover(); err != nil {
				m.logger.Error("Discovery failed", "error", err)
				// Continue running even if discovery fails
			}
		}
	}
}

// validateLight checks if the mDNS entry is a valid Elgato Key Light by querying /elgato/accessory-info
func validateLight(entry *mdns.ServiceEntry, logger *slog.Logger) (Light, bool) {
	if entry == nil || entry.AddrV4 == nil || entry.Port == 0 {
		if logger != nil {
			logger.Debug("Skipping invalid service entry", "name", entry.Name, "addr", entry.AddrV4, "port", entry.Port)
		}
		return Light{}, false
	}

	client := NewKeyLightClient(entry.AddrV4.String(), entry.Port, logger)
	info, err := client.GetAccessoryInfo()
	if err != nil {
		if logger != nil {
			logger.Debug("Failed to get accessory info", "ip", entry.AddrV4, "port", entry.Port, "error", err)
		}
		return Light{}, false
	}
	if !isValidProductName(info.ProductName) {
		if logger != nil {
			logger.Debug("Discovered device is not a valid Elgato Key Light", "productName", info.ProductName, "name", entry.Name, "addr", entry.AddrV4)
		}
		return Light{}, false
	}
	// Build the Light struct with info
	light := Light{
		ID:                entry.Name,
		IP:                entry.AddrV4,
		Port:              entry.Port,
		ProductName:       info.ProductName,
		HardwareBoardType: info.HardwareBoardType,
		FirmwareVersion:   info.FirmwareVersion,
		FirmwareBuild:     info.FirmwareBuildNumber,
		SerialNumber:      info.SerialNumber,
		Name:              info.DisplayName,
	}
	return light, true
}
