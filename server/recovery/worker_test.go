package recovery

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func assertReceive[X any](t *testing.T, c chan X, failureMessage string) {
	select {
	case <-c:
	case <-time.After(5 * time.Second):
		require.Fail(t, failureMessage)
	}
}

func TestGoWorker(t *testing.T) {
	// makeDoStart simulates a start function with the defined sequence of actions, whether to
	// do a "normal" run, waiting for the signal to stop, to "panic", immediately crashing,
	// or to unexpectedly "exit".
	makeDoStart := func(t *testing.T, sequence []string, started chan int, stopping, stopped chan bool) (func(), func(string, ...any)) {
		count := 0

		doStart := func() {
			started <- count
			defer func() {
				count++
			}()

			require.Less(t, count, len(sequence), "unexpected number of invocations")

			switch sequence[count] {
			case "normal":
				// Wait for shut down
				<-stopping
				close(stopped)

			case "panic":
				panic("doStart panic")

			case "exit":
				return

			default:
				require.Failf(t, "unexpected sequence", "got %s", sequence[count])
			}
		}

		logError := func(msg string, keyValuePairs ...any) {
			// logError gets called after count++
			actualCount := count - 1

			require.Less(t, actualCount, len(sequence), "unexpected number of invocations")

			switch sequence[actualCount] {
			case "normal":
				require.Failf(t, "should not log error", "got %v", msg)
			case "panic":
				require.Equal(t, "Recovering from panic in job", msg)
			case "exit":
				require.Equal(t, "Recovering from unexpected exit in job", msg)
			default:
				require.Failf(t, "unexpected sequence", "got %s", sequence[actualCount])
			}
		}

		return doStart, logError
	}

	t.Run("quitting normally does not recover", func(t *testing.T) {
		started := make(chan int)
		stopping := make(chan bool)
		stopped := make(chan bool)

		doStart, logError := makeDoStart(t, []string{"normal"}, started, stopping, stopped)
		isQuitting := func() bool {
			select {
			case <-stopping:
				return true
			default:
				return false
			}
		}

		GoWorker("job", doStart, isQuitting, logError)
		assertReceive(t, started, "doStart failed to start")
		close(stopping)
		assertReceive(t, stopped, "doStart failed to finish")
	})

	t.Run("single panic recovers", func(t *testing.T) {
		started := make(chan int)
		stopping := make(chan bool)
		stopped := make(chan bool)

		doStart, logError := makeDoStart(t, []string{"panic", "normal"}, started, stopping, stopped)
		isQuitting := func() bool {
			select {
			case <-stopping:
				return true
			default:
				return false
			}
		}

		GoWorker("job", doStart, isQuitting, logError)
		assertReceive(t, started, "doStart failed to start")
		assertReceive(t, started, "doStart failed to start the second time")
		close(stopping)
		assertReceive(t, stopped, "doStart failed to finish")
	})

	t.Run("unexpected exit recovers", func(t *testing.T) {
		started := make(chan int)
		stopping := make(chan bool)
		stopped := make(chan bool)

		doStart, logError := makeDoStart(t, []string{"exit", "normal"}, started, stopping, stopped)
		isQuitting := func() bool {
			select {
			case <-stopping:
				return true
			default:
				return false
			}
		}

		GoWorker("job", doStart, isQuitting, logError)
		assertReceive(t, started, "doStart failed to start")
		assertReceive(t, started, "doStart failed to start the second time")
		close(stopping)
		assertReceive(t, stopped, "doStart failed to finish")
	})

	t.Run("multiple panics and unexpected exits recover", func(t *testing.T) {
		started := make(chan int)
		stopping := make(chan bool)
		stopped := make(chan bool)

		doStart, logError := makeDoStart(t, []string{"panic", "panic", "exit", "normal"}, started, stopping, stopped)
		isQuitting := func() bool {
			select {
			case <-stopping:
				return true
			default:
				return false
			}
		}

		GoWorker("job", doStart, isQuitting, logError)
		assertReceive(t, started, "doStart failed to start")
		assertReceive(t, started, "doStart failed to start the second time")
		assertReceive(t, started, "doStart failed to start the third time")
		assertReceive(t, started, "doStart failed to start the fourth time")
		close(stopping)
		assertReceive(t, stopped, "doStart failed to finish")
	})
}
