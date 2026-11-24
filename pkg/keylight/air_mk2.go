package keylight

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/websocket"

	"github.com/jmylchreest/keylightd/internal/config"
)

// AirMK2Client controls a provisioned light using its trusted TLS configuration.
type AirMK2Client struct {
	endpoint string
	dialer   websocket.Dialer
	slot     chan struct{}
}

var _ DeviceClient = (*AirMK2Client)(nil)

// NewAirMK2Client requires a client identity trusted by the light and normal
// server certificate verification against the supplied TLS configuration.
func NewAirMK2Client(host string, port int, config *tls.Config) (*AirMK2Client, error) {
	if config == nil || config.InsecureSkipVerify || (len(config.Certificates) == 0 && config.GetClientCertificate == nil) {
		return nil, fmt.Errorf("Air MK.2 requires verified TLS and a client certificate")
	}
	endpoint := url.URL{Scheme: "wss", Host: net.JoinHostPort(host, strconv.Itoa(port)), Path: "/"}
	return &AirMK2Client{endpoint: endpoint.String(), dialer: websocket.Dialer{TLSClientConfig: config.Clone()}, slot: make(chan struct{}, 1)}, nil
}

func (c *AirMK2Client) GetAccessoryInfo(ctx context.Context) (*AccessoryInfo, error) {
	var info AccessoryInfo
	err := c.exchange(ctx, nil, "get.elgato.accessory-info", &info)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func (c *AirMK2Client) GetLightState(ctx context.Context) (*LightState, error) {
	var state LightState
	if err := c.exchange(ctx, nil, "get.elgato.lights", &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func (c *AirMK2Client) SetLightState(ctx context.Context, on bool, brightness, temperature int) error {
	brightness = max(3, min(100, brightness))
	payload := LightState{NumberOfLights: 1}
	payload.Lights = append(payload.Lights, struct {
		On          int `json:"on"`
		Brightness  int `json:"brightness"`
		Temperature int `json:"temperature"`
	}{boolToInt(on), brightness, temperature})
	var state LightState
	if err := c.exchange(ctx, &payload, "get.elgato.lights", &state); err != nil {
		return err
	}
	if state.NumberOfLights != 1 || len(state.Lights) != 1 || state.Lights[0] != payload.Lights[0] {
		return fmt.Errorf("Air MK.2 state did not match the requested update")
	}
	return nil
}

type airRPCRequest struct {
	Version string      `json:"jsonrpc"`
	ID      string      `json:"id"`
	Method  string      `json:"method"`
	Params  *LightState `json:"params,omitempty"`
}

func (c *AirMK2Client) exchange(ctx context.Context, write *LightState, method string, result any) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	select {
	case c.slot <- struct{}{}:
		defer func() { <-c.slot }()
	case <-ctx.Done():
		return ctx.Err()
	}
	conn, response, err := c.dialer.DialContext(ctx, c.endpoint, nil)
	if err != nil {
		if response != nil {
			_ = response.Body.Close()
		}
		return fmt.Errorf("connect to Air MK.2: %w", err)
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	if err := conn.SetReadDeadline(deadline); err != nil {
		return err
	}
	if err := conn.SetWriteDeadline(deadline); err != nil {
		return err
	}
	conn.SetReadLimit(1 << 20)

	if write != nil {
		if err := conn.WriteJSON(airRPCRequest{"2.0", "write", "put.elgato.lights", write}); err != nil {
			return err
		}
	}
	if err := conn.WriteJSON(airRPCRequest{"2.0", "read", method, nil}); err != nil {
		return err
	}
	for {
		var reply struct {
			Version string          `json:"jsonrpc"`
			ID      string          `json:"id"`
			Method  string          `json:"method"`
			Result  json.RawMessage `json:"result"`
			Error   *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := conn.ReadJSON(&reply); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("read Air MK.2 response: %w", err)
		}
		if reply.Method != "" || (reply.ID != "read" && reply.ID != "write") {
			continue
		}
		if reply.Version != "2.0" {
			return fmt.Errorf("invalid Air MK.2 RPC version")
		}
		if reply.Error != nil {
			return fmt.Errorf("Air MK.2 RPC error %d: %s", reply.Error.Code, reply.Error.Message)
		}
		if reply.ID != "read" {
			continue
		}
		if len(reply.Result) == 0 || string(reply.Result) == "null" {
			return fmt.Errorf("missing Air MK.2 RPC result")
		}
		if err := json.Unmarshal(reply.Result, result); err != nil {
			return err
		}
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), deadline)
		return nil
	}
}

// AirMK2Discovery checks explicitly configured endpoints with a stable client per endpoint.
type AirMK2Discovery struct {
	devices []airMK2Endpoint
	logger  *slog.Logger
}

type airMK2Endpoint struct {
	address string
	host    string
	port    int
	client  *AirMK2Client
}

// NewAirMK2Discovery loads credentials for provisioned Air MK.2 devices.
func NewAirMK2Discovery(entries []config.AirMK2Config, logger *slog.Logger) (*AirMK2Discovery, error) {
	provider := &AirMK2Discovery{logger: logger}
	for _, entry := range entries {
		endpoint, err := loadAirMK2Endpoint(entry)
		if err != nil {
			return nil, fmt.Errorf("Air MK.2 %q: %w", entry.Address, err)
		}
		provider.devices = append(provider.devices, endpoint)
	}
	return provider, nil
}

func loadAirMK2Endpoint(entry config.AirMK2Config) (airMK2Endpoint, error) {
	host, rawPort, err := net.SplitHostPort(entry.Address)
	if err != nil {
		return airMK2Endpoint{}, err
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil || port < 1 || port > 65535 || net.ParseIP(host) == nil {
		return airMK2Endpoint{}, fmt.Errorf("expected an IP address and valid port")
	}
	pair, err := tls.LoadX509KeyPair(entry.CertFile, entry.KeyFile)
	if err != nil {
		return airMK2Endpoint{}, err
	}
	pem, err := os.ReadFile(entry.CAFile)
	if err != nil {
		return airMK2Endpoint{}, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem) {
		return airMK2Endpoint{}, fmt.Errorf("invalid device CA certificate")
	}
	client, err := NewAirMK2Client(host, port, &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{pair}, ServerName: entry.ServerName})
	if err != nil {
		return airMK2Endpoint{}, err
	}
	return airMK2Endpoint{entry.Address, host, port, client}, nil
}

func (p *AirMK2Discovery) openDiscovered(ctx context.Context, entry *ServiceEntry) (Light, DeviceClient, bool) {
	if entry.AddrV4 == nil || entry.Port == 0 {
		return Light{}, nil, false
	}
	const model = "Elgato Key Light Air MK.2"
	light := Light{ID: UnescapeRFC6763Label(entry.Name), Name: UnescapeRFC6763Label(entry.Name),
		IP: entry.AddrV4, Port: entry.Port, Manufacturer: "elgato", Model: model, Experimental: true,
		ProductName: model}
	if p != nil {
		for _, device := range p.devices {
			if device.port != entry.Port || !entry.AddrV4.Equal(net.ParseIP(device.host)) {
				continue
			}
			info, err := device.client.GetAccessoryInfo(ctx)
			if err != nil {
				var verification *tls.CertificateVerificationError
				if errors.As(err, &verification) {
					light.NeedsConfiguration = []string{"server_untrusted"}
					return light, nil, true
				}
				p.logger.Debug("Air MK.2 identity unavailable", "address", device.address, "error", err)
				return Light{}, nil, false
			}
			if info.HardwareBoardType != 214 || info.SerialNumber == "" {
				return Light{}, nil, false
			}
			light.HardwareBoardType = info.HardwareBoardType
			light.SerialNumber = info.SerialNumber
			light.PowerInfo = info.PowerInfo
			light.FirmwareVersion = info.FirmwareVersion
			light.FirmwareBuild = info.FirmwareBuildNumber
			if info.DisplayName != "" {
				light.Name = UnescapeRFC6763Label(info.DisplayName)
			}
			return light, device.client, true
		}
	}
	light.NeedsConfiguration = []string{"credentials_missing"}
	return light, nil, true
}
