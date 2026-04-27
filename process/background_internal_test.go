package process

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRunGuardedBackgroundTaskRecoversPanic(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})

	runGuardedBackgroundTask("panic-test", func() {
		defer close(done)
		panic("boom")
	})

	require.Eventually(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)
}
