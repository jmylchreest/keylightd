package keylight

import (
	"context"
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
)

func setupFailureReason(err error) string {
	for _, reason := range []SetupFailure{
		SetupBluetoothFailed, SetupCertificateInstallFailed, SetupWiFiCredentialsRejected,
		SetupWiFiJoinFailed, SetupVerificationFailed, SetupSaveFailed,
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
	DeviceID      string `json:"deviceId"`
	Stage         string `json:"stage"`
	FailureReason string `json:"failureReason,omitempty"`
}

// SetupSessions owns the lifetime of an explicitly started device setup.
type SetupSessions struct {
	mu     sync.Mutex
	active context.CancelFunc
	status SetupStatus
}

func (s *SetupSessions) Start(ctx context.Context, id string, run func(context.Context) error) (SetupStatus, error) {
	if id == "" || run == nil {
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
		err := run(setupCtx)
		s.mu.Lock()
		defer s.mu.Unlock()
		switch {
		case err == nil:
			s.status.Stage = "ready"
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
