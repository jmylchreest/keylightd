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

// AirMK2Provisioning is the material sent to a light in pairing mode.
// The client's private key stays with the daemon.
type AirMK2Provisioning struct {
	CA                []byte
	ServerCertificate []byte
	ServerKey         []byte
	ServerName        string
}

// AirMK2Pairer provisions one explicitly selected light and returns its
// address once it has joined the network.
type AirMK2Pairer interface {
	Provision(context.Context, AirMK2Provisioning) (host string, port int, err error)
}

// AirMK2Identity contains the verified credentials the daemon can persist.
type AirMK2Identity struct {
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
	host, port, err := pairer.Provision(ctx, AirMK2Provisioning{ca.pem, server.pem, server.keyPEM, serverName})
	if err != nil {
		return nil, fmt.Errorf("provision Air MK.2: %w", err)
	}
	device, err := NewAirMK2Client(host, port, &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: serverName, Certificates: []tls.Certificate{pair}})
	if err != nil {
		return nil, err
	}
	info, err := device.GetAccessoryInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("verify provisioned Air MK.2: %w", err)
	}
	if info.HardwareBoardType != 214 || info.SerialNumber == "" {
		return nil, errors.New("unexpected identity from provisioned Air MK.2")
	}
	if _, err := device.GetLightState(ctx); err != nil {
		return nil, fmt.Errorf("read provisioned Air MK.2 state: %w", err)
	}
	return &AirMK2Identity{ca.pem, client.pem, client.keyPEM, serverName, net.JoinHostPort(host, strconv.Itoa(port)), *info}, nil
}
