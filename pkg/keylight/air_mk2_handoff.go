package keylight

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"time"

	"github.com/grandcat/zeroconf"
)

const airMK2TLSText = "tls=CERT_1_VER_EN"

func discoverProvisionedAirMK2(ctx context.Context, ip net.IP, verify func(context.Context, string, int) error) (string, int, error) {
	if ip.To4() == nil {
		return "", 0, SetupDiscoveryFailed
	}
	browseCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return "", 0, fmt.Errorf("%w: %w", SetupDiscoveryFailed, err)
	}
	entries := make(chan *zeroconf.ServiceEntry, 16)
	if err := resolver.Browse(browseCtx, "_elg._tcp", domain, entries); err != nil {
		return "", 0, fmt.Errorf("%w: %w", SetupDiscoveryFailed, err)
	}
	return findProvisionedAirMK2(browseCtx, ip, entries, verify)
}

func findProvisionedAirMK2(ctx context.Context, ip net.IP, entries <-chan *zeroconf.ServiceEntry, verify func(context.Context, string, int) error) (string, int, error) {
	type candidate struct {
		id   string
		port int
	}
	candidates := make(map[candidate]struct{})
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if len(candidates) != 0 {
				return "", 0, fmt.Errorf("%w: %w", SetupVerificationFailed, ctx.Err())
			}
			return "", 0, fmt.Errorf("%w: %w", SetupDiscoveryFailed, ctx.Err())
		case entry, ok := <-entries:
			if !ok {
				if ctx.Err() != nil {
					entries = nil
					continue
				}
				return "", 0, fmt.Errorf("%w: %w", SetupDiscoveryFailed, errors.New("mDNS browse ended"))
			}
			if entry.Instance != "" && entry.Service == "_elg._tcp" && entry.Port > 0 && entry.Port <= 65535 &&
				slices.Contains(entry.Text, airMK2TLSText) && slices.ContainsFunc(entry.AddrIPv4, ip.Equal) {
				id := UnescapeRFC6763Label(entry.Instance + "." + entry.Service + "." + entry.Domain)
				candidates[candidate{id, entry.Port}] = struct{}{}
			}
		case <-ticker.C:
			var match candidate
			for entry := range candidates {
				if err := verify(ctx, ip.String(), entry.port); err != nil {
					continue
				}
				if match.id != "" {
					return "", 0, fmt.Errorf("%w: multiple mDNS instances matched the light", SetupDiscoveryFailed)
				}
				match = entry
			}
			if match.id != "" {
				return match.id, match.port, nil
			}
		}
	}
}
