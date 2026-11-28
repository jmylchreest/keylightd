package server

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/netip"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jmylchreest/keylightd/internal/config"
	"github.com/jmylchreest/keylightd/internal/pairing"
)

type remoteAPI struct {
	mu               sync.Mutex
	listener         *apiListener
	fingerprint      string
	enabled, stopped bool
	invitations      map[string]time.Time
}

// Switching the accepted connection type keeps ownership of the port across
// protocol changes. Track the underlying sockets so upgraded WebSockets close too.
type apiListener struct {
	net.Listener
	mu        sync.Mutex
	tlsConfig *tls.Config
	clients   map[*apiConn]bool
}
type apiConn struct {
	net.Conn
	owner *apiListener
}

func (c *apiConn) Close() error {
	c.owner.mu.Lock()
	delete(c.owner.clients, c)
	c.owner.mu.Unlock()
	return c.Conn.Close()
}
func (l *apiListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	tracked := &apiConn{Conn: c, owner: l}
	l.clients[tracked] = true
	if l.tlsConfig != nil {
		return tls.Server(tracked, l.tlsConfig), nil
	}
	return tracked, nil
}
func (l *apiListener) changeMode(cfg *tls.Config) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.tlsConfig = cfg
	for c := range l.clients {
		_ = c.Conn.Close()
		delete(l.clients, c)
	}
}
func (l *apiListener) disconnect() { l.changeMode(nil) }

func newIdentity() (config.HTTPSIdentity, error) {
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return config.HTTPSIdentity{}, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return config.HTTPSIdentity{}, err
	}
	now := time.Now()
	tmpl := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "keylightd"}, NotBefore: now.Add(-time.Hour), NotAfter: now.AddDate(10, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, pub, key)
	if err != nil {
		return config.HTTPSIdentity{}, err
	}
	pk, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return config.HTTPSIdentity{}, err
	}
	return config.HTTPSIdentity{Certificate: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), PrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pk}))}, nil
}

func parseIdentity(identity config.HTTPSIdentity) (*tls.Config, string, error) {
	cert, err := tls.X509KeyPair([]byte(identity.Certificate), []byte(identity.PrivateKey))
	if err != nil {
		return nil, "", fmt.Errorf("load saved HTTPS identity: %w", err)
	}
	if cert.Leaf == nil {
		cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return nil, "", err
		}
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}}, pairing.Fingerprint(cert.Leaf), nil
}

func (s *Server) startNetwork() error {
	identity := s.cfg.HTTPSIdentity()
	var cfg *tls.Config
	if identity != (config.HTTPSIdentity{}) {
		var err error
		cfg, s.network.fingerprint, err = parseIdentity(identity)
		if err != nil {
			return err
		}
	}
	s.network.enabled = identity.Enabled
	s.network.invitations = make(map[string]time.Time)
	if !identity.Enabled {
		cfg = nil
	}
	if s.cfg.Config.API.ListenAddress == "" && cfg == nil {
		return nil
	}
	err := s.listenAPI(cfg)
	if err != nil && !identity.Enabled {
		s.logger.Error("HTTP listener unavailable; local socket is available", "error", err)
		return nil
	}
	return err
}
func (s *Server) listenAPI(cfg *tls.Config) error {
	addr := s.cfg.Config.API.ListenAddress
	if addr == "" {
		addr = config.DefaultAPIListenAddress
	}
	raw, err := (&net.ListenConfig{}).Listen(s.rootCtx, "tcp", addr)
	if err != nil {
		return err
	}
	l := &apiListener{Listener: raw, tlsConfig: cfg, clients: make(map[*apiConn]bool)}
	s.network.listener = l
	s.wg.Go(func() {
		if err := s.httpServer.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
			s.logger.Error("API listener stopped", "error", err)
		}
	})
	return nil
}

func addressURLs(ctx context.Context, addr net.Addr) []string {
	socket, ok := addr.(*net.TCPAddr)
	if !ok {
		return nil
	}
	port := strconv.Itoa(socket.Port)
	makeURL := func(host string) string { return "https://" + net.JoinHostPort(host, port) }
	var hosts []string
	add := func(ip netip.Addr) {
		ip = ip.Unmap()
		if !ip.IsValid() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() {
			return
		}
		if !socket.IP.IsUnspecified() && !socket.IP.Equal(net.IP(ip.AsSlice())) {
			return
		}
		if socket.IP.To4() != nil && !ip.Is4() {
			return
		}
		host := ip.String()
		if !slices.Contains(hosts, host) {
			hosts = append(hosts, host)
		}
	}
	// UDP connect asks the kernel which source address it would route through.
	// No application datagram is sent.
	if c, err := (&net.Dialer{Timeout: 300 * time.Millisecond}).DialContext(ctx, "udp4", "192.0.2.1:9"); err == nil {
		if source, ok := c.LocalAddr().(*net.UDPAddr); ok {
			add(source.AddrPort().Addr())
		}
		_ = c.Close()
	}
	if addr, ok := netip.AddrFromSlice(socket.IP); ok {
		add(addr)
	}
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			if p, err := netip.ParsePrefix(a.String()); err == nil {
				add(p.Addr())
			}
		}
	}
	urls := make([]string, 0, len(hosts)+1)
	for _, h := range hosts {
		urls = append(urls, makeURL(h))
	}
	if host, err := os.Hostname(); err == nil && host != "" {
		// Only advertise a hostname whose resolved addresses include a listener address.
		ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		ips, _ := net.DefaultResolver.LookupHost(ctx, host)
		for _, ip := range ips {
			if slices.Contains(hosts, ip) {
				urls = append(urls, makeURL(host))
				break
			}
		}
	}
	return urls
}

func (s *Server) controlHTTPS(action, endpoint string) (map[string]any, error) {
	n := &s.network
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.stopped {
		return nil, errors.New("daemon is stopping")
	}
	identity := s.cfg.HTTPSIdentity()
	switch action {
	case "https.enable":
		if !n.enabled {
			if identity.Certificate == "" {
				var err error
				identity, err = newIdentity()
				if err != nil {
					return nil, err
				}
			}
			cfg, pin, err := parseIdentity(identity)
			if err != nil {
				return nil, err
			}
			identity.Enabled = true
			if n.listener == nil {
				if err = s.listenAPI(cfg); err != nil {
					return nil, err
				}
				if err = s.cfg.SetHTTPSIdentity(identity); err != nil {
					_ = n.listener.Close()
					n.listener.disconnect()
					n.listener = nil
					return nil, err
				}
			} else {
				if err = s.cfg.SetHTTPSIdentity(identity); err != nil {
					return nil, err
				}
				n.listener.changeMode(cfg)
			}
			n.fingerprint, n.enabled = pin, true
		}
	case "https.disable":
		if n.enabled {
			identity.Enabled = false
			if err := s.cfg.SetHTTPSIdentity(identity); err != nil {
				return nil, err
			}
			if s.cfg.Config.API.ListenAddress == "" {
				_ = n.listener.Close()
			}
			n.listener.disconnect()
			if s.cfg.Config.API.ListenAddress == "" {
				n.listener = nil
			}
			clear(n.invitations)
			n.enabled = false
		}
	case "https.delete-identity":
		if n.enabled {
			return nil, errors.New("disable HTTPS before forgetting its identity")
		}
		if err := s.cfg.SetHTTPSIdentity(config.HTTPSIdentity{}); err != nil {
			return nil, err
		}
		n.fingerprint = ""
	}
	var urls []string
	if n.listener != nil {
		urls = addressURLs(s.rootCtx, n.listener.Addr())
	}
	if action == "pair.create" {
		if !n.enabled {
			return nil, errors.New("enable HTTPS before creating an invitation")
		}
		if endpoint == "" && len(urls) > 0 {
			endpoint = urls[0]
		}
		if !slices.Contains(urls, endpoint) {
			return nil, errors.New("choose an address the daemon listens on")
		}
		now := time.Now()
		for token, expiry := range n.invitations {
			if !expiry.After(now) {
				delete(n.invitations, token)
			}
		}
		if len(n.invitations) >= 128 {
			return nil, errors.New("too many outstanding invitations")
		}
		var secret [32]byte
		if _, err := rand.Read(secret[:]); err != nil {
			return nil, err
		}
		i := pairing.Invitation{URL: endpoint, Fingerprint: n.fingerprint, Secret: base64.RawURLEncoding.EncodeToString(secret[:])}
		token, err := i.Encode()
		if err != nil {
			return nil, err
		}
		expires := now.Add(5 * time.Minute)
		n.invitations[i.Secret] = expires
		return map[string]any{"invitation": token, "expires": expires}, nil
	}
	return map[string]any{"enabled": n.enabled, "fingerprint": n.fingerprint, "urls": urls, "restore_http": s.cfg.Config.API.ListenAddress != ""}, nil
}
func (s *Server) handleHTTPS(r socketRequest) socketActionResult {
	endpoint, _ := r.data["url"].(string)
	data, err := s.controlHTTPS(r.action, endpoint)
	if err != nil {
		s.sendError(r.conn, r.id, err.Error())
	} else {
		s.sendResponse(r.conn, r.id, data)
	}
	return socketContinue
}
func (s *Server) exchangeInvitation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.TLS == nil {
		http.NotFound(w, r)
		return
	}
	var input struct {
		Secret string `json:"secret"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&input) != nil {
		http.Error(w, "invalid invitation", http.StatusBadRequest)
		return
	}
	n := &s.network
	n.mu.Lock()
	defer n.mu.Unlock()
	expiry, ok := n.invitations[input.Secret]
	if !n.enabled || !ok || !expiry.After(time.Now()) {
		http.Error(w, "invitation expired or unavailable", http.StatusForbidden)
		return
	}
	suffix, err := config.GenerateKey(10)
	if err != nil {
		http.Error(w, "cannot issue key", http.StatusInternalServerError)
		return
	}
	key, err := s.apikeyManager.CreateAPIKey("paired-"+strings.ToLower(suffix), 0)
	if err != nil {
		http.Error(w, "cannot persist paired API key", http.StatusInternalServerError)
		return
	}
	delete(n.invitations, input.Secret)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"api_key": key.Key})
}

// A failed setup can revoke its provisional credential without placing the
// credential in a request path (which HTTP access logs routinely record).
func (s *Server) revokeCredential(w http.ResponseWriter, r *http.Request) {
	if r.TLS == nil {
		http.NotFound(w, r)
		return
	}
	if err := s.apikeyManager.DeleteAPIKey(r.Header.Get("X-API-Key")); err != nil {
		http.Error(w, "cannot revoke credential", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
