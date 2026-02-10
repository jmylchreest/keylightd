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

	logfilter "github.com/jmylchreest/slog-logfilter"

	"github.com/jmylchreest/keylightd/internal/apikey"
	"github.com/jmylchreest/keylightd/internal/config"
	"github.com/jmylchreest/keylightd/internal/events"
	"github.com/jmylchreest/keylightd/internal/group"
	"github.com/jmylchreest/keylightd/internal/http/handlers"
	"github.com/jmylchreest/keylightd/internal/http/mw"
	"github.com/jmylchreest/keylightd/internal/http/routes"
	"github.com/jmylchreest/keylightd/internal/logging"
	"github.com/jmylchreest/keylightd/internal/utils"
	"github.com/jmylchreest/keylightd/internal/ws"
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
	eventBus      *events.Bus
}

// New creates a new server instance.
func New(logger *slog.Logger, cfg *config.Config, lightManager keylight.LightManager) *Server {
	groupManager := group.NewManager(logger, lightManager, cfg)
	apikeyMgr := apikey.NewManager(cfg, logger)
	eventBus := events.NewBus()

	// Wire the event bus into managers so they emit state change events.
	if lm, ok := lightManager.(*keylight.Manager); ok {
		lm.SetEventBus(eventBus)
	}
	groupManager.SetEventBus(eventBus)

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
		eventBus:      eventBus,
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
		loggingHandler := &handlers.LoggingHandler{Logger: s.logger}

		// Create Chi router with global middleware.
		// Rate limiting runs at Chi level (before auth) to protect against brute-force.
		router := chi.NewRouter()
		router.Use(mw.RequestLogging(s.logger))
		router.Use(mw.RateLimitByIP(mw.DefaultRateLimitConfig()))

		// Create Huma API
		humaConfig := routes.NewHumaConfig("dev", "")
		api := humachi.New(router, humaConfig)

		// Add Huma-level auth middleware. This checks each operation's Security
		// field to determine if auth is needed. Public routes (health, OpenAPI
		// spec, docs) have no Security set and pass through unauthenticated.
		api.UseMiddleware(mw.HumaAuth(api, s.logger, s.apikeyManager))

		// Register all routes via shared registration
		routes.Register(api, &routes.Handlers{
			HealthCheck: handlers.HealthCheck,
			Light:       lightHandler,
			Group:       groupHandler,
			APIKey:      apiKeyHandler,
			Logging:     loggingHandler,
		})

		// Override the group state route with a raw handler for 207 Multi-Status support.
		// Huma doesn't natively support 207, so we use a raw Chi route.
		// Auth is applied via router.With() since this bypasses Huma's middleware.
		// The Huma registration above still provides OpenAPI documentation.
		rawAuth := mw.RawAPIKeyAuth(s.logger, s.apikeyManager)
		router.With(rawAuth).Put("/api/v1/groups/{id}/state", groupHandler.SetGroupStateRaw(api))

		// Start WebSocket hub and register the endpoint.
		// The hub runs in a background goroutine and broadcasts events from the event bus.
		wsHub := ws.NewHub(s.logger, s.eventBus)
		s.wg.Go(func() {
			defer func() {
				if r := recover(); r != nil {
					s.logger.Error("panic in WebSocket hub", "recover", r)
				}
			}()
			wsHub.Run(s.rootCtx)
		})
		router.With(rawAuth).Get("/api/v1/ws", ws.Handler(wsHub, s.logger))

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
			lightID, _ := data["id"].(string)
			if lightID == "" {
				s.sendError(conn, id, "missing id for set_light_state")
				continue
			}

			// Support both single-property (property+value) and multi-property (on, brightness, temperature) modes.
			property, _ := data["property"].(string)
			value := data["value"]

			var errs []string
			if property != "" && value != nil {
				// Legacy single-property mode
				if err := s.setLightProperty(ctx, lightID, property, value); err != nil {
					s.sendError(conn, id, fmt.Sprintf("failed to set light %s state %s: %s", lightID, property, err))
					continue
				}
			} else {
				// Multi-property mode: check for on, brightness, temperature in data
				set := false
				if onVal, ok := data["on"]; ok {
					set = true
					if err := s.setLightProperty(ctx, lightID, "on", onVal); err != nil {
						errs = append(errs, err.Error())
					}
				}
				if bVal, ok := data["brightness"]; ok {
					set = true
					if err := s.setLightProperty(ctx, lightID, "brightness", bVal); err != nil {
						errs = append(errs, err.Error())
					}
				}
				if tVal, ok := data["temperature"]; ok {
					set = true
					if err := s.setLightProperty(ctx, lightID, "temperature", tVal); err != nil {
						errs = append(errs, err.Error())
					}
				}
				if !set {
					s.sendError(conn, id, "missing property/value or on/brightness/temperature for set_light_state")
					continue
				}
			}

			if len(errs) > 0 {
				s.sendError(conn, id, fmt.Sprintf("failed to set light %s state: %s", lightID, strings.Join(errs, "; ")))
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
			groupList := make([]map[string]any, 0, len(groups))
			for _, g := range groups {
				lights := g.Lights
				if lights == nil {
					lights = []string{}
				}
				groupList = append(groupList, map[string]any{
					"id":     g.ID,
					"name":   g.Name,
					"lights": lights,
				})
			}
			s.sendResponse(conn, id, map[string]any{"groups": groupList})

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
			if groupKeys == "" {
				s.sendError(conn, id, "missing id for set_group_state")
				continue
			}
			matchedGroups, notFound := s.groups.GetGroupsByKeys(groupKeys)
			if len(matchedGroups) == 0 {
				s.sendError(conn, id, "no groups found for: "+strings.Join(notFound, ", "))
				continue
			}

			// Build list of properties to set.
			// Support both single-property (property+value) and multi-property (on, brightness, temperature).
			type propVal struct {
				name  string
				value any
			}
			var props []propVal

			property, _ := data["property"].(string)
			value := data["value"]
			if property != "" && value != nil {
				props = append(props, propVal{property, value})
			} else {
				if v, ok := data["on"]; ok {
					props = append(props, propVal{"on", v})
				}
				if v, ok := data["brightness"]; ok {
					props = append(props, propVal{"brightness", v})
				}
				if v, ok := data["temperature"]; ok {
					props = append(props, propVal{"temperature", v})
				}
			}
			if len(props) == 0 {
				s.sendError(conn, id, "missing property/value or on/brightness/temperature for set_group_state")
				continue
			}

			var errs []string
			for _, grp := range matchedGroups {
				for _, p := range props {
					if err := s.setGroupProperty(ctx, grp.ID, p.name, p.value); err != nil {
						errs = append(errs, fmt.Sprintf("group %s: %s", grp.ID, err))
					}
				}
			}
			if len(errs) > 0 {
				s.sendResponse(conn, id, map[string]any{"status": "partial", "errors": errs})
				continue
			}
			s.sendResponse(conn, id, map[string]any{"status": "ok"})

		case "apikey_add":
			name, _ := data["name"].(string)
			expiresInStr, _ := data["expires_in"].(string)
			var expiresIn time.Duration
			if expiresInStr != "" {
				// Try Go duration string first (e.g., "720h", "30m"), then seconds-as-float for backward compat
				d, err := time.ParseDuration(expiresInStr)
				if err != nil {
					expiresInSecs, err2 := strconv.ParseFloat(expiresInStr, 64)
					if err2 != nil {
						s.sendError(conn, id, fmt.Sprintf("invalid expires_in format (use Go duration like '720h' or seconds): %s", err))
						continue
					}
					expiresIn = time.Duration(expiresInSecs * float64(time.Second))
				} else {
					expiresIn = d
				}
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
			keyOrName, _ := data["key_or_name"].(string)

			if keyOrName == "" {
				s.sendError(conn, id, "missing key_or_name for apikey_set_disabled_status")
				continue
			}

			// Accept disabled as bool or string for compatibility with both HTTP and legacy socket clients
			var disabled bool
			switch v := data["disabled"].(type) {
			case bool:
				disabled = v
			case string:
				var err error
				disabled, err = strconv.ParseBool(v)
				if err != nil {
					s.sendError(conn, id, fmt.Sprintf("invalid boolean value for disabled state: %s", err))
					continue
				}
			default:
				s.sendError(conn, id, "missing or invalid disabled state for apikey_set_disabled_status")
				continue
			}

			updatedKey, err := s.apikeyManager.SetAPIKeyDisabledStatus(keyOrName, disabled)
			if err != nil {
				s.sendError(conn, id, fmt.Sprintf("failed to set API key disabled status: %s", err))
				continue
			}
			s.sendResponse(conn, id, map[string]any{"status": "ok", "key": updatedKey})

		case "health":
			s.sendResponse(conn, id, map[string]any{"health": "ok"})

		case "list_filters":
			filters := logfilter.GetFilters()
			level := logfilter.GetLevel()

			filterList := make([]map[string]any, len(filters))
			for i, f := range filters {
				fm := map[string]any{
					"type":    f.Type,
					"pattern": f.Pattern,
					"level":   f.Level,
					"enabled": f.Enabled,
				}
				if f.OutputLevel != "" {
					fm["output_level"] = f.OutputLevel
				}
				if f.ExpiresAt != nil {
					fm["expires_at"] = f.ExpiresAt.Format(time.RFC3339Nano)
				}
				filterList[i] = fm
			}

			s.sendResponse(conn, id, map[string]any{
				"level":   handlers.LevelToString(level),
				"filters": filterList,
			})

		case "set_filters":
			filtersRaw, _ := data["filters"].([]any)
			var newFilters []logfilter.LogFilter
			for _, raw := range filtersRaw {
				fm, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				f := logfilter.LogFilter{
					Type:        stringFromMap(fm, "type"),
					Pattern:     stringFromMap(fm, "pattern"),
					Level:       stringFromMap(fm, "level"),
					OutputLevel: stringFromMap(fm, "output_level"),
					Enabled:     boolFromMap(fm, "enabled"),
				}
				if expiresStr := stringFromMap(fm, "expires_at"); expiresStr != "" {
					if t, err := time.Parse(time.RFC3339Nano, expiresStr); err == nil {
						f.ExpiresAt = &t
					}
				}
				newFilters = append(newFilters, f)
			}

			if errs := logging.ValidateFilters(newFilters); len(errs) > 0 {
				s.sendError(conn, id, fmt.Sprintf("invalid filters: %s", logging.FormatErrors(errs)))
				continue
			}

			logfilter.SetFilters(newFilters)
			s.logger.Info("Log filters updated via socket", "count", len(newFilters))

			// Return updated state
			updatedFilters := logfilter.GetFilters()
			resultFilters := make([]map[string]any, len(updatedFilters))
			for i, f := range updatedFilters {
				fm := map[string]any{
					"type":    f.Type,
					"pattern": f.Pattern,
					"level":   f.Level,
					"enabled": f.Enabled,
				}
				if f.OutputLevel != "" {
					fm["output_level"] = f.OutputLevel
				}
				if f.ExpiresAt != nil {
					fm["expires_at"] = f.ExpiresAt.Format(time.RFC3339Nano)
				}
				resultFilters[i] = fm
			}
			s.sendResponse(conn, id, map[string]any{
				"level":   handlers.LevelToString(logfilter.GetLevel()),
				"filters": resultFilters,
			})

		case "set_level":
			level, _ := data["level"].(string)
			if level == "" {
				s.sendError(conn, id, "missing level for set_level")
				continue
			}
			validated := utils.ValidateLogLevel(level)
			if validated != level {
				s.sendError(conn, id, fmt.Sprintf("invalid log level %q; must be debug, info, warn, or error", level))
				continue
			}
			newLevel := utils.GetLogLevel(validated)
			logfilter.SetLevel(newLevel)
			s.logger.Info("Log level changed via socket", "level", validated)
			s.sendResponse(conn, id, map[string]any{"level": validated})

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

// setLightProperty sets a single property on a light by name.
func (s *Server) setLightProperty(ctx context.Context, lightID, property string, value any) error {
	switch property {
	case "on":
		onVal, ok := value.(bool)
		if !ok {
			return fmt.Errorf("invalid value type for 'on', expected boolean")
		}
		return s.lights.SetLightState(ctx, lightID, keylight.OnValue(onVal))
	case "brightness":
		bVal, ok := value.(float64)
		if !ok {
			return fmt.Errorf("invalid value type for 'brightness', expected number")
		}
		return s.lights.SetLightBrightness(ctx, lightID, int(bVal))
	case "temperature":
		tVal, ok := value.(float64)
		if !ok {
			return fmt.Errorf("invalid value type for 'temperature', expected number")
		}
		return s.lights.SetLightTemperature(ctx, lightID, int(tVal))
	default:
		return fmt.Errorf("unknown property: %s", property)
	}
}

// setGroupProperty sets a single property on a group by name.
func (s *Server) setGroupProperty(ctx context.Context, groupID, property string, value any) error {
	switch property {
	case "on":
		onVal, ok := value.(bool)
		if !ok {
			return fmt.Errorf("invalid value type for 'on', expected boolean")
		}
		return s.groups.SetGroupState(ctx, groupID, onVal)
	case "brightness":
		bVal, ok := value.(float64)
		if !ok {
			return fmt.Errorf("invalid value type for 'brightness', expected number")
		}
		return s.groups.SetGroupBrightness(ctx, groupID, int(bVal))
	case "temperature":
		tVal, ok := value.(float64)
		if !ok {
			return fmt.Errorf("invalid value type for 'temperature', expected number")
		}
		return s.groups.SetGroupTemperature(ctx, groupID, int(tVal))
	default:
		return fmt.Errorf("unknown property: %s", property)
	}
}

// stringFromMap extracts a string from a map[string]any, returning "" if missing or wrong type.
func stringFromMap(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

// boolFromMap extracts a bool from a map[string]any, returning false if missing or wrong type.
func boolFromMap(m map[string]any, key string) bool {
	v, _ := m[key].(bool)
	return v
}
