package keylight

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"time"
)

// AirMK2SetupOptions selects a Bluetooth peripheral and supplies its Wi-Fi settings.
type AirMK2SetupOptions struct {
	Peripheral string `json:"peripheral" minLength:"1" maxLength:"36"`
	SSID       string `json:"ssid" minLength:"1" maxLength:"32"`
	Password   string `json:"password,omitempty" writeOnly:"true"`
	Security   uint16 `json:"security" doc:"Wi-Fi security code, such as 200 for WPA2 AES"`
	Port       int    `json:"port,omitempty" doc:"Deprecated. The light port is discovered via mDNS."`
}

// AirMK2Provisioning is the material sent to a light in pairing mode.
// The client's private key stays with the daemon.
type AirMK2Provisioning struct {
	CA                []byte
	ServerCertificate []byte
	ServerKey         []byte
	ServerName        string
	Confirm           func(context.Context) error
}

// AirMK2Pairer provisions one explicitly selected light and returns its
// address once it has joined the network.
type AirMK2Pairer interface {
	Provision(context.Context, AirMK2Provisioning) (host string, port int, err error)
}

// AirMK2Identity contains the verified credentials the daemon can persist.
type AirMK2Identity struct {
	DeviceID          string
	CA                []byte
	ClientCertificate []byte
	ClientKey         []byte
	ServerName        string
	Address           string
	Info              AccessoryInfo
}

type airCertificate struct {
	cert   *x509.Certificate
	key    *ecdsa.PrivateKey
	pem    []byte
	keyPEM []byte
}

func issueAirCertificate(template *x509.Certificate, issuer *airCertificate) (airCertificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return airCertificate{}, err
	}
	parent, signer := template, key
	if issuer != nil {
		parent, signer = issuer.cert, issuer.key
	}
	der, err := x509.CreateCertificate(rand.Reader, template, parent, &key.PublicKey, signer)
	if err != nil {
		return airCertificate{}, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return airCertificate{}, err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return airCertificate{}, err
	}
	return airCertificate{cert, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})}, nil
}

func airCertificateTemplate(name string, now time.Time) (*x509.Certificate, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}
	serial.Add(serial, big.NewInt(1))
	return &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: name},
		NotBefore: now.Add(-5 * time.Minute), NotAfter: now.AddDate(5, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature}, nil
}

// OnboardAirMK2 generates a private identity for a light, provisions it using
// the supplied pairing transport, and verifies it before returning credentials.
func OnboardAirMK2(ctx context.Context, pairer AirMK2Pairer) (*AirMK2Identity, error) {
	return onboardAirMK2(ctx, pairer, nil, nil)
}

type airMK2Handoff func(context.Context, net.IP, func(context.Context, string, int) error) (string, int, error)

func onboardAirMK2(ctx context.Context, pairer AirMK2Pairer, confirm func(context.Context) error, handoff airMK2Handoff) (*AirMK2Identity, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if pairer == nil {
		return nil, errors.New("air MK.2 pairing transport is required")
	}
	now := time.Now()
	caTemplate, err := airCertificateTemplate("keylightd Air MK.2 CA", now)
	if err != nil {
		return nil, err
	}
	caTemplate.IsCA = true
	caTemplate.BasicConstraintsValid = true
	caTemplate.KeyUsage |= x509.KeyUsageCertSign
	ca, err := issueAirCertificate(caTemplate, nil)
	if err != nil {
		return nil, err
	}
	serverName := "air-mk2.keylightd.local"
	serverTemplate, err := airCertificateTemplate(serverName, now)
	if err != nil {
		return nil, err
	}
	serverTemplate.DNSNames = []string{serverName}
	serverTemplate.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	server, err := issueAirCertificate(serverTemplate, &ca)
	if err != nil {
		return nil, err
	}
	clientTemplate, err := airCertificateTemplate("keylightd Air MK.2 client", now)
	if err != nil {
		return nil, err
	}
	clientTemplate.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	client, err := issueAirCertificate(clientTemplate, &ca)
	if err != nil {
		return nil, err
	}
	pair, err := tls.X509KeyPair(client.pem, client.keyPEM)
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca.cert)
	host, port, err := pairer.Provision(ctx, AirMK2Provisioning{CA: ca.pem, ServerCertificate: server.pem, ServerKey: server.keyPEM, ServerName: serverName, Confirm: confirm})
	if err != nil {
		return nil, fmt.Errorf("provision Air MK.2: %w", err)
	}
	var info *AccessoryInfo
	verify := func(ctx context.Context, host string, port int) error {
		device, err := NewAirMK2Client(host, port, &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: serverName, Certificates: []tls.Certificate{pair}})
		if err != nil {
			return err
		}
		accessory, err := device.GetAccessoryInfo(ctx)
		if err != nil {
			return fmt.Errorf("verify provisioned Air MK.2: %w", err)
		}
		if accessory.HardwareBoardType != 214 || accessory.SerialNumber == "" {
			return errors.New("unexpected device identity")
		}
		if _, err := device.GetLightState(ctx); err != nil {
			return fmt.Errorf("read provisioned Air MK.2 state: %w", err)
		}
		info = accessory
		return nil
	}
	var deviceID string
	if handoff != nil {
		deviceID, port, err = handoff(ctx, net.ParseIP(host), verify)
	} else {
		err = verify(ctx, host, port)
	}
	if errors.Is(err, SetupDiscoveryFailed) {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", SetupVerificationFailed, err)
	}
	return &AirMK2Identity{
		DeviceID: deviceID, CA: ca.pem, ClientCertificate: client.pem, ClientKey: client.keyPEM,
		ServerName: serverName, Address: net.JoinHostPort(host, strconv.Itoa(port)), Info: *info,
	}, nil
}
