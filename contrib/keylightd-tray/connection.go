package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jmylchreest/keylightd/internal/config"
	"github.com/jmylchreest/keylightd/pkg/client"
)

func (a *App) makeClient(s Settings) (client.ClientInterface, error) {
	switch s.ConnectionType {
	case "http":
		if s.APIUrl == "" || s.APIKey == "" {
			return nil, errors.New("API URL and key are required")
		}
		if s.Fingerprint != "" {
			return client.NewPaired(a.logger, client.PairedProfile{URL: s.APIUrl, Certificate: s.Certificate, Fingerprint: s.Fingerprint, APIKey: s.APIKey})
		}
		return client.NewHTTP(a.logger, s.APIUrl, s.APIKey), nil
	case "socket":
		return client.New(a.logger, s.SocketPath), nil
	default:
		return nil, errors.New("unknown connection type")
	}
}
func (a *App) restoreConnection() {
	raw, err := os.ReadFile(filepath.Join(a.getConfigDir(), "connection.json"))
	if os.IsNotExist(err) {
		return
	}
	var saved Settings
	if err == nil {
		err = json.Unmarshal(raw, &saved)
	}
	if err == nil {
		var c client.ClientInterface
		c, err = a.makeClient(saved)
		if err == nil {
			a.client = c
			a.settings = saved
			return
		}
	}
	a.logger.Error("Cannot restore saved connection", "error", err)
}
func (a *App) applySettings(s Settings) error {
	if s.ConnectionType == "socket" {
		s.APIKey = ""
		s.APIUrl = ""
		s.Fingerprint = ""
		s.Certificate = ""
	}
	c, err := a.makeClient(s)
	if err != nil {
		return err
	}
	dir := a.getConfigDir()
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".connection-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	err = json.NewEncoder(f).Encode(s) //nolint:gosec // CreateTemp restricts this connection profile to its owner.
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(f.Name(), filepath.Join(dir, "connection.json")); err != nil {
		return err
	}
	a.connectionMu.Lock()
	a.settings = s
	a.client = c
	a.connectionMu.Unlock()
	a.startWatch(a.ctx)
	a.SignalTrayRefresh()
	return nil
}

func (a *App) HTTPSStatus(socket string) (map[string]any, error) {
	return client.New(a.logger, socket).HTTPS("https.status", "")
}
func (a *App) CreateInvitation(socket, endpoint string) (string, error) {
	result, err := client.New(a.logger, socket).HTTPS("pair.create", endpoint)
	if err != nil {
		return "", err
	}
	token, ok := result["invitation"].(string)
	if !ok {
		return "", errors.New("daemon returned no invitation")
	}
	return token, nil
}
func (a *App) EnableHTTPS(socket, endpoint string) error {
	a.profileMu.Lock()
	defer a.profileMu.Unlock()
	local := client.New(a.logger, socket)
	if _, err := local.HTTPS("https.enable", ""); err != nil {
		return err
	}
	token, err := a.CreateInvitation(socket, endpoint)
	if err != nil {
		return err
	}
	return a.pairClient(token, socket)
}
func (a *App) PairClient(token string) error {
	a.profileMu.Lock()
	defer a.profileMu.Unlock()
	return a.pairClient(token, a.GetSettings().SocketPath)
}
func (a *App) pairClient(token, socket string) error {
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()
	profile, err := client.Redeem(ctx, a.logger, token)
	if err != nil {
		return err
	}
	candidate, err := client.NewPaired(a.logger, profile)
	if err != nil {
		return err
	}
	_, err = candidate.GetLights()
	if err == nil {
		_, err = candidate.Watch(ctx)
	}
	if err == nil {
		err = a.applySettings(Settings{ConnectionType: "http", SocketPath: socket, APIUrl: profile.URL, APIKey: profile.APIKey, Fingerprint: profile.Fingerprint, Certificate: profile.Certificate})
	}
	if err != nil {
		// Avoid leaving a usable credential for a profile the tray could not save.
		if cleanupErr := candidate.RevokePairing(); cleanupErr != nil {
			a.logger.Warn("Could not revoke unsuccessful pairing")
		}
		return fmt.Errorf("pairing verification or save failed: %w", err)
	}
	return nil
}
func (a *App) DisableHTTPS(socket string) error {
	a.profileMu.Lock()
	defer a.profileMu.Unlock()
	local := client.New(a.logger, socket)
	status, err := local.HTTPS("https.status", "")
	if err != nil {
		return err
	}
	saved := a.GetSettings()
	if saved.Fingerprint != "" && saved.Fingerprint == status["fingerprint"] {
		if socket == "" {
			socket = config.GetRuntimeSocketPath()
		}
		if err := a.applySettings(Settings{ConnectionType: "socket", SocketPath: socket}); err != nil {
			return err
		}
	}
	_, err = local.HTTPS("https.disable", "")
	return err
}
func (a *App) ForgetHTTPSIdentity(socket string) error {
	_, err := client.New(a.logger, socket).HTTPS("https.delete-identity", "")
	return err
}

func (a *App) HasSavedConnection() bool {
	_, err := os.Stat(filepath.Join(a.getConfigDir(), "connection.json"))
	return err == nil
}
