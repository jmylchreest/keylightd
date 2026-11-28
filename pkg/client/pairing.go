package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/jmylchreest/keylightd/internal/pairing"
)

type PairedProfile struct {
	Certificate string `json:"certificate,omitempty"`
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
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, errors.New("cannot configure TLS pinning on the default HTTP transport")
	}
	tr := base.Clone()
	if profile.Certificate != "" {
		der, err := base64.StdEncoding.DecodeString(profile.Certificate)
		if err != nil {
			return nil, err
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, err
		}
		tr.TLSClientConfig, err = pairedTLS(cert, profile.Fingerprint)
		if err != nil {
			return nil, err
		}
		c.dialTLS = tr.TLSClientConfig
	} else {
		// Early profiles stored just the pin. Recover the public certificate
		// before sending any credentials and retain normal TLS verification.
		c.dialPinned = func(ctx context.Context, network, address string) (net.Conn, error) {
			return connectWithSavedPin(ctx, network, address, profile.Fingerprint)
		}
		tr.DialTLSContext = c.dialPinned
		tr.Proxy = nil
	}
	c.client.Transport = tr
	c.client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return errors.New("paired daemon redirected the connection")
	}
	return c, nil
}

func pairedTLS(cert *x509.Certificate, pin string) (*tls.Config, error) {
	if pairing.Fingerprint(cert) != pin {
		return nil, errors.New("daemon identity differs from the paired fingerprint")
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	return &tls.Config{
		MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: pairing.ServerName,
		VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) == 0 || pairing.Fingerprint(state.PeerCertificates[0]) != pin {
				return errors.New("daemon identity differs from the paired fingerprint")
			}
			return nil
		},
	}, nil
}

func connectWithSavedPin(ctx context.Context, network, address, pin string) (net.Conn, error) {
	dialer := tls.Dialer{NetDialer: &net.Dialer{Timeout: 10 * time.Second}, Config: &tls.Config{
		MinVersion: tls.VersionTLS12, RootCAs: x509.NewCertPool(), ServerName: pairing.ServerName,
	}}
	// With an empty trust store the first handshake returns the presented
	// chain in a verification error. No application data travels on this connection.
	conn, err := dialer.DialContext(ctx, network, address)
	if err == nil {
		_ = conn.Close()
		return nil, errors.New("unexpected trusted peer during certificate discovery")
	}
	var failure *tls.CertificateVerificationError
	if !errors.As(err, &failure) || len(failure.UnverifiedCertificates) == 0 {
		return nil, err
	}
	dialer.Config, err = pairedTLS(failure.UnverifiedCertificates[0], pin)
	if err != nil {
		return nil, err
	}
	return dialer.DialContext(ctx, network, address)
}

func Redeem(ctx context.Context, logger *slog.Logger, token string) (PairedProfile, error) {
	var profile PairedProfile
	invite, err := pairing.Decode(token)
	if err != nil {
		return profile, err
	}
	profile = PairedProfile{URL: invite.URL, Fingerprint: invite.Fingerprint, Certificate: invite.Certificate}
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
