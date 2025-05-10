package keylight

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/mdns"
)

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

		// Start the discovery
		err := mdns.Lookup("_elg._tcp", entriesCh)
		if err != nil {
			return fmt.Errorf("failed to start discovery: %w", err)
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
				if entry == nil {
					continue
				}

				// Skip non-Elgato services
				if !strings.HasSuffix(entry.Name, "._elg._tcp.local.") {
					m.logger.Log(ctx, -8, "Skipping non-Elgato device", "name", entry.Name)
					continue
				}

				// Validate the service has required fields
				if entry.AddrV4 == nil || entry.Port == 0 {
					m.logger.Log(ctx, -8, "Skipping invalid service entry",
						"name", entry.Name,
						"addr", entry.AddrV4,
						"port", entry.Port)
					continue
				}

				m.logger.Debug("Discovered Elgato Key Light",
					"name", entry.Name,
					"addr", entry.AddrV4,
					"port", entry.Port)

				// Create a new light
				light := Light{
					ID:   entry.Name,
					IP:   entry.AddrV4,
					Port: entry.Port,
				}

				// Add the light to the manager
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
