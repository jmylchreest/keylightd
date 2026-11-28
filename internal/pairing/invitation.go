// Package pairing defines the invitation transferred through an authorized local client.
package pairing

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
)

type Invitation struct {
	URL         string `json:"url"`
	Fingerprint string `json:"fingerprint"`
	Secret      string `json:"secret"`
}

func Fingerprint(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (i Invitation) Encode() (string, error) {
	raw, err := json.Marshal(i) //nolint:gosec // The single-use secret is transferred in the invitation.
	return "keylightd1." + base64.RawURLEncoding.EncodeToString(raw), err
}

func ValidEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.Path == "" && u.RawQuery == "" && !u.ForceQuery && u.Fragment == ""
}

func Decode(token string) (Invitation, error) {
	var i Invitation
	code, ok := strings.CutPrefix(strings.TrimSpace(token), "keylightd1.")
	if ok && len(code) < 8192 {
		raw, err := base64.RawURLEncoding.DecodeString(code)
		if err == nil && json.Unmarshal(raw, &i) == nil && ValidEndpoint(i.URL) && len(i.Fingerprint) == 43 && len(i.Secret) == 43 {
			return i, nil
		}
	}
	return Invitation{}, errors.New("invalid keylightd invitation")
}
