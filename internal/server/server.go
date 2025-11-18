package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
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

	"github.com/jmylchreest/keylightd/internal/apikey"
	"github.com/jmylchreest/keylightd/internal/config"
	"github.com/jmylchreest/keylightd/internal/group"
	"github.com/jmylchreest/keylightd/pkg/keylight"
)

// --- Added loggingResponseWriter ---
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{w, http.StatusOK} // Default to 200 OK
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// --- End added loggingResponseWriter ---

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
		mux := http.NewServeMux()

		// API Key Management Endpoints
		mux.Handle("POST /api/v1/apikeys", s.authMiddleware(s.handleAPIKeyCreate()))
		mux.Handle("GET /api/v1/apikeys", s.authMiddleware(s.handleAPIKeyList()))
		mux.Handle("DELETE /api/v1/apikeys/{key}", s.authMiddleware(s.handleAPIKeyDelete()))
		mux.Handle("PUT /api/v1/apikeys/{key}/disabled", s.authMiddleware(s.handleAPIKeySetDisabled()))

		// Light Endpoints
		mux.Handle("GET /api/v1/lights", s.authMiddleware(s.handleLightsList()))
		mux.Handle("GET /api/v1/lights/{id}", s.authMiddleware(s.handleLightGet()))
		mux.Handle("POST /api/v1/lights/{id}/state", s.authMiddleware(s.handleLightSetState()))

		// Group Endpoints
		mux.Handle("GET /api/v1/groups", s.authMiddleware(s.handleGroupsList()))
		mux.Handle("POST /api/v1/groups", s.authMiddleware(s.handleGroupCreate()))
		mux.Handle("GET /api/v1/groups/{id}", s.authMiddleware(s.handleGroupGet()))
		mux.Handle("DELETE /api/v1/groups/{id}", s.authMiddleware(s.handleGroupDelete()))
		mux.Handle("PUT /api/v1/groups/{id}/lights", s.authMiddleware(s.handleGroupSetLights()))
		mux.Handle("PUT /api/v1/groups/{id}/state", s.authMiddleware(s.handleGroupSetState()))

		s.httpServer = &http.Server{
			Addr:    s.cfg.Config.API.ListenAddress,
			Handler: s.loggingMiddleware(mux), // Apply logging middleware here
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
		ctx, cancel := context.WithTimeout(s.rootCtx, 5*time.Second)
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
	encoder := json.NewEncoder(conn)

	for {
		select {
		case <-ctx.Done(): // Check if context was cancelled (e.g. server shutdown)
			return
		default:
			// Proceed with reading
		}

		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err.Error() == "EOF" || strings.Contains(err.Error(), "use of closed network connection") {
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
				return
			}
			light, err := s.lights.GetLight(ctx, lightID)
			if err != nil {
				s.sendError(conn, id, fmt.Sprintf("failed to get light %s: %s", lightID, err))
				return
			}
			// Marshal to JSON and then unmarshal to map[string]any
			b, err := json.Marshal(light)
			if err != nil {
				s.logger.Error("Failed to marshal light for socket response", "id", lightID, "error", err)
				s.sendError(conn, id, "internal error marshaling light")
				return
			}
			var m map[string]any
			if err := json.Unmarshal(b, &m); err != nil {
				s.logger.Error("Failed to unmarshal light for socket response", "id", lightID, "error", err)
				s.sendError(conn, id, "internal error unmarshaling light")
				return
			}
			s.sendResponse(conn, id, map[string]any{"light": m})
		case "set_light_state":
			// Always extract from 'data' map
			lightID, _ := data["id"].(string)
			property, _ := data["property"].(string)
			value := data["value"] // Keep as any for flexibility

			if lightID == "" || property == "" || value == nil {
				s.sendError(conn, id, "missing id, property, or value for set_light_state")
				return
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
				return
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
				return
			}
			group, err := s.groups.CreateGroup(ctx, name, lightIDs)
			if err != nil {
				s.sendError(conn, id, fmt.Sprintf("failed to create group: %s", err))
				return
			}
			s.sendResponse(conn, id, map[string]any{"group": group})

		case "delete_group":
			groupID, _ := data["id"].(string)
			if groupID == "" {
				s.sendError(conn, id, "missing group ID for delete_group")
				return
			}
			if err := s.groups.DeleteGroup(groupID); err != nil {
				s.sendError(conn, id, fmt.Sprintf("failed to delete group %s: %s", groupID, err))
				return
			}
			s.sendResponse(conn, id, map[string]any{"status": "ok"})

		case "get_group":
			groupID, _ := data["id"].(string)
			if groupID == "" {
				s.sendError(conn, id, "missing group ID for get_group")
				return
			}
			group, err := s.groups.GetGroup(groupID)
			if err != nil {
				s.sendError(conn, id, fmt.Sprintf("failed to get group %s: %s", groupID, err))
				return
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
				return
			}
			if err := s.groups.SetGroupLights(ctx, groupID, lightIDs); err != nil {
				s.sendError(conn, id, fmt.Sprintf("failed to set lights for group %s: %s", groupID, err))
				return
			}
			s.sendResponse(conn, id, map[string]any{"status": "ok"})

		case "set_group_state":
			groupKeys, _ := data["id"].(string)
			property, _ := data["property"].(string)
			value := data["value"]
			if groupKeys == "" || property == "" || value == nil {
				s.sendError(conn, id, "missing id, property, or value for set_group_state")
				return
			}
			matchedGroups, notFound := s.groups.GetGroupsByKeys(groupKeys)
			if len(matchedGroups) == 0 {
				s.sendError(conn, id, "no groups found for: "+strings.Join(notFound, ", "))
				return
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
					return
				}
				expiresIn = time.Duration(expiresInSecs * float64(time.Second))
			}
			if name == "" {
				s.sendError(conn, id, "missing name for apikey_add")
				return
			}
			apiKey, err := s.apikeyManager.CreateAPIKey(name, expiresIn)
			if err != nil {
				s.sendError(conn, id, fmt.Sprintf("failed to create API key: %s", err))
				return
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
				return
			}
			if err := s.apikeyManager.DeleteAPIKey(key); err != nil {
				s.sendError(conn, id, fmt.Sprintf("failed to delete API key: %s", err))
				return
			}
			s.sendResponse(conn, id, map[string]any{"status": "ok"})

		case "apikey_set_disabled_status":
			keyOrName, _ := data["key_or_name"].(string) // Corrected: use data, not req
			disabledStr, _ := data["disabled"].(string)  // Corrected: use data, not req

			if keyOrName == "" {
				s.sendError(conn, id, "missing key_or_name for apikey_set_disabled_status")
				return
			}
			if disabledStr == "" {
				s.sendError(conn, id, "missing disabled state for apikey_set_disabled_status")
				return
			}

			disabled, err := strconv.ParseBool(disabledStr)
			if err != nil {
				s.sendError(conn, id, fmt.Sprintf("invalid boolean value for disabled state: %s", err))
				return
			}

			updatedKey, err := s.apikeyManager.SetAPIKeyDisabledStatus(keyOrName, disabled)
			if err != nil {
				s.sendError(conn, id, fmt.Sprintf("failed to set API key disabled status: %s", err))
				return
			}
			s.sendResponse(conn, id, map[string]any{"status": "ok", "key": updatedKey})

		default:
			s.logger.Warn("received unknown action", "action", action)
			encoder.Encode(map[string]any{"id": id, "error": "unknown action: " + action})
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

// HTTP Handlers

// authMiddleware performs API key authentication.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("Authorization")
		const bearerPrefix = "Bearer "
		if strings.HasPrefix(apiKey, bearerPrefix) {
			apiKey = apiKey[len(bearerPrefix):]
		} else {
			apiKey = r.Header.Get("X-API-Key")
		}

		if apiKey == "" {
			s.logger.Warn("API key missing")
			http.Error(w, "Unauthorized: API key required", http.StatusUnauthorized)
			return
		}

		validKey, err := s.apikeyManager.ValidateAPIKey(apiKey)
		if err != nil { // Corrected: check err != nil
			s.logger.Warn("Invalid API key used", "key_prefix", keyPrefix(apiKey), "error", err)
			http.Error(w, fmt.Sprintf("Unauthorized: %s", err.Error()), http.StatusUnauthorized)
			return
		}

		s.logger.Debug("Authenticated API key", "name", validKey.Name, "key_prefix", keyPrefix(validKey.Key))
		next.ServeHTTP(w, r)
	})
}

func keyPrefix(key string) string {
	if len(key) >= 4 {
		return key[:4]
	}
	return key
}

// APIKeyRequest represents the request body for creating an API key.
type APIKeyRequest struct {
	Name      string `json:"name"`
	ExpiresIn string `json:"expires_in,omitempty"` // Duration string like "720h", "30d"
}

// APIKeyResponse represents a created API key (omits the full key string for security).
type APIKeyResponse struct {
	ID        string    `json:"id"` // This is actually the key itself, or just the name?
	Name      string    `json:"name"`
	Key       string    `json:"key,omitempty"` // Only present on creation, otherwise omitted
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// LightStateRequest represents the request body for setting a light's state.
type LightStateRequest struct {
	On          *bool `json:"on,omitempty"`          // Pointer to distinguish between not set and false
	Brightness  *int  `json:"brightness,omitempty"`  // Pointer for optional field
	Temperature *int  `json:"temperature,omitempty"` // Pointer for optional field
}

// GroupSetLightsRequest represents the request to set lights in a group
type GroupSetLightsRequest struct {
	LightIDs []string `json:"light_ids"`
}

func (s *Server) handleAPIKeyCreate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req APIKeyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if req.Name == "" {
			http.Error(w, "API key name is required", http.StatusBadRequest)
			return
		}

		var expiresInDuration time.Duration
		if req.ExpiresIn != "" {
			var err error
			expiresInDuration, err = time.ParseDuration(req.ExpiresIn)
			if err != nil {
				http.Error(w, fmt.Sprintf("Invalid expires_in duration: %s", err), http.StatusBadRequest)
				return
			}
		}

		// Permissions not handled via HTTP API yet, pass nil
		newKey, err := s.apikeyManager.CreateAPIKey(req.Name, expiresInDuration)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create API key: %s", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		writeJSONResponse(w, APIKeyResponse{
			ID:        newKey.Key, // Using Key as ID
			Name:      newKey.Name,
			Key:       newKey.Key, // Show full key on creation only
			CreatedAt: newKey.CreatedAt,
			ExpiresAt: newKey.ExpiresAt,
		})
	}
}

func (s *Server) handleAPIKeyList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		keys := s.apikeyManager.ListAPIKeys() // Returns []config.APIKey
		responseKeys := make([]APIKeyResponse, len(keys))
		for i, k := range keys {
			responseKeys[i] = APIKeyResponse{
				ID:   k.Key, // Using Key as ID
				Name: k.Name,
				// Key field is omitted here for security (not on creation)
				CreatedAt: k.CreatedAt,
				ExpiresAt: k.ExpiresAt,
			}
		}
		writeJSONResponse(w, responseKeys)
	}
}

func (s *Server) handleAPIKeyDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		if key == "" {
			http.Error(w, "API key is required in path", http.StatusBadRequest)
			return
		}

		if err := s.apikeyManager.DeleteAPIKey(key); err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, "API key not found", http.StatusNotFound)
			} else {
				http.Error(w, fmt.Sprintf("Failed to delete API key: %s", err), http.StatusInternalServerError)
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleAPIKeySetDisabled handles PUT /api/v1/apikeys/{key}/disabled
func (s *Server) handleAPIKeySetDisabled() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		keyOrName := r.PathValue("key") // key could be the key string or its name
		if keyOrName == "" {
			http.Error(w, "API key/name is required in path", http.StatusBadRequest)
			return
		}

		var payload struct {
			Disabled bool `json:"disabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request body, expected {\"disabled\": true/false}", http.StatusBadRequest)
			return
		}

		updatedKey, err := s.apikeyManager.SetAPIKeyDisabledStatus(keyOrName, payload.Disabled)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, "API key not found", http.StatusNotFound)
			} else {
				http.Error(w, fmt.Sprintf("Failed to update API key: %s", err), http.StatusInternalServerError)
			}
			return
		}

		// Return the updated key details (omitting the full key string for security)
		writeJSONResponse(w, APIKeyResponse{
			ID:        updatedKey.Key,
			Name:      updatedKey.Name,
			CreatedAt: updatedKey.CreatedAt,
			ExpiresAt: updatedKey.ExpiresAt,
			// Disabled status is implicitly updated, not typically part of response here unless desired.
		})
	}
}

func (s *Server) handleLightsList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lights := s.lights.GetLights()
		writeJSONResponse(w, lights)
	}
}

func (s *Server) handleLightGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lightID := r.PathValue("id")
		light, err := s.lights.GetLight(r.Context(), lightID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Light not found: %s", err), http.StatusNotFound)
			return
		}
		writeJSONResponse(w, light)
	}
}

func (s *Server) handleLightSetState() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lightID := r.PathValue("id")
		var reqBody LightStateRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		var errs []string

		if reqBody.On != nil {
			if err := s.lights.SetLightState(r.Context(), lightID, keylight.OnValue(*reqBody.On)); err != nil {
				errs = append(errs, err.Error())
			}
		}
		if reqBody.Brightness != nil {
			if err := s.lights.SetLightState(r.Context(), lightID, keylight.BrightnessValue(*reqBody.Brightness)); err != nil {
				errs = append(errs, err.Error())
			}
		}
		if reqBody.Temperature != nil {
			if err := s.lights.SetLightState(r.Context(), lightID, keylight.TemperatureValue(*reqBody.Temperature)); err != nil {
				errs = append(errs, err.Error())
			}
		}

		if len(errs) > 0 {
			http.Error(w, fmt.Sprintf("Error(s) setting light state: %s", strings.Join(errs, "; ")), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		writeJSONResponse(w, map[string]string{"status": "ok"})
	}
}

func (s *Server) handleGroupsList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groups := s.groups.GetGroups()
		writeJSONResponse(w, groups)
	}
}

func (s *Server) handleGroupCreate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var reqBody struct {
			Name     string   `json:"name"`
			LightIDs []string `json:"light_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if reqBody.Name == "" {
			http.Error(w, "Group name is required", http.StatusBadRequest)
			return
		}

		group, err := s.groups.CreateGroup(r.Context(), reqBody.Name, reqBody.LightIDs)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create group: %s", err), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSONResponse(w, group)
	}
}

func (s *Server) handleGroupGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID := r.PathValue("id")
		group, err := s.groups.GetGroup(groupID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Group not found: %s", err), http.StatusNotFound)
			return
		}
		writeJSONResponse(w, group)
	}
}

func (s *Server) handleGroupDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID := r.PathValue("id")
		if err := s.groups.DeleteGroup(groupID); err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, "Group not found", http.StatusNotFound)
			} else {
				http.Error(w, fmt.Sprintf("Failed to delete group: %s", err), http.StatusInternalServerError)
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) handleGroupSetLights() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID := r.PathValue("id")
		var reqBody GroupSetLightsRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if err := s.groups.SetGroupLights(r.Context(), groupID, reqBody.LightIDs); err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, "Group or light not found", http.StatusNotFound)
			} else {
				http.Error(w, fmt.Sprintf("Failed to set group lights: %s", err), http.StatusInternalServerError)
			}
			return
		}
		writeJSONResponse(w, map[string]string{"status": "ok"})
	}
}

func (s *Server) handleGroupSetState() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupParam := r.PathValue("id") // e.g., "office" or "group-1,office"
		groupKeys := strings.Split(groupParam, ",")
		var matchedGroups []*group.Group
		var notFound []string
		groupSeen := make(map[string]bool) // To avoid duplicate group actions

		for _, key := range groupKeys {
			key = strings.TrimSpace(key)
			// Try by ID
			grp, err := s.groups.GetGroup(key)
			if err == nil {
				if !groupSeen[grp.ID] {
					matchedGroups = append(matchedGroups, grp)
					groupSeen[grp.ID] = true
				}
				continue
			}
			// Try by name (could be multiple)
			byName := s.groups.GetGroupsByName(key)
			if len(byName) > 0 {
				for _, g := range byName {
					if !groupSeen[g.ID] {
						matchedGroups = append(matchedGroups, g)
						groupSeen[g.ID] = true
					}
				}
			} else {
				notFound = append(notFound, key)
			}
		}

		if len(matchedGroups) == 0 {
			http.Error(w, fmt.Sprintf("No groups found for: %v", notFound), http.StatusNotFound)
			return
		}

		var reqBody LightStateRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		var errs []string
		for _, grp := range matchedGroups {
			if reqBody.On != nil {
				if err := s.groups.SetGroupState(r.Context(), grp.ID, *reqBody.On); err != nil {
					errs = append(errs, fmt.Sprintf("group %s: %s", grp.ID, err))
				}
			}
			if reqBody.Brightness != nil {
				if err := s.groups.SetGroupBrightness(r.Context(), grp.ID, *reqBody.Brightness); err != nil {
					errs = append(errs, fmt.Sprintf("group %s: %s", grp.ID, err))
				}
			}
			if reqBody.Temperature != nil {
				if err := s.groups.SetGroupTemperature(r.Context(), grp.ID, *reqBody.Temperature); err != nil {
					errs = append(errs, fmt.Sprintf("group %s: %s", grp.ID, err))
				}
			}
		}

		if len(errs) > 0 {
			w.WriteHeader(http.StatusMultiStatus) // 207
			writeJSONResponse(w, map[string]any{"status": "partial", "errors": errs})
			return
		}
		writeJSONResponse(w, map[string]string{"status": "ok"})
	}
}

// writeJSONResponse is a helper to write JSON responses
func writeJSONResponse(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// If encoding fails, it's too late to send a different status code normally.
		// Log the error. The client will likely receive a truncated or malformed response.
		slog.Default().Error("Failed to encode JSON response", "error", err)
	}
}

// --- Added loggingMiddleware ---
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Log request details before handling
		s.logger.Debug("HTTP Request Received",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		)

		// Wrap the response writer to capture status code
		lrw := newLoggingResponseWriter(w)

		// Call the next handler
		next.ServeHTTP(lrw, r)

		// Log response details after handling
		s.logger.Debug("HTTP Response Sent",
			"method", r.Method,
			"path", r.URL.Path,
			"status", lrw.statusCode,
			"duration", time.Since(start),
		)
	})
}

// --- End added loggingMiddleware ---
