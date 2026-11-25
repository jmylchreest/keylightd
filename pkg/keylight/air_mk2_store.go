package keylight

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jmylchreest/keylightd/internal/events"
)

// AirMK2CredentialStore holds daemon-owned identities in a private directory.
type AirMK2CredentialStore struct {
	Directory string
}

type airMK2Credentials struct {
	CA          []byte
	Certificate []byte
	Key         []byte
	ServerName  string
	Serial      string
}

func airMK2CredentialFile(id string) string {
	return fmt.Sprintf("%x.json", sha256.Sum256([]byte(id)))
}

func (s AirMK2CredentialStore) save(id string, identity *AirMK2Identity) error {
	if err := os.MkdirAll(s.Directory, 0o700); err != nil {
		return err
	}
	root, err := os.OpenRoot(s.Directory)
	if err != nil {
		return err
	}
	defer root.Close()
	data, err := json.Marshal(airMK2Credentials{identity.CA, identity.ClientCertificate, identity.ClientKey, identity.ServerName, identity.Info.SerialNumber})
	if err != nil {
		return err
	}
	// Publish the complete record without replacing a previous identity.
	temporary := ".pending-" + rand.Text()
	file, err := root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = root.Remove(temporary) }()
	_, writeErr := file.Write(data)
	syncErr := file.Sync()
	closeErr := file.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	return root.Link(temporary, airMK2CredentialFile(id))
}

func (s AirMK2CredentialStore) load(id, host string, port int) (*AirMK2Client, string, error) {
	root, err := os.OpenRoot(s.Directory)
	if err != nil {
		return nil, "", err
	}
	defer root.Close()
	data, err := root.ReadFile(airMK2CredentialFile(id))
	if err != nil {
		return nil, "", err
	}
	var stored airMK2Credentials
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, "", err
	}
	if stored.Serial == "" || stored.ServerName == "" {
		return nil, "", errors.New("air MK.2 credential record has no verified identity")
	}
	pair, err := tls.X509KeyPair(stored.Certificate, stored.Key)
	if err != nil {
		return nil, "", err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(stored.CA) {
		return nil, "", errors.New("invalid device CA certificate")
	}
	client, err := NewAirMK2Client(host, port, &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{pair}, ServerName: stored.ServerName})
	if err == nil {
		client.managedIdentity = true
	}
	return client, stored.Serial, err
}

// SetAirMK2CredentialDirectory enables persisted credentials for automatic discovery.
// Call before starting discovery.
func (p *AirMK2Discovery) SetAirMK2CredentialDirectory(configPath string) {
	p.credentials = &AirMK2CredentialStore{Directory: filepath.Join(filepath.Dir(configPath), "air-mk2-identities")}
}

// ConfigureAirMK2 pairs one discovered light and persists its verified identity.
// The pairer must already be bound to the explicitly selected peripheral.
func (m *Manager) ConfigureAirMK2(ctx context.Context, id string, pairer AirMK2Pairer) error {
	p := m.airMK2
	if p == nil || p.credentials == nil {
		return errors.New("air MK.2 credential storage is unavailable")
	}
	m.mu.RLock()
	light, exists := m.lights[id]
	m.mu.RUnlock()
	if !exists {
		return ErrLightNotFound
	}
	if light.Model != "Elgato Key Light Air MK.2" || len(light.NeedsConfiguration) == 0 {
		return errors.New("light does not require Air MK.2 setup")
	}
	// Only one operation may use the pairing transport at a time.
	if !p.pairing.TryLock() {
		return errors.New("air MK.2 setup is in progress")
	}
	defer p.pairing.Unlock()
	if _, err := os.Stat(filepath.Join(p.credentials.Directory, airMK2CredentialFile(id))); !errors.Is(err, os.ErrNotExist) {
		return errors.New("air MK.2 already has stored credentials or storage is unavailable")
	}
	identity, err := OnboardAirMK2(ctx, pairer)
	if err != nil {
		return err
	}
	return m.publishAirMK2(ctx, light, identity)
}

func (m *Manager) publishAirMK2(ctx context.Context, light Light, identity *AirMK2Identity) error {
	p := m.airMK2
	if light.SerialNumber != "" && !strings.EqualFold(light.SerialNumber, identity.Info.SerialNumber) {
		return fmt.Errorf("%w: paired Air MK.2 identity does not match the selected light", SetupVerificationFailed)
	}
	host, portText, err := net.SplitHostPort(identity.Address)
	if err != nil {
		return err
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := p.credentials.save(light.ID, identity); err != nil {
		return fmt.Errorf("%w: %w", SetupSaveFailed, err)
	}
	client, _, err := p.credentials.load(light.ID, host, port)
	if err != nil {
		return err
	}
	light.IP = net.ParseIP(host)
	light.Port = port
	light.NeedsConfiguration = nil
	light.ManagedCredentials = true
	light.HardwareBoardType = identity.Info.HardwareBoardType
	light.SerialNumber = identity.Info.SerialNumber
	light.FirmwareVersion = identity.Info.FirmwareVersion
	light.FirmwareBuild = identity.Info.FirmwareBuildNumber
	light.PowerInfo = identity.Info.PowerInfo
	if identity.Info.DisplayName != "" {
		light.Name = UnescapeRFC6763Label(identity.Info.DisplayName)
	}
	m.addDevice(ctx, light, client)
	return nil
}

// NearbyAirMK2SetupID identifies a setup session before the light has a LAN identity.
const NearbyAirMK2SetupID = "nearby"

// StartNearbyAirMK2Setup binds the session to a confirmed BLE peripheral and
// saves the result under the verified mDNS instance.
func (m *Manager) StartNearbyAirMK2Setup(ctx context.Context, pairer AirMK2Pairer) (SetupStatus, error) {
	return m.startNearbyAirMK2Setup(ctx, pairer, discoverProvisionedAirMK2)
}

// StartNamedAirMK2Setup requires the BLE peripheral to report the network
// identity of the selected, pending mDNS light.
func (m *Manager) StartNamedAirMK2Setup(ctx context.Context, id string, pairer AirMK2Pairer) (SetupStatus, error) {
	m.mu.RLock()
	light, exists := m.lights[id]
	m.mu.RUnlock()
	if !exists || light.setupMAC == "" || len(light.NeedsConfiguration) == 0 {
		return SetupStatus{}, errors.New("the selected light has no matching Bluetooth setup identity")
	}
	return m.startAirMK2Handoff(ctx, id, light.setupMAC, pairer, discoverProvisionedAirMK2)
}

func (m *Manager) startNearbyAirMK2Setup(ctx context.Context, pairer AirMK2Pairer, handoff airMK2Handoff) (SetupStatus, error) {
	return m.startAirMK2Handoff(ctx, NearbyAirMK2SetupID, "", pairer, handoff)
}

func (m *Manager) startAirMK2Handoff(ctx context.Context, id, expectedMAC string, pairer AirMK2Pairer, handoff airMK2Handoff) (SetupStatus, error) {
	p := m.airMK2
	if p == nil || p.credentials == nil || pairer == nil {
		return SetupStatus{}, errors.New("air MK.2 pairing or credential storage is unavailable")
	}
	return m.setup.start(ctx, id, func(ctx context.Context) (string, error) {
		if !p.pairing.TryLock() {
			return "", errors.New("air MK.2 setup is in progress")
		}
		defer p.pairing.Unlock()
		identity, err := onboardAirMK2(ctx, pairer, expectedMAC, m.setup.waitForConfirmation, m.setup.waitForWiFi, handoff)
		if err != nil {
			return "", err
		}
		if expectedMAC != "" && identity.DeviceID != id {
			return "", SetupIdentityMismatch
		}
		m.mu.RLock()
		light, exists := m.lights[identity.DeviceID]
		m.mu.RUnlock()
		if exists && (light.Model != "Elgato Key Light Air MK.2" || (light.SerialNumber != "" && !strings.EqualFold(light.SerialNumber, identity.Info.SerialNumber))) {
			return "", SetupVerificationFailed
		}
		if !exists {
			light = Light{ID: identity.DeviceID, Name: identity.DeviceID, Manufacturer: "elgato"}
		}
		if err := m.publishAirMK2(ctx, light, identity); err != nil {
			return "", err
		}
		return light.ID, nil
	})
}

// JoinDeviceSetupWiFi submits network settings after physical confirmation.
func (m *Manager) JoinDeviceSetupWiFi(id, wifiID string, options AirMK2WiFiOptions) bool {
	return m.setup.JoinWiFi(id, wifiID, options)
}

// BlinkDeviceSetup requests another blink on the selected peripheral.
func (m *Manager) BlinkDeviceSetup(id, confirmationID string) bool {
	return m.setup.Blink(id, confirmationID)
}

// ConfirmDeviceSetup confirms the light identified itself over BLE.
func (m *Manager) ConfirmDeviceSetup(id, confirmationID string) bool {
	return m.setup.Confirm(id, confirmationID)
}

// StartAirMK2Setup starts pairing with a context owned by the daemon.
func (m *Manager) StartAirMK2Setup(ctx context.Context, id string, pairer AirMK2Pairer) (SetupStatus, error) {
	if pairer == nil {
		return SetupStatus{}, errors.New("air MK.2 pairing transport is unavailable")
	}
	return m.setup.Start(ctx, id, func(ctx context.Context) error {
		if err := m.ConfigureAirMK2(ctx, id, pairer); err != nil {
			m.logger.Debug("Air MK.2 setup failed", "id", id)
			return err
		}
		return nil
	})
}

// DeviceSetupStatus returns the current or last setup session for the device.
func (m *Manager) DeviceSetupStatus(id string) (SetupStatus, bool) {
	return m.setup.Status(id)
}

// CancelDeviceSetup requests cancellation of the active setup session.
func (m *Manager) CancelDeviceSetup(id string) bool {
	return m.setup.Cancel(id)
}

// ForgetAirMK2 removes a daemon-managed Air MK.2 identity.
func (m *Manager) ForgetAirMK2(id string) error {
	p := m.airMK2
	if p == nil || p.credentials == nil {
		return errors.New("air MK.2 credential storage is unavailable")
	}
	if !p.pairing.TryLock() {
		return errors.New("air MK.2 setup is in progress")
	}
	defer p.pairing.Unlock()
	root, err := os.OpenRoot(p.credentials.Directory)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrLightNotFound
		}
		return err
	}
	defer root.Close()
	m.mu.Lock()
	if err := root.Remove(airMK2CredentialFile(id)); err != nil {
		m.mu.Unlock()
		if errors.Is(err, os.ErrNotExist) {
			return ErrLightNotFound
		}
		return err
	}
	light, exists := m.lights[id]
	if exists {
		light.NeedsConfiguration = []string{"credentials_missing"}
		light.State = nil
		light.ManagedCredentials = false
		m.lights[id] = light
	}
	delete(m.clients, id)
	m.mu.Unlock()
	if exists {
		m.emit(events.LightDiscovered, &light)
	}
	return nil
}
