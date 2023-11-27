package recovery

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGo(t *testing.T) {
	t.Run("quitting normally does not recover", func(t *testing.T) {
		done := make(chan bool)
		callback := func() {
			close(done)
		}
		logError := func(msg string, keyValuePairs ...any) {
			require.Failf(t, "should not log error", "got %v", msg)
		}

		Go("callback", logError, callback)
		assertReceive(t, done, "callback failed to finish")
	})

	t.Run("panic recovers, but goroutine terminated", func(t *testing.T) {
		logged := make(chan bool)
		callback := func() {
			panic("test")
		}
		logError := func(msg string, keyValuePairs ...any) {
			require.Equal(t, "Recovering from panic in callback", msg)
			close(logged)
		}

		Go("callback", logError, callback)
		assertReceive(t, logged, "logger failed to log")
	})
}
