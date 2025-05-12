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

	t.Run("GetLights", func(t *testing.T) {
		resp := map[string]interface{}{
			"lights": map[string]interface{}{"light1": map[string]interface{}{"id": "light1"}},
		}
		buf := &bytes.Buffer{}
		_ = json.NewEncoder(buf).Encode(resp)
		conn := &mockConn{readBuf: buf, writeBuf: &bytes.Buffer{}}
		oldDial := dial
		dial = mockDialer(conn)
		defer func() { dial = oldDial }()

		lights, err := c.GetLights()
		if err != nil || lights["light1"].(map[string]interface{})["id"] != "light1" {
			t.Fatalf("unexpected result: %v %v", lights, err)
		}
	})

	t.Run("GetLight", func(t *testing.T) {
		resp := map[string]interface{}{"id": "light1"}
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
		resp := map[string]interface{}{"ok": true}
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
		resp := map[string]interface{}{"ok": true}
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
		resp := map[string]interface{}{"id": "g1"}
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
		resp := map[string]interface{}{
			"groups": map[string]interface{}{
				"g1": map[string]interface{}{"name": "G1"},
				"g2": map[string]interface{}{"name": "G2"},
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
		resp := map[string]interface{}{"ok": true}
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
		resp := map[string]interface{}{"ok": true}
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
		resp := map[string]interface{}{"ok": true}
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
