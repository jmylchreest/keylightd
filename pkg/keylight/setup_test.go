package keylight

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSetupSessionCancellation(t *testing.T) {
	var sessions SetupSessions
	ctx := t.Context()
	status, err := sessions.Start(ctx, "light", func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	require.NoError(t, err)
	require.Equal(t, "configuring", status.Stage)
	_, err = sessions.Start(ctx, "another", func(context.Context) error { return nil })
	require.ErrorContains(t, err, "already running")
	require.False(t, sessions.Cancel("another"))
	require.True(t, sessions.Cancel("light"))
	require.Eventually(t, func() bool {
		status, ok := sessions.Status("light")
		return ok && status.Stage == "canceled"
	}, time.Second, time.Millisecond)
}

func TestSetupSessionFailureReason(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		stage  string
		reason string
	}{
		{"credentials", fmt.Errorf("%w: private detail", SetupWiFiCredentialsRejected), "failed", "wifi_credentials_rejected"},
		{"timeout", fmt.Errorf("%w: %w", SetupWiFiJoinFailed, context.DeadlineExceeded), "timed_out", "wifi_join_failed"},
		{"unknown", errors.New("private detail"), "failed", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var sessions SetupSessions
			_, err := sessions.Start(t.Context(), "light", func(context.Context) error { return tc.err })
			require.NoError(t, err)
			require.Eventually(t, func() bool {
				status, ok := sessions.Status("light")
				return ok && status.Stage == tc.stage
			}, time.Second, time.Millisecond)
			status, _ := sessions.Status("light")
			require.Equal(t, tc.reason, status.FailureReason)
		})
	}
}
