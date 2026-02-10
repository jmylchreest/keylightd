package client

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"
)

type mockConn struct {
	readBuf  *bytes.Buffer
	writeBuf *bytes.Buffer
	closed   bool
}

func (m *mockConn) Read(b []byte) (int, error)         { return m.readBuf.Read(b) }
func (m *mockConn) Write(b []byte) (int, error)        { return m.writeBuf.Write(b) }
func (m *mockConn) Close() error                       { m.closed = true; return nil }
func (m *mockConn) LocalAddr() net.Addr                { return nil }
func (m *mockConn) RemoteAddr() net.Addr               { return nil }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

// Use the same dial variable as in client.go
var _ = func() bool {
	dial = net.Dial
	return true
}()

func mockDialer(conn *mockConn) func(network, address string) (net.Conn, error) {
	return func(network, address string) (net.Conn, error) {
		return conn, nil
	}
}

func TestClient_AllMethods(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	c := New(logger, "/tmp/fake.sock")

	t.Run("AddAPIKey", func(t *testing.T) {
		resp := map[string]any{
			"key": map[string]any{
				"name":         "test-key",
				"key":          "abcd1234",
				"created_at":   time.Now().Format(time.RFC3339Nano),
				"expires_at":   time.Now().Add(time.Hour * 24).Format(time.RFC3339Nano),
				"last_used_at": time.Time{}.Format(time.RFC3339Nano),
				"disabled":     false,
			},
		}
		buf := &bytes.Buffer{}
		_ = json.NewEncoder(buf).Encode(resp)
		conn := &mockConn{readBuf: buf, writeBuf: &bytes.Buffer{}}
		oldDial := dial
		dial = mockDialer(conn)
		defer func() { dial = oldDial }()

		key, err := c.AddAPIKey("test-key", 86400)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if key["name"] != "test-key" || key["key"] != "abcd1234" {
			t.Fatalf("unexpected result: %v", key)
		}
	})

	t.Run("ListAPIKeys", func(t *testing.T) {
		resp := map[string]any{
			"keys": []any{
				map[string]any{
					"name":         "key1",
					"key":          "abcd1234",
					"created_at":   time.Now().Format(time.RFC3339Nano),
					"expires_at":   time.Now().Add(time.Hour * 24).Format(time.RFC3339Nano),
					"last_used_at": time.Time{}.Format(time.RFC3339Nano),
					"disabled":     false,
				},
				map[string]any{
					"name":         "key2",
					"key":          "efgh5678",
					"created_at":   time.Now().Format(time.RFC3339Nano),
					"expires_at":   time.Now().Add(time.Hour * 48).Format(time.RFC3339Nano),
					"last_used_at": time.Now().Format(time.RFC3339Nano),
					"disabled":     true,
				},
			},
		}
		buf := &bytes.Buffer{}
		_ = json.NewEncoder(buf).Encode(resp)
		conn := &mockConn{readBuf: buf, writeBuf: &bytes.Buffer{}}
		oldDial := dial
		dial = mockDialer(conn)
		defer func() { dial = oldDial }()

		keys, err := c.ListAPIKeys()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(keys) != 2 {
			t.Fatalf("expected 2 keys, got %d", len(keys))
		}
		if keys[0]["name"] != "key1" || keys[1]["name"] != "key2" {
			t.Fatalf("unexpected keys: %v", keys)
		}
	})

	t.Run("DeleteAPIKey", func(t *testing.T) {
		// For DeleteAPIKey, we need to handle the client.request usage differently
		// since it passes nil as the response parameter
		// Instead, create a valid response the client can process even with nil
		resp := map[string]any{"status": "ok"}
		buf := &bytes.Buffer{}
		_ = json.NewEncoder(buf).Encode(resp)
		conn := &mockConn{readBuf: buf, writeBuf: &bytes.Buffer{}}
		oldDial := dial
		dial = mockDialer(conn)
		defer func() { dial = oldDial }()

		err := c.DeleteAPIKey("abcd1234")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("SetAPIKeyDisabledStatus", func(t *testing.T) {
		resp := map[string]any{
			"key": map[string]any{
				"name":         "test-key",
				"key":          "abcd1234",
				"created_at":   time.Now().Format(time.RFC3339Nano),
				"expires_at":   time.Now().Add(time.Hour * 24).Format(time.RFC3339Nano),
				"last_used_at": time.Time{}.Format(time.RFC3339Nano),
				"disabled":     true,
			},
		}
		buf := &bytes.Buffer{}
		_ = json.NewEncoder(buf).Encode(resp)
		conn := &mockConn{readBuf: buf, writeBuf: &bytes.Buffer{}}
		oldDial := dial
		dial = mockDialer(conn)
		defer func() { dial = oldDial }()

		key, err := c.SetAPIKeyDisabledStatus("test-key", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if key["name"] != "test-key" || key["disabled"] != true {
			t.Fatalf("unexpected result: %v", key)
		}
	})

	t.Run("GetLights", func(t *testing.T) {
		resp := map[string]any{
			"lights": map[string]any{"light1": map[string]any{"id": "light1"}},
		}
		buf := &bytes.Buffer{}
		_ = json.NewEncoder(buf).Encode(resp)
		conn := &mockConn{readBuf: buf, writeBuf: &bytes.Buffer{}}
		oldDial := dial
		dial = mockDialer(conn)
		defer func() { dial = oldDial }()

		lights, err := c.GetLights()
		if err != nil || lights["light1"].(map[string]any)["id"] != "light1" {
			t.Fatalf("unexpected result: %v %v", lights, err)
		}
	})

	t.Run("GetLight", func(t *testing.T) {
		resp := map[string]any{"id": "light1"}
		buf := &bytes.Buffer{}
		_ = json.NewEncoder(buf).Encode(resp)
		conn := &mockConn{readBuf: buf, writeBuf: &bytes.Buffer{}}
		oldDial := dial
		dial = mockDialer(conn)
		defer func() { dial = oldDial }()

		light, err := c.GetLight("light1")
		if err != nil || light["id"] != "light1" {
			t.Fatalf("unexpected result: %v %v", light, err)
		}
	})

	t.Run("SetLightState", func(t *testing.T) {
		resp := map[string]any{"ok": true}
		buf := &bytes.Buffer{}
		_ = json.NewEncoder(buf).Encode(resp)
		conn := &mockConn{readBuf: buf, writeBuf: &bytes.Buffer{}}
		oldDial := dial
		dial = mockDialer(conn)
		defer func() { dial = oldDial }()

		err := c.SetLightState("light1", "on", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("CreateGroup", func(t *testing.T) {
		resp := map[string]any{"ok": true}
		buf := &bytes.Buffer{}
		_ = json.NewEncoder(buf).Encode(resp)
		conn := &mockConn{readBuf: buf, writeBuf: &bytes.Buffer{}}
		oldDial := dial
		dial = mockDialer(conn)
		defer func() { dial = oldDial }()

		err := c.CreateGroup("g1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("GetGroup", func(t *testing.T) {
		resp := map[string]any{"id": "g1"}
		buf := &bytes.Buffer{}
		_ = json.NewEncoder(buf).Encode(resp)
		conn := &mockConn{readBuf: buf, writeBuf: &bytes.Buffer{}}
		oldDial := dial
		dial = mockDialer(conn)
		defer func() { dial = oldDial }()

		group, err := c.GetGroup("g1")
		if err != nil || group["id"] != "g1" {
			t.Fatalf("unexpected result: %v %v", group, err)
		}
	})

	t.Run("GetGroups", func(t *testing.T) {
		resp := map[string]any{
			"groups": []any{
				map[string]any{"id": "g1", "name": "G1", "lights": []any{}},
				map[string]any{"id": "g2", "name": "G2", "lights": []any{}},
			},
		}
		buf := &bytes.Buffer{}
		_ = json.NewEncoder(buf).Encode(resp)
		conn := &mockConn{readBuf: buf, writeBuf: &bytes.Buffer{}}
		oldDial := dial
		dial = mockDialer(conn)
		defer func() { dial = oldDial }()

		groups, err := c.GetGroups()
		if err != nil || len(groups) != 2 {
			t.Fatalf("unexpected result: %v %v", groups, err)
		}
	})

	t.Run("SetGroupState", func(t *testing.T) {
		resp := map[string]any{"ok": true}
		buf := &bytes.Buffer{}
		_ = json.NewEncoder(buf).Encode(resp)
		conn := &mockConn{readBuf: buf, writeBuf: &bytes.Buffer{}}
		oldDial := dial
		dial = mockDialer(conn)
		defer func() { dial = oldDial }()

		err := c.SetGroupState("g1", "on", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("DeleteGroup", func(t *testing.T) {
		resp := map[string]any{"ok": true}
		buf := &bytes.Buffer{}
		_ = json.NewEncoder(buf).Encode(resp)
		conn := &mockConn{readBuf: buf, writeBuf: &bytes.Buffer{}}
		oldDial := dial
		dial = mockDialer(conn)
		defer func() { dial = oldDial }()

		err := c.DeleteGroup("g1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("SetGroupLights", func(t *testing.T) {
		resp := map[string]any{"ok": true}
		buf := &bytes.Buffer{}
		_ = json.NewEncoder(buf).Encode(resp)
		conn := &mockConn{readBuf: buf, writeBuf: &bytes.Buffer{}}
		oldDial := dial
		dial = mockDialer(conn)
		defer func() { dial = oldDial }()

		err := c.SetGroupLights("g1", []string{"l1", "l2"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
