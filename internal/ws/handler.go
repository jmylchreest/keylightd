package ws

import (
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins for now; API key auth provides access control.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Handler returns an http.HandlerFunc that upgrades connections to WebSocket
// and registers the client with the hub. Auth is handled at the Chi middleware
// layer (RawAPIKeyAuth) before this handler is called.
func Handler(hub *Hub, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("ws: upgrade failed", "error", err, "remote_addr", r.RemoteAddr)
			return
		}

		client := hub.NewClient(conn)
		hub.Register(client)

		// Start read/write pumps in separate goroutines.
		go client.WritePump()
		go client.ReadPump()
	}
}
