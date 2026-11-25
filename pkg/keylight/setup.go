package keylight

import (
	"context"
	"crypto/rand"
	"errors"
	"sync"
	"time"
)

// SetupFailure is a stable reason that can be returned without transport details.
type SetupFailure string

func (reason SetupFailure) Error() string { return string(reason) }

const (
	SetupBluetoothFailed          SetupFailure = "bluetooth_failed"
	SetupCertificateInstallFailed SetupFailure = "certificate_install_failed"
	SetupWiFiCredentialsRejected  SetupFailure = "wifi_credentials_rejected"
	SetupWiFiJoinFailed           SetupFailure = "wifi_join_failed"
	SetupVerificationFailed       SetupFailure = "verification_failed"
	SetupSaveFailed               SetupFailure = "save_failed"
	SetupDiscoveryFailed          SetupFailure = "discovery_failed"
)

func setupFailureReason(err error) string {
	for _, reason := range []SetupFailure{
		SetupBluetoothFailed, SetupCertificateInstallFailed, SetupWiFiCredentialsRejected,
		SetupWiFiJoinFailed, SetupVerificationFailed, SetupSaveFailed, SetupDiscoveryFailed,
	} {
		if errors.Is(err, reason) {
			return string(reason)
		}
	}
	return ""
}

// SetupStatus is safe to return to a frontend. Credentials and transport
// errors remain on the daemon.
type SetupStatus struct {
	DeviceID       string `json:"deviceId"`
	Stage          string `json:"stage"`
	ConfirmationID string `json:"confirmationId,omitempty"`
	ConfiguredID   string `json:"configuredId,omitempty"`
	FailureReason  string `json:"failureReason,omitempty"`
}

// SetupSessions owns the lifetime of an explicitly started device setup.
type SetupSessions struct {
	mu           sync.Mutex
	active       context.CancelFunc
	confirmation chan struct{}
	blink        chan struct{}
	status       SetupStatus
}

func (s *SetupSessions) Start(ctx context.Context, id string, run func(context.Context) error) (SetupStatus, error) {
	if run == nil {
		return SetupStatus{}, errors.New("setup operation is required")
	}
	return s.start(ctx, id, func(ctx context.Context) (string, error) { return "", run(ctx) })
}

func (s *SetupSessions) start(ctx context.Context, id string, run func(context.Context) (string, error)) (SetupStatus, error) {
	if id == "" {
		return SetupStatus{}, errors.New("device and setup operation are required")
	}
	if err := ctx.Err(); err != nil {
		return SetupStatus{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active != nil {
		return SetupStatus{}, errors.New("device setup is already running")
	}
	setupCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	s.active = cancel
	s.status = SetupStatus{DeviceID: id, Stage: "configuring"}
	go func() {
		defer cancel()
		configuredID, err := run(setupCtx)
		s.mu.Lock()
		defer s.mu.Unlock()
		switch {
		case err == nil:
			s.status.Stage = "ready"
			s.status.ConfiguredID = configuredID
		case setupCtx.Err() == context.Canceled:
			s.status.Stage = "canceled"
		case setupCtx.Err() == context.DeadlineExceeded || errors.Is(err, context.DeadlineExceeded):
			s.status.Stage = "timed_out"
			s.status.FailureReason = setupFailureReason(err)
		default:
			s.status.Stage = "failed"
			s.status.FailureReason = setupFailureReason(err)
		}
		s.active = nil
	}()
	return s.status, nil
}

func (s *SetupSessions) Status(id string) (SetupStatus, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status, s.status.DeviceID != "" && s.status.DeviceID == id
}

func (s *SetupSessions) Cancel(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil || s.status.DeviceID != id {
		return false
	}
	s.status.Stage = "canceling"
	s.active()
	return true
}

func (s *SetupSessions) waitForConfirmation(ctx context.Context, identify func() error) error {
	s.mu.Lock()
	if err := ctx.Err(); err != nil {
		s.mu.Unlock()
		return err
	}
	confirmed := make(chan struct{})
	s.confirmation = confirmed
	blink := make(chan struct{}, 1)
	s.blink = blink
	s.status.Stage = "awaiting_confirmation"
	s.status.ConfirmationID = rand.Text()
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.confirmation = nil
		s.blink = nil
		s.status.ConfirmationID = ""
		s.mu.Unlock()
	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-confirmed:
			return ctx.Err()
		case <-blink:
			s.mu.Lock()
			waiting := s.status.Stage == "awaiting_confirmation"
			s.mu.Unlock()
			if waiting && ctx.Err() == nil {
				if err := identify(); err != nil {
					return err
				}
			}
		}
	}
}

// Blink requests another identification on the connection awaiting confirmation.
func (s *SetupSessions) Blink(id, confirmationID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.awaitingConfirmation(id, confirmationID) {
		return false
	}
	select {
	case s.blink <- struct{}{}:
	default:
	}
	return true
}

func (s *SetupSessions) awaitingConfirmation(id, confirmationID string) bool {
	return s.status.DeviceID == id && s.status.Stage == "awaiting_confirmation" &&
		s.confirmation != nil && s.status.ConfirmationID == confirmationID
}

func (s *SetupSessions) Confirm(id, confirmationID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.awaitingConfirmation(id, confirmationID) {
		return false
	}
	s.status.Stage = "configuring"
	close(s.confirmation)
	return true
}
