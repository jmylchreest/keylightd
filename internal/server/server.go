package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"

	"github.com/jmylchreest/keylightd/internal/apikey"
	"github.com/jmylchreest/keylightd/internal/config"
	"github.com/jmylchreest/keylightd/internal/group"
	"github.com/jmylchreest/keylightd/internal/http/handlers"
	"github.com/jmylchreest/keylightd/internal/http/mw"
	"github.com/jmylchreest/keylightd/internal/http/routes"
	"github.com/jmylchreest/keylightd/pkg/keylight"
)

// Server manages the keylightd daemon, including discovery, groups, and socket/HTTP APIs.
type Server struct {
	logger        *slog.Logger
	cfg           *config.Config
	lights        keylight.LightManager
	groups        *group.Manager
	socketPath    string
	listener      net.Listener
	shutdown      chan struct{}
	wg            sync.WaitGroup
	apikeyManager *apikey.Manager
	rootCtx       context.Context
	rootCancel    context.CancelFunc
	httpServer    *http.Server
}

// New creates a new server instance.
func New(logger *slog.Logger, cfg *config.Config, lightManager keylight.LightManager) *Server {
	groupManager := group.NewManager(logger, lightManager, cfg)
	apikeyMgr := apikey.NewManager(cfg, logger)

	rootCtx, rootCancel := context.WithCancel(context.Background())

	return &Server{
		logger:        logger,
		cfg:           cfg,
		lights:        lightManager,
		groups:        groupManager,
		socketPath:    cfg.Config.Server.UnixSocket,
		shutdown:      make(chan struct{}),
		apikeyManager: apikeyMgr,
		rootCtx:       rootCtx,
		rootCancel:    rootCancel,
	}
}

// Start begins the server operations, including listening on the socket and starting the HTTP server.
func (s *Server) Start() error {
	s.logger.Info("Starting keylightd server")

	// Start cleanup worker for stale lights
	s.wg.Go(func() {
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("panic in cleanup worker", "recover", r)
			}
		}()
		// Create a cancellable context for the worker tied to rootCtx
		workerCtx, cancelWorker := context.WithCancel(s.rootCtx)
		go func() {
			<-s.shutdown   // Wait for server shutdown signal
			cancelWorker() // Cancel the worker's context
		}()
		s.lights.StartCleanupWorker(workerCtx,
			time.Duration(s.cfg.Config.Discovery.CleanupInterval)*time.Second,
			time.Duration(s.cfg.Config.Discovery.CleanupTimeout)*time.Second)
	})

	// Ensure socket directory exists
	sockDir := filepath.Dir(s.socketPath)
	if err := os.MkdirAll(sockDir, 0755); err != nil {
		return fmt.Errorf("failed to create socket directory %s: %w", sockDir, err)
	}

	// Remove existing socket file if it exists
	if _, err := os.Stat(s.socketPath); err == nil {
		if err := os.Remove(s.socketPath); err != nil {
			return fmt.Errorf("failed to remove existing socket file %s: %w", s.socketPath, err)
		}
	}

	// Start listening on Unix socket
	var err error
	s.listener, err = net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on socket %s: %w", s.socketPath, err)
	}
	s.logger.Info("Listening on Unix socket", "path", s.socketPath)

	s.wg.Add(1)
	go s.acceptConnections()

	// Start HTTP server if API is configured
	if s.cfg.Config.API.ListenAddress != "" {
		s.logger.Info("Starting HTTP API server", "address", s.cfg.Config.API.ListenAddress)

		// Create handler implementations
		lightHandler := &handlers.LightHandler{Lights: s.lights}
		groupHandler := &handlers.GroupHandler{Groups: s.groups, Lights: s.lights}
		apiKeyHandler := &handlers.APIKeyHandler{Manager: s.apikeyManager}

		// Create Chi router with middleware stack.
		// Auth is enforced at the Chi level so it covers ALL routes uniformly
		// (both Huma-managed routes and raw handlers like the 207 group state route).
		router := chi.NewRouter()
		router.Use(mw.RequestLogging(s.logger))
		router.Use(mw.APIKeyAuth(s.logger, s.apikeyManager))
		router.Use(mw.RateLimitByIP(mw.DefaultRateLimitConfig()))

		// Create Huma API (security annotations remain for OpenAPI docs only)
		humaConfig := routes.NewHumaConfig("dev", "")
		api := humachi.New(router, humaConfig)

		// Register all routes via shared registration
		routes.Register(api, &routes.Handlers{
			Light:  lightHandler,
			Group:  groupHandler,
			APIKey: apiKeyHandler,
		})

		// Override the group state route with a raw handler for 207 Multi-Status support.
		// Huma doesn't natively support 207, so we use a raw Chi route.
		// Auth is already handled by the Chi middleware above.
		// The Huma registration above still provides OpenAPI documentation.
		router.Put("/api/v1/groups/{id}/state", groupHandler.SetGroupStateRaw(api))

		s.httpServer = &http.Server{
			Addr:         s.cfg.Config.API.ListenAddress,
			Handler:      router,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		}

		s.wg.Go(func() {
			defer func() {
				if r := recover(); r != nil {
					s.logger.Error("panic in HTTP server goroutine", "recover", r)
				}
			}()
			if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				s.logger.Error("HTTP server failed", "error", err)
			}
			s.logger.Info("HTTP server stopped")
		})
	}

	return nil
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() {
	s.logger.Info("Shutting down keylightd server")
	s.rootCancel()    // Cancel root context first
	close(s.shutdown) // Signal all goroutines to stop

	if s.listener != nil {
		s.logger.Info("Closing Unix socket listener")
		s.listener.Close() // Close the socket listener to stop accepting new connections
	}

	if s.httpServer != nil {
		s.logger.Info("Shutting down HTTP server")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(ctx); err != nil {
			s.logger.Error("HTTP server shutdown failed", "error", err)
		}
	}

	s.logger.Info("Waiting for services to stop...")
	s.wg.Wait() // Wait for all goroutines to finish
	s.logger.Info("Keylightd server shut down gracefully")
}

func (s *Server) acceptConnections() {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("panic in acceptConnections", "recover", r)
		}
	}()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.shutdown:
				s.logger.Info("Socket listener shutting down")
				return
			default:
				s.logger.Error("Failed to accept connection", "error", err)
				// Decide if we should continue or not, for now, we continue
				// If the error is critical (e.g. listener closed unexpectedly), this loop might spin.
				// A more robust handling might check for specific error types.
				continue
			}
		}
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("panic in connection handler", "recover", r)
		}
	}()

	// Create a context that is cancelled when the server shuts down
	ctx, cancel := context.WithCancel(s.rootCtx)
	defer cancel()

	go func() {
		select {
		case <-s.shutdown:
			cconn, ok := conn.(*net.UnixConn)
			if ok {
				cconn.CloseRead() // Force connection to unblock for shutdown
			}
			cancel() // cancel the context for this connection
		case <-ctx.Done(): // if connection context is cancelled (e.g. normal close)
			return
		}
	}()

	reader := bufio.NewReader(conn)

	for {
		select {
		case <-ctx.Done(): // Check if context was cancelled (e.g. server shutdown)
			return
		default:
			// Proceed with reading
		}

		line, err := reader.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "use of closed network connection") {
				s.logger.Debug("Client disconnected")
			} else {
				s.logger.Error("Failed to read from connection", "error", err)
			}
			return // Exit handler on read error or EOF
		}

		var req map[string]any
		if err := json.Unmarshal(line, &req); err != nil {
			s.logger.Error("Failed to unmarshal request", "error", err, "request", string(line))
			s.sendError(conn, "", fmt.Sprintf("invalid JSON request: %s", err))
			continue
		}

		action, _ := req["action"].(string)
		id, _ := req["id"].(string)             // Optional request ID for client tracking
		data, _ := req["data"].(map[string]any) // Data payload

		s.logger.Debug("Received request", "action", action, "id", id, "data", data)

		switch action {
		case "ping":
			s.sendResponse(conn, id, map[string]any{"message": "pong"})
		case "list_lights":
			lights := s.lights.GetLights()
			result := make(map[string]any, len(lights))
			for id, light := range lights {
				// Marshal to JSON and then unmarshal to map[string]any
				b, err := json.Marshal(light)
				if err != nil {
					s.logger.Error("Failed to marshal light for socket response", "id", id, "error", err)
					continue
				}
				var m map[string]any
				if err := json.Unmarshal(b, &m); err != nil {
					s.logger.Error("Failed to unmarshal light for socket response", "id", id, "error", err)
					continue
				}
				result[id] = m
			}
			s.sendResponse(conn, id, map[string]any{"lights": result})
		case "get_light":
			lightID, _ := data["id"].(string)
			if lightID == "" {
				s.sendError(conn, id, "missing light ID for get_light")
				continue
			}
			light, err := s.lights.GetLight(ctx, lightID)
			if err != nil {
				s.sendError(conn, id, fmt.Sprintf("failed to get light %s: %s", lightID, err))
				continue
			}
			// Marshal to JSON and then unmarshal to map[string]any
			b, err := json.Marshal(light)
			if err != nil {
				s.logger.Error("Failed to marshal light for socket response", "id", lightID, "error", err)
				s.sendError(conn, id, "internal error marshaling light")
				continue
			}
			var m map[string]any
			if err := json.Unmarshal(b, &m); err != nil {
				s.logger.Error("Failed to unmarshal light for socket response", "id", lightID, "error", err)
				s.sendError(conn, id, "internal error unmarshaling light")
				continue
			}
			s.sendResponse(conn, id, map[string]any{"light": m})
		case "set_light_state":
			// Always extract from 'data' map
			lightID, _ := data["id"].(string)
			property, _ := data["property"].(string)
			value := data["value"] // Keep as any for flexibility

			if lightID == "" || property == "" || value == nil {
				s.sendError(conn, id, "missing id, property, or value for set_light_state")
				continue
			}

			var errSet error
			switch property {
			case "on":
				onVal, ok := value.(bool)
				if !ok {
					errSet = fmt.Errorf("invalid value type for 'on', expected boolean")
				} else {
					errSet = s.lights.SetLightState(ctx, lightID, keylight.OnValue(onVal))
				}
			case "brightness":
				brightnessVal, ok := value.(float64) // JSON numbers are float64
				if !ok {
					errSet = fmt.Errorf("invalid value type for 'brightness', expected number")
				} else {
					errSet = s.lights.SetLightBrightness(ctx, lightID, int(brightnessVal))
				}
			case "temperature":
				tempVal, ok := value.(float64) // JSON numbers are float64
				if !ok {
					errSet = fmt.Errorf("invalid value type for 'temperature', expected number")
				} else {
					errSet = s.lights.SetLightTemperature(ctx, lightID, int(tempVal))
				}
			default:
				errSet = fmt.Errorf("unknown property: %s", property)
			}

			if errSet != nil {
				s.sendError(conn, id, fmt.Sprintf("failed to set light %s state %s: %s", lightID, property, errSet))
				continue
			}
			s.sendResponse(conn, id, map[string]any{"status": "ok"})

		case "create_group":
			name, _ := data["name"].(string)
			lightIDsReq, _ := data["lights"].([]any)
			lightIDs := make([]string, len(lightIDsReq))
			for i, v := range lightIDsReq {
				lightIDs[i], _ = v.(string)
			}
			if name == "" {
				s.sendError(conn, id, "missing name for create_group")
				continue
			}
			group, err := s.groups.CreateGroup(ctx, name, lightIDs)
			if err != nil {
				s.sendError(conn, id, fmt.Sprintf("failed to create group: %s", err))
				continue
			}
			s.sendResponse(conn, id, map[string]any{"group": group})

		case "delete_group":
			groupID, _ := data["id"].(string)
			if groupID == "" {
				s.sendError(conn, id, "missing group ID for delete_group")
				continue
			}
			if err := s.groups.DeleteGroup(groupID); err != nil {
				s.sendError(conn, id, fmt.Sprintf("failed to delete group %s: %s", groupID, err))
				continue
			}
			s.sendResponse(conn, id, map[string]any{"status": "ok"})

		case "get_group":
			groupID, _ := data["id"].(string)
			if groupID == "" {
				s.sendError(conn, id, "missing group ID for get_group")
				continue
			}
			group, err := s.groups.GetGroup(groupID)
			if err != nil {
				s.sendError(conn, id, fmt.Sprintf("failed to get group %s: %s", groupID, err))
				continue
			}
			lights := group.Lights
			if lights == nil {
				lights = []string{}
			}
			s.sendResponse(conn, id, map[string]any{"group": map[string]any{"id": group.ID, "name": group.Name, "lights": lights}})

		case "list_groups":
			groups := s.groups.GetGroups()
			groupMap := make(map[string]any, len(groups))
			for _, g := range groups {
				lights := g.Lights
				if lights == nil {
					lights = []string{}
				}
				groupMap[g.ID] = map[string]any{
					"name":   g.Name,
					"lights": lights,
				}
			}
			s.sendResponse(conn, id, map[string]any{"groups": groupMap})

		case "set_group_lights":
			groupID, _ := data["id"].(string)
			lightIDsReq, _ := data["lights"].([]any)
			lightIDs := make([]string, len(lightIDsReq))
			for i, v := range lightIDsReq {
				lightIDs[i], _ = v.(string)
			}
			if groupID == "" {
				s.sendError(conn, id, "missing group ID for set_group_lights")
				continue
			}
			if err := s.groups.SetGroupLights(ctx, groupID, lightIDs); err != nil {
				s.sendError(conn, id, fmt.Sprintf("failed to set lights for group %s: %s", groupID, err))
				continue
			}
			s.sendResponse(conn, id, map[string]any{"status": "ok"})

		case "set_group_state":
			groupKeys, _ := data["id"].(string)
			property, _ := data["property"].(string)
			value := data["value"]
			if groupKeys == "" || property == "" || value == nil {
				s.sendError(conn, id, "missing id, property, or value for set_group_state")
				continue
			}
			matchedGroups, notFound := s.groups.GetGroupsByKeys(groupKeys)
			if len(matchedGroups) == 0 {
				s.sendError(conn, id, "no groups found for: "+strings.Join(notFound, ", "))
				continue
			}
			var errs []string
			for _, grp := range matchedGroups {
				var errSetGroup error
				switch property {
				case "on":
					onVal, ok := value.(bool)
					if !ok {
						errSetGroup = fmt.Errorf("invalid value type for 'on', expected boolean")
					} else {
						errSetGroup = s.groups.SetGroupState(ctx, grp.ID, onVal)
					}
				case "brightness":
					bVal, ok := value.(float64)
					if !ok {
						errSetGroup = fmt.Errorf("invalid value type for 'brightness', expected number")
					} else {
						errSetGroup = s.groups.SetGroupBrightness(ctx, grp.ID, int(bVal))
					}
				case "temperature":
					tVal, ok := value.(float64)
					if !ok {
						errSetGroup = fmt.Errorf("invalid value type for 'temperature', expected number")
					} else {
						errSetGroup = s.groups.SetGroupTemperature(ctx, grp.ID, int(tVal))
					}
				default:
					errSetGroup = fmt.Errorf("unknown property for group: %s", property)
				}
				if errSetGroup != nil {
					errs = append(errs, fmt.Sprintf("group %s: %s", grp.ID, errSetGroup))
				}
			}
			if len(errs) > 0 {
				s.sendResponse(conn, id, map[string]any{"status": "partial", "errors": errs})
				return
			}
			s.sendResponse(conn, id, map[string]any{"status": "ok"})

		case "apikey_add":
			name, _ := data["name"].(string)
			expiresInStr, _ := data["expires_in"].(string)
			var expiresIn time.Duration
			if expiresInStr != "" {
				expiresInSecs, err := strconv.ParseFloat(expiresInStr, 64)
				if err != nil {
					s.sendError(conn, id, fmt.Sprintf("invalid expires_in format: %s", err))
					continue
				}
				expiresIn = time.Duration(expiresInSecs * float64(time.Second))
			}
			if name == "" {
				s.sendError(conn, id, "missing name for apikey_add")
				continue
			}
			apiKey, err := s.apikeyManager.CreateAPIKey(name, expiresIn)
			if err != nil {
				s.sendError(conn, id, fmt.Sprintf("failed to create API key: %s", err))
				continue
			}
			// Construct a map with lowercase keys for the client
			apiKeyResponse := map[string]any{
				"name":         apiKey.Name,
				"key":          apiKey.Key,
				"created_at":   apiKey.CreatedAt.Format(time.RFC3339Nano),
				"expires_at":   apiKey.ExpiresAt.Format(time.RFC3339Nano),
				"last_used_at": apiKey.LastUsedAt.Format(time.RFC3339Nano),
				"disabled":     apiKey.IsDisabled(),
				// Permissions are not included for now
			}
			s.sendResponse(conn, id, map[string]any{"status": "ok", "key": apiKeyResponse})

		case "apikey_list":
			keys := s.apikeyManager.ListAPIKeys() // Returns []config.APIKey
			// For socket response, we might not want to send the full key string.
			// Let's send Name, CreatedAt, ExpiresAt, LastUsedAt, Disabled and a partial key.
			responseKeys := make([]map[string]any, len(keys))
			for i, k := range keys {
				responseKeys[i] = map[string]any{
					"name":         k.Name,
					"key":          k.Key, // Client side will decide on obfuscation if needed for display
					"created_at":   k.CreatedAt.Format(time.RFC3339Nano),
					"expires_at":   k.ExpiresAt.Format(time.RFC3339Nano),
					"last_used_at": k.LastUsedAt.Format(time.RFC3339Nano),
					"disabled":     k.IsDisabled(),
				}
			}
			s.sendResponse(conn, id, map[string]any{"status": "ok", "keys": responseKeys})

		case "apikey_delete":
			key, _ := data["key"].(string)
			if key == "" {
				s.sendError(conn, id, "missing key for apikey_delete")
				continue
			}
			if err := s.apikeyManager.DeleteAPIKey(key); err != nil {
				s.sendError(conn, id, fmt.Sprintf("failed to delete API key: %s", err))
				continue
			}
			s.sendResponse(conn, id, map[string]any{"status": "ok"})

		case "apikey_set_disabled_status":
			keyOrName, _ := data["key_or_name"].(string) // Corrected: use data, not req
			disabledStr, _ := data["disabled"].(string)  // Corrected: use data, not req

			if keyOrName == "" {
				s.sendError(conn, id, "missing key_or_name for apikey_set_disabled_status")
				continue
			}
			if disabledStr == "" {
				s.sendError(conn, id, "missing disabled state for apikey_set_disabled_status")
				continue
			}

			disabled, err := strconv.ParseBool(disabledStr)
			if err != nil {
				s.sendError(conn, id, fmt.Sprintf("invalid boolean value for disabled state: %s", err))
				continue
			}

			updatedKey, err := s.apikeyManager.SetAPIKeyDisabledStatus(keyOrName, disabled)
			if err != nil {
				s.sendError(conn, id, fmt.Sprintf("failed to set API key disabled status: %s", err))
				continue
			}
			s.sendResponse(conn, id, map[string]any{"status": "ok", "key": updatedKey})

		default:
			s.logger.Warn("received unknown action", "action", action)
			s.sendError(conn, id, "unknown action: "+action)
		}
	}
}

func (s *Server) sendResponse(conn net.Conn, id string, data map[string]any) {
	response := map[string]any{"status": "ok"}
	if id != "" {
		response["id"] = id
	}
	maps.Copy(response, data)
	if err := json.NewEncoder(conn).Encode(response); err != nil {
		s.logger.Error("Failed to send response", "error", err)
	}
}

func (s *Server) sendError(conn net.Conn, id string, message string) {
	s.logger.Error("Sending error response to client", "id", id, "message", message)
	response := map[string]any{"error": message}
	if id != "" {
		response["id"] = id
	}
	if err := json.NewEncoder(conn).Encode(response); err != nil {
		s.logger.Error("Failed to send error response", "error", err)
	}
}
