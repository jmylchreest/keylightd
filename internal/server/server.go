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

// VersionInfo holds build version metadata for the running daemon.
type VersionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
}

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
	versionInfo   VersionInfo
}

// New creates a new server instance.
func New(logger *slog.Logger, cfg *config.Config, lightManager keylight.LightManager, vi VersionInfo) *Server {
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
		versionInfo:   vi,
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
	if err := os.MkdirAll(sockDir, 0755); err != nil { //nolint:gosec // G301: socket dir needs to be accessible
		return fmt.Errorf("failed to create socket directory %s: %w", sockDir, err)
	}

	// Check for an existing socket file
	if _, err := os.Stat(s.socketPath); err == nil {
		// Socket file exists — check if another instance is listening
		conn, dialErr := (&net.Dialer{Timeout: 500 * time.Millisecond}).DialContext(context.Background(), "unix", s.socketPath)
		if dialErr == nil {
			// Connection succeeded: another instance is running
			_ = conn.Close()
			return fmt.Errorf("another keylightd instance is already running (socket %s is active)", s.socketPath)
		}
		// Connection failed: stale socket file from a crashed instance, safe to remove
		s.logger.Debug("Removing stale socket file", "path", s.socketPath)
		if err := os.Remove(s.socketPath); err != nil {
			return fmt.Errorf("failed to remove existing socket file %s: %w", s.socketPath, err)
		}
	}

	// Start listening on Unix socket
	var err error
	s.listener, err = (&net.ListenConfig{}).Listen(context.Background(), "unix", s.socketPath)
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
			HealthCheck:  handlers.HealthCheck,
			VersionCheck: handlers.NewVersionCheck(s.versionInfo.Version, s.versionInfo.Commit, s.versionInfo.BuildDate),
			Light:        lightHandler,
			Group:        groupHandler,
			APIKey:       apiKeyHandler,
			Logging:      loggingHandler,
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
		_ = s.listener.Close() // Close the socket listener to stop accepting new connections
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

// socketRequest holds the parsed fields of an incoming socket request.
type socketRequest struct {
	conn   net.Conn
	ctx    context.Context
	id     string
	data   map[string]any
	action string
}

// socketActionResult indicates how the connection loop should proceed after an action handler.
type socketActionResult int

const (
	socketContinue socketActionResult = iota // keep reading next request
	socketReturn                             // close connection
)

// socketActionHandler processes a single socket action and returns whether
// the connection loop should continue or return.
type socketActionHandler func(s *Server, r socketRequest) socketActionResult

// socketActions maps action names to their handler functions.
var socketActions = map[string]socketActionHandler{
	"ping":                       (*Server).handlePing,
	"list_lights":                (*Server).handleListLights,
	"get_light":                  (*Server).handleGetLight,
	"set_light_state":            (*Server).handleSetLightState,
	"create_group":               (*Server).handleCreateGroup,
	"delete_group":               (*Server).handleDeleteGroup,
	"get_group":                  (*Server).handleGetGroup,
	"list_groups":                (*Server).handleListGroups,
	"set_group_lights":           (*Server).handleSetGroupLights,
	"set_group_state":            (*Server).handleSetGroupState,
	"apikey_add":                 (*Server).handleAPIKeyAdd,
	"apikey_list":                (*Server).handleAPIKeyList,
	"apikey_delete":              (*Server).handleAPIKeyDelete,
	"apikey_set_disabled_status": (*Server).handleAPIKeySetDisabledStatus,
	"subscribe_events":           (*Server).handleSubscribeEvents,
	"health":                     (*Server).handleHealth,
	"list_filters":               (*Server).handleListFilters,
	"set_filters":                (*Server).handleSetFilters,
	"set_level":                  (*Server).handleSetLevel,
	"version":                    (*Server).handleVersion,
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("panic in connection handler", "recover", r)
		}
	}()

	//nolint:misspell // British spelling intentional
	// Create a context that is cancelled when the server shuts down
	ctx, cancel := context.WithCancel(s.rootCtx)
	defer cancel()

	go func() {
		select {
		case <-s.shutdown:
			cconn, ok := conn.(*net.UnixConn)
			if ok {
				if err := cconn.CloseRead(); err != nil {
					s.logger.Warn("socket: CloseRead failed", "error", err)
				} // Force connection to unblock for shutdown
			}
			cancel() // cancel the context for this connection
		case <-ctx.Done(): //nolint:misspell // if connection context is cancelled (e.g. normal close)
			return
		}
	}()

	reader := bufio.NewReader(conn)

	for {
		select {
		case <-ctx.Done(): //nolint:misspell // Check if context was cancelled (e.g. server shutdown)
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

		r := socketRequest{conn: conn, ctx: ctx, id: id, data: data, action: action}

		handler, ok := socketActions[action]
		if !ok {
			s.logger.Warn("received unknown action", "action", action)
			s.sendError(conn, id, "unknown action: "+action)
			continue
		}
		if result := handler(s, r); result == socketReturn {
			return
		}
	}
}

func (s *Server) handlePing(r socketRequest) socketActionResult {
	s.sendResponse(r.conn, r.id, map[string]any{"message": "pong"})
	return socketContinue
}

func (s *Server) handleListLights(r socketRequest) socketActionResult {
	lights := s.lights.GetLights()
	result := make(map[string]any, len(lights))
	for id, light := range lights {
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
	s.sendResponse(r.conn, r.id, map[string]any{"lights": result})
	return socketContinue
}

func (s *Server) handleGetLight(r socketRequest) socketActionResult {
	lightID, _ := r.data["id"].(string)
	if lightID == "" {
		s.sendError(r.conn, r.id, "missing light ID for get_light")
		return socketContinue
	}
	light, err := s.lights.GetLight(r.ctx, lightID)
	if err != nil {
		s.sendError(r.conn, r.id, fmt.Sprintf("failed to get light %s: %s", lightID, err))
		return socketContinue
	}
	b, err := json.Marshal(light)
	if err != nil {
		s.logger.Error("Failed to marshal light for socket response", "id", lightID, "error", err)
		s.sendError(r.conn, r.id, "internal error marshaling light")
		return socketContinue
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		s.logger.Error("Failed to unmarshal light for socket response", "id", lightID, "error", err)
		s.sendError(r.conn, r.id, "internal error unmarshaling light")
		return socketContinue
	}
	s.sendResponse(r.conn, r.id, map[string]any{"light": m})
	return socketContinue
}

func (s *Server) handleSetLightState(r socketRequest) socketActionResult {
	lightID, _ := r.data["id"].(string)
	if lightID == "" {
		s.sendError(r.conn, r.id, "missing id for set_light_state")
		return socketContinue
	}

	// Support both single-property (property+value) and multi-property (on, brightness, temperature) modes.
	property, _ := r.data["property"].(string)
	value := r.data["value"]

	var errs []string
	if property != "" && value != nil {
		// Legacy single-property mode
		if err := s.setLightProperty(r.ctx, lightID, property, value); err != nil {
			s.sendError(r.conn, r.id, fmt.Sprintf("failed to set light %s state %s: %s", lightID, property, err))
			return socketContinue
		}
	} else {
		// Multi-property mode: check for on, brightness, temperature in data
		set := false
		if onVal, ok := r.data["on"]; ok {
			set = true
			if err := s.setLightProperty(r.ctx, lightID, "on", onVal); err != nil {
				errs = append(errs, err.Error())
			}
		}
		if bVal, ok := r.data["brightness"]; ok {
			set = true
			if err := s.setLightProperty(r.ctx, lightID, "brightness", bVal); err != nil {
				errs = append(errs, err.Error())
			}
		}
		if tVal, ok := r.data["temperature"]; ok {
			set = true
			if err := s.setLightProperty(r.ctx, lightID, "temperature", tVal); err != nil {
				errs = append(errs, err.Error())
			}
		}
		if !set {
			s.sendError(r.conn, r.id, "missing property/value or on/brightness/temperature for set_light_state")
			return socketContinue
		}
	}

	if len(errs) > 0 {
		s.sendError(r.conn, r.id, fmt.Sprintf("failed to set light %s state: %s", lightID, strings.Join(errs, "; ")))
		return socketContinue
	}
	s.sendResponse(r.conn, r.id, map[string]any{"status": "ok"})
	return socketContinue
}

func (s *Server) handleCreateGroup(r socketRequest) socketActionResult {
	name, _ := r.data["name"].(string)
	lightIDsReq, _ := r.data["lights"].([]any)
	lightIDs := make([]string, len(lightIDsReq))
	for i, v := range lightIDsReq {
		lightIDs[i], _ = v.(string)
	}
	if name == "" {
		s.sendError(r.conn, r.id, "missing name for create_group")
		return socketContinue
	}
	grp, err := s.groups.CreateGroup(r.ctx, name, lightIDs)
	if err != nil {
		s.sendError(r.conn, r.id, fmt.Sprintf("failed to create group: %s", err))
		return socketContinue
	}
	s.sendResponse(r.conn, r.id, map[string]any{"group": grp})
	return socketContinue
}

func (s *Server) handleDeleteGroup(r socketRequest) socketActionResult {
	groupID, _ := r.data["id"].(string)
	if groupID == "" {
		s.sendError(r.conn, r.id, "missing group ID for delete_group")
		return socketContinue
	}
	if err := s.groups.DeleteGroup(groupID); err != nil {
		s.sendError(r.conn, r.id, fmt.Sprintf("failed to delete group %s: %s", groupID, err))
		return socketContinue
	}
	s.sendResponse(r.conn, r.id, map[string]any{"status": "ok"})
	return socketContinue
}

func (s *Server) handleGetGroup(r socketRequest) socketActionResult {
	groupID, _ := r.data["id"].(string)
	if groupID == "" {
		s.sendError(r.conn, r.id, "missing group ID for get_group")
		return socketContinue
	}
	grp, err := s.groups.GetGroup(groupID)
	if err != nil {
		s.sendError(r.conn, r.id, fmt.Sprintf("failed to get group %s: %s", groupID, err))
		return socketContinue
	}
	lights := grp.Lights
	if lights == nil {
		lights = []string{}
	}
	s.sendResponse(r.conn, r.id, map[string]any{"group": map[string]any{"id": grp.ID, "name": grp.Name, "lights": lights}})
	return socketContinue
}

func (s *Server) handleListGroups(r socketRequest) socketActionResult {
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
	s.sendResponse(r.conn, r.id, map[string]any{"groups": groupList})
	return socketContinue
}

func (s *Server) handleSetGroupLights(r socketRequest) socketActionResult {
	groupID, _ := r.data["id"].(string)
	lightIDsReq, _ := r.data["lights"].([]any)
	lightIDs := make([]string, len(lightIDsReq))
	for i, v := range lightIDsReq {
		lightIDs[i], _ = v.(string)
	}
	if groupID == "" {
		s.sendError(r.conn, r.id, "missing group ID for set_group_lights")
		return socketContinue
	}
	if err := s.groups.SetGroupLights(r.ctx, groupID, lightIDs); err != nil {
		s.sendError(r.conn, r.id, fmt.Sprintf("failed to set lights for group %s: %s", groupID, err))
		return socketContinue
	}
	s.sendResponse(r.conn, r.id, map[string]any{"status": "ok"})
	return socketContinue
}

func (s *Server) handleSetGroupState(r socketRequest) socketActionResult {
	groupKeys, _ := r.data["id"].(string)
	if groupKeys == "" {
		s.sendError(r.conn, r.id, "missing id for set_group_state")
		return socketContinue
	}
	matchedGroups, notFound := s.groups.GetGroupsByKeys(groupKeys)
	if len(matchedGroups) == 0 {
		s.sendError(r.conn, r.id, "no groups found for: "+strings.Join(notFound, ", "))
		return socketContinue
	}

	// Build list of properties to set.
	// Support both single-property (property+value) and multi-property (on, brightness, temperature).
	type propVal struct {
		name  string
		value any
	}
	var props []propVal

	property, _ := r.data["property"].(string)
	value := r.data["value"]
	if property != "" && value != nil {
		props = append(props, propVal{property, value})
	} else {
		if v, ok := r.data["on"]; ok {
			props = append(props, propVal{"on", v})
		}
		if v, ok := r.data["brightness"]; ok {
			props = append(props, propVal{"brightness", v})
		}
		if v, ok := r.data["temperature"]; ok {
			props = append(props, propVal{"temperature", v})
		}
	}
	if len(props) == 0 {
		s.sendError(r.conn, r.id, "missing property/value or on/brightness/temperature for set_group_state")
		return socketContinue
	}

	var errs []string
	for _, grp := range matchedGroups {
		for _, p := range props {
			if err := s.setGroupProperty(r.ctx, grp.ID, p.name, p.value); err != nil {
				errs = append(errs, fmt.Sprintf("group %s: %s", grp.ID, err))
			}
		}
	}
	if len(errs) > 0 {
		s.sendResponse(r.conn, r.id, map[string]any{"status": "partial", "errors": errs})
		return socketContinue
	}
	s.sendResponse(r.conn, r.id, map[string]any{"status": "ok"})
	return socketContinue
}

func (s *Server) handleAPIKeyAdd(r socketRequest) socketActionResult {
	name, _ := r.data["name"].(string)
	expiresInStr, _ := r.data["expires_in"].(string)
	var expiresIn time.Duration
	if expiresInStr != "" {
		// Try Go duration string first (e.g., "720h", "30m"), then seconds-as-float for backward compat
		d, err := time.ParseDuration(expiresInStr)
		if err != nil {
			expiresInSecs, err2 := strconv.ParseFloat(expiresInStr, 64)
			if err2 != nil {
				s.sendError(r.conn, r.id, fmt.Sprintf("invalid expires_in format (use Go duration like '720h' or seconds): %s", err))
				return socketContinue
			}
			expiresIn = time.Duration(expiresInSecs * float64(time.Second))
		} else {
			expiresIn = d
		}
	}
	if name == "" {
		s.sendError(r.conn, r.id, "missing name for apikey_add")
		return socketContinue
	}
	apiKey, err := s.apikeyManager.CreateAPIKey(name, expiresIn)
	if err != nil {
		s.sendError(r.conn, r.id, fmt.Sprintf("failed to create API key: %s", err))
		return socketContinue
	}
	// Construct a map with lowercase keys for the client
	apiKeyResponse := map[string]any{
		"name":         apiKey.Name,
		"key":          apiKey.Key,
		"created_at":   apiKey.CreatedAt.Format(time.RFC3339Nano),
		"expires_at":   apiKey.ExpiresAt.Format(time.RFC3339Nano),
		"last_used_at": apiKey.LastUsedAt.Format(time.RFC3339Nano),
		"disabled":     apiKey.IsDisabled(),
	}
	s.sendResponse(r.conn, r.id, map[string]any{"status": "ok", "key": apiKeyResponse})
	return socketContinue
}

func (s *Server) handleAPIKeyList(r socketRequest) socketActionResult {
	keys := s.apikeyManager.ListAPIKeys()
	responseKeys := make([]map[string]any, len(keys))
	for i, k := range keys {
		responseKeys[i] = map[string]any{
			"name":         k.Name,
			"key":          k.Key,
			"created_at":   k.CreatedAt.Format(time.RFC3339Nano),
			"expires_at":   k.ExpiresAt.Format(time.RFC3339Nano),
			"last_used_at": k.LastUsedAt.Format(time.RFC3339Nano),
			"disabled":     k.IsDisabled(),
		}
	}
	s.sendResponse(r.conn, r.id, map[string]any{"status": "ok", "keys": responseKeys})
	return socketContinue
}

func (s *Server) handleAPIKeyDelete(r socketRequest) socketActionResult {
	key, _ := r.data["key"].(string)
	if key == "" {
		s.sendError(r.conn, r.id, "missing key for apikey_delete")
		return socketContinue
	}
	if err := s.apikeyManager.DeleteAPIKey(key); err != nil {
		s.sendError(r.conn, r.id, fmt.Sprintf("failed to delete API key: %s", err))
		return socketContinue
	}
	s.sendResponse(r.conn, r.id, map[string]any{"status": "ok"})
	return socketContinue
}

func (s *Server) handleAPIKeySetDisabledStatus(r socketRequest) socketActionResult {
	keyOrName, _ := r.data["key_or_name"].(string)

	if keyOrName == "" {
		s.sendError(r.conn, r.id, "missing key_or_name for apikey_set_disabled_status")
		return socketContinue
	}

	// Accept disabled as bool or string for compatibility with both HTTP and legacy socket clients
	var disabled bool
	switch v := r.data["disabled"].(type) {
	case bool:
		disabled = v
	case string:
		var err error
		disabled, err = strconv.ParseBool(v)
		if err != nil {
			s.sendError(r.conn, r.id, fmt.Sprintf("invalid boolean value for disabled state: %s", err))
			return socketContinue
		}
	default:
		s.sendError(r.conn, r.id, "missing or invalid disabled state for apikey_set_disabled_status")
		return socketContinue
	}

	updatedKey, err := s.apikeyManager.SetAPIKeyDisabledStatus(keyOrName, disabled)
	if err != nil {
		s.sendError(r.conn, r.id, fmt.Sprintf("failed to set API key disabled status: %s", err))
		return socketContinue
	}
	s.sendResponse(r.conn, r.id, map[string]any{"status": "ok", "key": updatedKey})
	return socketContinue
}

func (s *Server) handleSubscribeEvents(r socketRequest) socketActionResult {
	// Acknowledge the subscription, then switch to streaming mode.
	s.sendResponse(r.conn, r.id, map[string]any{"subscribed": true})
	s.handleEventSubscription(r.ctx, r.conn)
	return socketReturn // Connection is done after event streaming ends
}

func (s *Server) handleHealth(r socketRequest) socketActionResult {
	s.sendResponse(r.conn, r.id, map[string]any{"health": "ok"})
	return socketContinue
}

func (s *Server) handleListFilters(r socketRequest) socketActionResult {
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

	s.sendResponse(r.conn, r.id, map[string]any{
		"level":   handlers.LevelToString(level),
		"filters": filterList,
	})
	return socketContinue
}

func (s *Server) handleSetFilters(r socketRequest) socketActionResult {
	filtersRaw, _ := r.data["filters"].([]any)
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
		s.sendError(r.conn, r.id, "invalid filters: "+logging.FormatErrors(errs))
		return socketContinue
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
	s.sendResponse(r.conn, r.id, map[string]any{
		"level":   handlers.LevelToString(logfilter.GetLevel()),
		"filters": resultFilters,
	})
	return socketContinue
}

func (s *Server) handleSetLevel(r socketRequest) socketActionResult {
	level, _ := r.data["level"].(string)
	if level == "" {
		s.sendError(r.conn, r.id, "missing level for set_level")
		return socketContinue
	}
	validated := utils.ValidateLogLevel(level)
	if validated != level {
		s.sendError(r.conn, r.id, fmt.Sprintf("invalid log level %q; must be debug, info, warn, or error", level))
		return socketContinue
	}
	newLevel := utils.GetLogLevel(validated)
	logfilter.SetLevel(newLevel)
	s.logger.Info("Log level changed via socket", "level", validated)
	s.sendResponse(r.conn, r.id, map[string]any{"level": validated})
	return socketContinue
}

func (s *Server) handleVersion(r socketRequest) socketActionResult {
	s.sendResponse(r.conn, r.id, map[string]any{
		"version":    s.versionInfo.Version,
		"commit":     s.versionInfo.Commit,
		"build_date": s.versionInfo.BuildDate,
	})
	return socketContinue
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

// handleEventSubscription streams events to a socket client until the connection
// closes or the server shuts down. Events are sent as newline-delimited JSON,
// using the same events.Event format as the WebSocket endpoint.
func (s *Server) handleEventSubscription(ctx context.Context, conn net.Conn) {
	eventCh := make(chan []byte, 64)

	// Subscribe to the event bus
	unsub := s.eventBus.Subscribe(func(e events.Event) {
		data, err := json.Marshal(e)
		if err != nil {
			s.logger.Error("socket events: failed to marshal event", "error", err)
			return
		}
		select {
		case eventCh <- data:
		default:
			s.logger.Warn("socket events: client buffer full, dropping event")
		}
	})
	defer unsub()

	// Watch for client disconnect in a goroutine.
	// When the client closes, the read will return an error and we cancel.
	connCtx, connCancel := context.WithCancel(ctx)
	defer connCancel()

	go func() {
		buf := make([]byte, 1)
		for {
			_, err := conn.Read(buf)
			if err != nil {
				connCancel()
				return
			}
		}
	}()

	s.logger.Info("socket events: client subscribed")

	for {
		select {
		case <-connCtx.Done():
			s.logger.Info("socket events: client disconnected")
			return
		case data := <-eventCh:
			// Append newline for NDJSON framing
			data = append(data, '\n')
			if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
				s.logger.Warn("socket events: SetWriteDeadline failed", "error", err)
			}
			if _, err := conn.Write(data); err != nil {
				s.logger.Debug("socket events: write failed", "error", err)
				return
			}
		}
	}
}

// setLightProperty sets a single property on a light by name.
func (s *Server) setLightProperty(ctx context.Context, lightID, property string, value any) error {
	switch property {
	case "on":
		onVal, ok := value.(bool)
		if !ok {
			return errors.New("invalid value type for 'on', expected boolean")
		}
		return s.lights.SetLightState(ctx, lightID, keylight.OnValue(onVal))
	case "brightness":
		bVal, ok := value.(float64)
		if !ok {
			return errors.New("invalid value type for 'brightness', expected number")
		}
		return s.lights.SetLightBrightness(ctx, lightID, int(bVal))
	case "temperature":
		tVal, ok := value.(float64)
		if !ok {
			return errors.New("invalid value type for 'temperature', expected number")
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
			return errors.New("invalid value type for 'on', expected boolean")
		}
		return s.groups.SetGroupState(ctx, groupID, onVal)
	case "brightness":
		bVal, ok := value.(float64)
		if !ok {
			return errors.New("invalid value type for 'brightness', expected number")
		}
		return s.groups.SetGroupBrightness(ctx, groupID, int(bVal))
	case "temperature":
		tVal, ok := value.(float64)
		if !ok {
			return errors.New("invalid value type for 'temperature', expected number")
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
