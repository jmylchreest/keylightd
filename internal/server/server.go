package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"

	"github.com/jmylchreest/keylightd/pkg/keylight"
)

// Server represents the Unix socket server
type Server struct {
	logger     *slog.Logger
	manager    keylight.LightManager
	unixServer net.Listener
	config     *Config
}

// Config represents the server configuration
type Config struct {
	UnixSocket string
	AllowLocal bool
}

// New creates a new server instance
func New(logger *slog.Logger, manager keylight.LightManager, config *Config) *Server {
	return &Server{
		logger:  logger,
		manager: manager,
		config:  config,
	}
}

// Start starts the Unix socket server
func (s *Server) Start() error {
	// Ensure socket directory exists
	socketDir := filepath.Dir(s.config.UnixSocket)
	if err := os.MkdirAll(socketDir, 0755); err != nil {
		return fmt.Errorf("failed to create socket directory: %w", err)
	}

	// Remove existing socket if it exists
	if err := os.RemoveAll(s.config.UnixSocket); err != nil {
		return fmt.Errorf("failed to remove existing socket: %w", err)
	}

	// Create Unix socket listener
	listener, err := net.Listen("unix", s.config.UnixSocket)
	if err != nil {
		return fmt.Errorf("failed to create Unix socket: %w", err)
	}

	s.unixServer = listener
	s.logger.Info("Starting Unix socket server", "socket", s.config.UnixSocket)

	// Accept connections
	go func() {
		for {
			conn, err := s.unixServer.Accept()
			if err != nil {
				if !isClosedError(err) {
					s.logger.Error("Failed to accept connection", "error", err)
				}
				return
			}

			go s.handleConnection(conn)
		}
	}()

	return nil
}

// Stop stops the Unix socket server
func (s *Server) Stop(ctx context.Context) error {
	var errs []error

	// Stop Unix socket server
	if s.unixServer != nil {
		if err := s.unixServer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("Unix socket server close error: %w", err))
		}
	}

	// Remove Unix socket
	if err := os.RemoveAll(s.config.UnixSocket); err != nil {
		errs = append(errs, fmt.Errorf("Failed to remove Unix socket: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("server shutdown errors: %v", errs)
	}
	return nil
}

// handleConnection handles a single client connection
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)

	var req map[string]interface{}
	if err := dec.Decode(&req); err != nil {
		s.logger.Error("Invalid request", "error", err)
		enc.Encode(map[string]interface{}{"error": "invalid request"})
		return
	}

	s.logger.Debug("Received request", "req", req)

	action, _ := req["action"].(string)
	switch action {
	case "list_lights":
		lights := s.manager.GetDiscoveredLights()
		resp := make(map[string]interface{})
		for _, l := range lights {
			resp[l.ID] = map[string]interface{}{
				"id":              l.ID,
				"productname":     l.ProductName,
				"serialnumber":    l.SerialNumber,
				"firmwareversion": l.FirmwareVersion,
				"firmwarebuild":   l.FirmwareBuild,
				"ip":              l.IP.String(),
				"port":            l.Port,
				"temperature":     l.Temperature,
				"brightness":      l.Brightness,
				"on":              l.On,
			}
		}
		enc.Encode(resp)
	case "get_light":
		id, _ := req["id"].(string)
		light, err := s.manager.GetLight(id)
		if err != nil {
			enc.Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		enc.Encode(map[string]interface{}{
			"id":              light.ID,
			"productname":     light.ProductName,
			"serialnumber":    light.SerialNumber,
			"firmwareversion": light.FirmwareVersion,
			"firmwarebuild":   light.FirmwareBuild,
			"ip":              light.IP.String(),
			"port":            light.Port,
			"temperature":     light.Temperature,
			"brightness":      light.Brightness,
			"on":              light.On,
		})
	case "set_light":
		id, _ := req["id"].(string)
		property, _ := req["property"].(string)
		value := req["value"]
		var err error
		switch property {
		case "on":
			b, ok := value.(bool)
			if !ok {
				enc.Encode(map[string]interface{}{"error": "invalid value for on"})
				return
			}
			err = s.manager.SetLightState(id, "on", b)
		case "brightness":
			f, ok := value.(float64)
			if !ok {
				enc.Encode(map[string]interface{}{"error": "invalid value for brightness"})
				return
			}
			err = s.manager.SetLightBrightness(id, int(f))
		case "temperature":
			f, ok := value.(float64)
			if !ok {
				enc.Encode(map[string]interface{}{"error": "invalid value for temperature"})
				return
			}
			err = s.manager.SetLightTemperature(id, int(f))
		default:
			enc.Encode(map[string]interface{}{"error": "unknown property"})
			return
		}
		if err != nil {
			enc.Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		enc.Encode(map[string]interface{}{"result": "ok"})
	default:
		enc.Encode(map[string]interface{}{"error": "unknown action"})
	}
}

// isClosedError checks if an error is due to a closed connection
func isClosedError(err error) bool {
	return err.Error() == "use of closed network connection"
}
