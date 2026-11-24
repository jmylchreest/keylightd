package keylight

import (
	"context"
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
