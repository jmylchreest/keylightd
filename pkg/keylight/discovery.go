package keylight

import (
	"context"
	"fmt"
	"net"
	"slices"
	"time"

	"log/slog"

	"github.com/grandcat/zeroconf"

	"github.com/jmylchreest/keylightd/internal/errors"
)

const (
	// Default domain for mDNS discovery
	domain = "local."
)

var (
	// Default service names to discover
	serviceNames = []string{
		"_elg._tcp", // Elgato Key Light
	}

	// validProductNames contains all valid Elgato Key Light product names
	validProductNames = []string{
		"Elgato Key Light",
		"Elgato Key Light Air",
	}

	// Discovery parameters - tuned for reliability across platforms.
	// initialBrowseTimeout must be >= the HTTP client timeout (5s) so that
	// validateLight's HTTP calls don't get pre-empted by the discovery context.
	defaultDiscoveryParams = DiscoveryParams{
		browseAttempts:       3,
		initialBrowseTimeout: 6 * time.Second,
		browseDelay:          500 * time.Millisecond,
		validateTimeout:      5 * time.Second,
	}
)

// No helper functions needed, using slices.Contains directly

// DiscoveryParams holds platform-specific discovery configuration
type DiscoveryParams struct {
	browseAttempts       int
	initialBrowseTimeout time.Duration
	browseDelay          time.Duration
	validateTimeout      time.Duration
}

// calculateMaxDiscoveryTime returns the maximum time a complete discovery cycle could take
func (d DiscoveryParams) calculateMaxDiscoveryTime() time.Duration {
	var total time.Duration
	for i := range d.browseAttempts {
		// Add exponential timeout for this attempt
		total += d.initialBrowseTimeout * time.Duration(1<<uint(i))
		// Add delay if this isn't the last attempt
		if i < d.browseAttempts-1 {
			total += d.browseDelay
		}
	}
	// Add validate timeout — validations may still be in-flight when browse ends
	total += d.validateTimeout
	return total
}

// ServiceEntry represents a discovered mDNS service entry
type ServiceEntry struct {
	Name   string
	AddrV4 net.IP
	Port   int
	Info   string
}

// DiscoverLights discovers Key Light devices on the network periodically.
// It makes multiple discovery attempts with exponential timeouts:
// - First attempt: 3 seconds
// - Second attempt: 6 seconds
// - Third attempt: 12 seconds
// There is a 500ms delay between attempts.
// The interval parameter determines how often this discovery process repeats.
// If interval is less than the total discovery time, it will be automatically increased.
func (m *Manager) StartDiscoveryWithRestart(ctx context.Context, interval time.Duration) {
	// Supervising wrapper that restarts discovery if it panics or returns unexpectedly.
	// Exits cleanly when ctx is canceled.
	for {
		if ctx.Err() != nil {
			return
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					m.logger.Error("panic in discovery loop (will restart)", "recover", r)
				}
			}()
			if err := m.DiscoverLights(ctx, interval); err != nil && ctx.Err() == nil {
				m.logger.Error("discovery loop exited with error (will restart)", "error", err)
			}
		}()
		// If context canceled, stop; otherwise short backoff before restart
		if ctx.Err() != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

func (m *Manager) DiscoverLights(ctx context.Context, interval time.Duration) error {
	params := defaultDiscoveryParams
	minInterval := params.calculateMaxDiscoveryTime() + time.Second
	if interval < minInterval {
		interval = minInterval
		m.logger.Warn("Discovery interval too short, using minimum required interval",
			"minInterval", minInterval)
	}

	// Create a ticker for periodic discovery
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	discover := func() error {
		for i := range params.browseAttempts {
			attempt := i + 1 // convert to 1-based for logging

			// If we already have lights from a previous attempt, skip retries
			if attempt > 1 {
				if len(m.GetLights()) > 0 {
					m.logger.Debug("Skipping retry, lights already discovered",
						"attempt", attempt,
						"lightCount", len(m.GetLights()))
					return nil
				}
				m.logger.Debug("Starting retry attempt", "attempt", attempt)
				time.Sleep(params.browseDelay)
			}

			timeout := params.initialBrowseTimeout * time.Duration(1<<uint(i))
			discoverCtx, cancel := context.WithTimeout(ctx, timeout)

			entries := make(chan *zeroconf.ServiceEntry, 10)
			resolver, err := zeroconf.NewResolver(nil)
			if err != nil {
				cancel()
				return errors.LogErrorAndReturn(
					m.logger,
					errors.Internalf("failed to create zeroconf resolver: %w", err),
					"discovery resolver creation failed",
					"attempt", attempt,
				)
			}

			entriesDone := make(chan struct{})
			go func() {
				defer close(entriesDone)
				for entry := range entries {
					m.logger.Debug("zeroconf: received entry",
						"instance", entry.Instance,
						"service", entry.Service,
						"addrIPv4", entry.AddrIPv4,
						"addrIPv6", entry.AddrIPv6,
						"port", entry.Port,
						"text", entry.Text,
						"attempt", attempt)

					if !slices.Contains(serviceNames, entry.Service) {
						continue
					}

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

					// Use the parent ctx for validation, NOT discoverCtx.
					//nolint:misspell // British spelling intentional
					// This ensures that cancelling the browse timeout does not
					// kill in-flight HTTP validation requests for other lights.
					validateCtx, validateCancel := context.WithTimeout(ctx, params.validateTimeout)
					light, valid := validateLight(validateCtx, localEntry, m.logger)
					validateCancel()

					if !valid {
						m.logger.Debug("zeroconf: entry did not validate as key light",
							"instance", entry.Instance,
							"addrIPv4", entry.AddrIPv4,
							"port", entry.Port,
							"attempt", attempt)
						continue
					}
					m.logger.Debug("light: validated Light",
						"name", light.Name,
						"id", light.ID,
						"addr", light.IP,
						"port", light.Port,
						"attempt", attempt)
					m.AddLight(ctx, light)
				}
			}()

			// Browse for each service name
			for _, serviceName := range serviceNames {
				err = resolver.Browse(discoverCtx, serviceName, domain, entries)
				if err != nil {
					_ = errors.LogErrorAndReturn(
						m.logger,
						err,
						"Browse attempt failed",
						"attempt", attempt,
						"service", serviceName,
					)
				}
			}

			// Wait for the browse timeout to expire. The zeroconf library
			//nolint:misspell // British spelling intentional
			// closes the entries channel when discoverCtx is cancelled,
			// which causes the entries goroutine to drain and exit.
			<-discoverCtx.Done()
			cancel()

			// Wait for all entry processing (including HTTP validations) to complete
			<-entriesDone

			m.logger.Debug("Browse attempt completed",
				"attempt", attempt,
				"timeout", timeout,
				"lightsFound", len(m.GetLights()))
		}

		return nil
	}

	if err := discover(); err != nil {
		return errors.LogErrorAndReturn(
			m.logger,
			err,
			"initial discovery failed",
		)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := discover(); err != nil {
				_ = errors.LogErrorAndReturn(
					m.logger,
					err,
					"light: stopping discovery",
				)
			}
		}
	}
}

// validateLight checks if the mDNS entry is a valid Elgato Key Light by querying /elgato/accessory-info
func validateLight(ctx context.Context, entry *ServiceEntry, logger *slog.Logger) (Light, bool) {
	if entry == nil {
		if logger != nil {
			logger.Debug("validateLight: skipping nil service entry")
		}
		return Light{}, false
	}
	if entry.AddrV4 == nil || entry.Port == 0 {
		if logger != nil {
			logger.Debug("validateLight: skipping invalid service entry",
				"name", entry.Name,
				"addr", entry.AddrV4,
				"port", entry.Port)
		}
		return Light{}, false
	}

	client := NewKeyLightClient(entry.AddrV4.String(), entry.Port, logger)
	info, err := client.GetAccessoryInfo(ctx)
	if err != nil {
		if logger != nil {
			_ = errors.LogErrorAndReturn(
				logger,
				errors.DeviceUnavailablef("failed to get accessory info: %w", err),
				"validateLight: failed to get accessory info",
				"ip", entry.AddrV4,
				"port", entry.Port,
			)
		}
		return Light{}, false
	}
	if !slices.Contains(validProductNames, info.ProductName) {
		if logger != nil {
			logger.Debug("validateLight: discovered device is not a valid Elgato Key Light",
				"productName", info.ProductName,
				"name", entry.Name,
				"addr", entry.AddrV4)
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
