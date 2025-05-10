package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"

	"github.com/jmylchreest/keylightd/internal/config"
	"github.com/jmylchreest/keylightd/internal/group"
	"github.com/jmylchreest/keylightd/pkg/keylight"
)

// Server represents the Unix socket server
type Server struct {
	logger     *slog.Logger
	lights     keylight.LightManager
	groups     *group.Manager
	unixServer net.Listener
	cfg        *config.Config
}

// New creates a new server instance
func New(logger *slog.Logger, lights keylight.LightManager, cfg *config.Config) *Server {
	return &Server{
		logger: logger,
		lights: lights,
		groups: group.NewManager(logger, lights, cfg),
		cfg:    cfg,
	}
}

// Start starts the Unix socket server
func (s *Server) Start() error {
	// Ensure socket directory exists
	socketDir := filepath.Dir(s.cfg.Server.UnixSocket)
	if err := os.MkdirAll(socketDir, 0755); err != nil {
		return fmt.Errorf("failed to create socket directory: %w", err)
	}

	// Remove existing socket if it exists
	if err := os.RemoveAll(s.cfg.Server.UnixSocket); err != nil {
		return fmt.Errorf("failed to remove existing socket: %w", err)
	}

	// Create Unix socket listener
	listener, err := net.Listen("unix", s.cfg.Server.UnixSocket)
	if err != nil {
		return fmt.Errorf("failed to create Unix socket: %w", err)
	}

	s.unixServer = listener
	s.logger.Info("Starting Unix socket server", "socket", s.cfg.Server.UnixSocket)

	// Accept connections
	go func() {
		for {
			s.logger.Debug("Waiting for connection")
			conn, err := s.unixServer.Accept()
			if err != nil {
				if !isClosedError(err) {
					s.logger.Error("Failed to accept connection", "error", err)
				}
				return
			}
			s.logger.Debug("New connection accepted", "remote", conn.RemoteAddr())

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
	if err := os.RemoveAll(s.cfg.Server.UnixSocket); err != nil {
		errs = append(errs, fmt.Errorf("Failed to remove Unix socket: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("server shutdown errors: %v", errs)
	}
	return nil
}

// handleConnection handles a single client connection
func (s *Server) handleConnection(conn net.Conn) {
	defer func() {
		s.logger.Debug("Closing connection", "remote", conn.RemoteAddr())
		conn.Close()
	}()

	s.logger.Debug("Handling connection", "remote", conn.RemoteAddr())
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
	s.logger.Debug("Processing action", "action", action)

	switch action {
	case "list_lights":
		lights := s.lights.GetDiscoveredLights()
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
		light, err := s.lights.GetLight(id)
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
			err = s.lights.SetLightState(id, "on", b)
		case "brightness":
			f, ok := value.(float64)
			if !ok {
				enc.Encode(map[string]interface{}{"error": "invalid value for brightness"})
				return
			}
			err = s.lights.SetLightBrightness(id, int(f))
		case "temperature":
			f, ok := value.(float64)
			if !ok {
				enc.Encode(map[string]interface{}{"error": "invalid value for temperature"})
				return
			}
			err = s.lights.SetLightTemperature(id, int(f))
		default:
			enc.Encode(map[string]interface{}{"error": "unknown property"})
			return
		}
		if err != nil {
			enc.Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		enc.Encode(map[string]interface{}{"result": "ok"})
	case "create_group":
		name, _ := req["name"].(string)
		if name == "" {
			s.logger.Error("Group name is required")
			enc.Encode(map[string]interface{}{"error": "group name is required"})
			return
		}
		s.logger.Debug("Creating group", "name", name)
		group, err := s.groups.CreateGroup(name, nil)
		if err != nil {
			s.logger.Error("Failed to create group", "error", err)
			enc.Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		s.logger.Debug("Group created successfully", "id", group.ID, "name", group.Name)
		resp := map[string]interface{}{
			"id":     group.ID,
			"name":   group.Name,
			"lights": group.Lights,
		}
		if err := enc.Encode(resp); err != nil {
			s.logger.Error("Failed to encode response", "error", err)
			return
		}
		s.logger.Debug("Response sent", "response", resp)
		// Flush the connection to ensure the response is sent
		if flusher, ok := conn.(interface{ Flush() error }); ok {
			if err := flusher.Flush(); err != nil {
				s.logger.Error("Failed to flush connection", "error", err)
			}
		}
	case "get_group":
		name, _ := req["name"].(string)
		group, err := s.groups.GetGroup(name)
		if err != nil {
			enc.Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		enc.Encode(map[string]interface{}{
			"id":     group.ID,
			"name":   group.Name,
			"lights": group.Lights,
		})
	case "list_groups":
		groups := s.groups.GetGroups()
		resp := make(map[string]interface{})
		for _, g := range groups {
			resp[g.ID] = map[string]interface{}{
				"name":   g.Name,
				"lights": g.Lights,
			}
		}
		enc.Encode(resp)
	case "set_group":
		name, _ := req["name"].(string)
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
			err = s.groups.SetGroupState(name, b)
		case "brightness":
			f, ok := value.(float64)
			if !ok {
				enc.Encode(map[string]interface{}{"error": "invalid value for brightness"})
				return
			}
			err = s.groups.SetGroupBrightness(name, int(f))
		case "temperature":
			f, ok := value.(float64)
			if !ok {
				enc.Encode(map[string]interface{}{"error": "invalid value for temperature"})
				return
			}
			err = s.groups.SetGroupTemperature(name, int(f))
		default:
			enc.Encode(map[string]interface{}{"error": "unknown property"})
			return
		}
		if err != nil {
			enc.Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		enc.Encode(map[string]interface{}{"result": "ok"})
	case "delete_group":
		name, _ := req["name"].(string)
		err := s.groups.DeleteGroup(name)
		if err != nil {
			enc.Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		enc.Encode(map[string]interface{}{"result": "ok"})
	case "set_group_lights":
		id, _ := req["id"].(string)
		lights, _ := req["lights"].([]interface{})
		if id == "" {
			enc.Encode(map[string]interface{}{"error": "group ID is required"})
			return
		}

		// Convert lights to []string
		lightIDs := make([]string, len(lights))
		for i, light := range lights {
			lightIDs[i] = light.(string)
		}

		if err := s.groups.SetGroupLights(id, lightIDs); err != nil {
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
