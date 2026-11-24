package keylight

import (
	"context"
	"errors"
	"sync"
	"time"
)

// SetupStatus is safe to return to a frontend. Credentials and transport
// errors remain on the daemon.
type SetupStatus struct {
	DeviceID string `json:"deviceId"`
	Stage    string `json:"stage"`
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
		case setupCtx.Err() == context.DeadlineExceeded:
			s.status.Stage = "timed_out"
		default:
			s.status.Stage = "failed"
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
