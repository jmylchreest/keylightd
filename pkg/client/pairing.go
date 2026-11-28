package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jmylchreest/keylightd/internal/pairing"
)

type PairedProfile struct {
	URL         string `json:"url"`
	Fingerprint string `json:"fingerprint"`
	APIKey      string `json:"api_key"`
}

func (c *Client) HTTPS(action, url string) (map[string]any, error) {
	switch action {
	case "https.status", "https.enable", "https.disable", "https.delete-identity", "pair.create":
	default:
		return nil, errors.New("unknown HTTPS operation")
	}
	var response map[string]any
	err := c.request(map[string]any{"action": action, "data": map[string]string{"url": url}}, &response)
	return response, err
}

func NewPaired(logger *slog.Logger, profile PairedProfile) (*HTTPClient, error) {
	if !pairing.ValidEndpoint(profile.URL) || len(profile.Fingerprint) != 43 {
		return nil, errors.New("invalid paired HTTPS profile")
	}
	c := NewHTTP(logger, profile.URL, profile.APIKey)
	// The out-of-band invitation authenticates the server public key, including
	// when the private IP or the hostname chosen to reach it changes.
	cfg := &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true, //nolint:gosec // VerifyConnection authenticates the pinned identity.
		VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) == 0 {
				return errors.New("missing daemon certificate")
			}
			cert := state.PeerCertificates[0]
			if pairing.Fingerprint(cert) != profile.Fingerprint {
				return errors.New("daemon identity differs from the paired fingerprint")
			}
			now := time.Now()
			if now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
				return errors.New("daemon certificate is outside its validity period")
			}
			return nil
		}}
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, errors.New("cannot configure TLS pinning on the default HTTP transport")
	}
	tr := base.Clone()
	tr.TLSClientConfig = cfg
	c.client.Transport = tr
	c.client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return errors.New("paired daemon redirected the connection")
	}
	c.dialTLS = cfg
	return c, nil
}

func Redeem(ctx context.Context, logger *slog.Logger, token string) (PairedProfile, error) {
	var profile PairedProfile
	invite, err := pairing.Decode(token)
	if err != nil {
		return profile, err
	}
	profile = PairedProfile{URL: invite.URL, Fingerprint: invite.Fingerprint}
	c, err := NewPaired(logger, profile)
	if err != nil {
		return PairedProfile{}, err
	}
	defer c.client.CloseIdleConnections()
	payload, err := json.Marshal(map[string]string{"secret": invite.Secret})
	if err != nil {
		return PairedProfile{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, profile.URL+"/api/v1/pair/exchange", bytes.NewReader(payload))
	if err != nil {
		return PairedProfile{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return PairedProfile{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden {
		return PairedProfile{}, errors.New("invitation expired or already used")
	}
	if resp.StatusCode != http.StatusOK {
		return PairedProfile{}, fmt.Errorf("pairing exchange returned HTTP %d", resp.StatusCode)
	}
	var key struct {
		APIKey string `json:"api_key"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&key); err != nil || key.APIKey == "" {
		return PairedProfile{}, errors.New("missing API key in exchange response")
	}
	profile.APIKey = key.APIKey
	return profile, nil
}

func (c *HTTPClient) RevokePairing() error {
	return c.request(http.MethodDelete, "/api/v1/pair/credential", nil, nil)
}
