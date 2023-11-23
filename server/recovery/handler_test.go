package recovery_test

import (
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/recovery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockMetrics struct {
	callback func(name string)
}

func (m mockMetrics) ObserveGoroutineFailure(name string) {
	m.callback(name)
}

func assertReceive[X any](t *testing.T, c chan X, failureMessage string) {
	select {
	case <-c:
	case <-time.After(5 * time.Second):
		require.Fail(t, failureMessage)
	}
}

func TestWrap(t *testing.T) {
	t.Run("quitting normally does not recover", func(t *testing.T) {
		done := make(chan bool)
		callback := func() {
			close(done)
		}
		logError := func(msg string, keyValuePairs ...any) {
			require.Failf(t, "should not log error", "got %v", msg)
		}
		metrics := &mockMetrics{callback: func(name string) {
			require.Failf(t, "should not log metric", "got %v", name)
		}}

		wrapped := recovery.Wrap("callback", logError, metrics, callback)
		wrapped()
		assertReceive(t, done, "callback failed to finish")
	})

	t.Run("panic recovers, but goroutine terminated", func(t *testing.T) {
		logged := make(chan bool)
		reportedMetric := make(chan bool)
		callback := func() {
			panic("test")
		}
		logError := func(msg string, keyValuePairs ...any) {
			require.Equal(t, "Recovering from panic in callback", msg)
			close(logged)
		}
		metrics := &mockMetrics{callback: func(name string) {
			assert.Equal(t, "callback", name)
			close(reportedMetric)
		}}

		wrapped := recovery.Wrap("callback", logError, metrics, callback)
		wrapped()
		assertReceive(t, logged, "logger failed to log")
		assertReceive(t, reportedMetric, "metric failed to report")
	})
}

func TestGo(t *testing.T) {
	t.Run("quitting normally does not recover", func(t *testing.T) {
		running := make(chan bool)
		stop := make(chan bool)
		done := make(chan bool)
		callback := func() {
			close(running)
			<-stop
			close(done)
		}
		logError := func(msg string, keyValuePairs ...any) {
			require.Failf(t, "should not log error", "got %v", msg)
		}
		metrics := &mockMetrics{callback: func(name string) {
			require.Failf(t, "should not log metric", "got %v", name)
		}}

		recovery.Go("callback", logError, metrics, callback)
		assertReceive(t, running, "callback failed to start")
		close(stop)
		assertReceive(t, done, "callback failed to finish")
	})

	t.Run("panic recovers, but goroutine terminated", func(t *testing.T) {
		running := make(chan bool)
		doPanic := make(chan bool)
		logged := make(chan bool)
		reportedMetric := make(chan bool)
		callback := func() {
			close(running)
			<-doPanic
			panic("test")
		}
		logError := func(msg string, keyValuePairs ...any) {
			require.Equal(t, "Recovering from panic in callback", msg)
			close(logged)
		}
		metrics := &mockMetrics{callback: func(name string) {
			assert.Equal(t, "callback", name)
			close(reportedMetric)
		}}

		recovery.Go("callback", logError, metrics, callback)
		assertReceive(t, running, "callback failed to start")
		close(doPanic)
		assertReceive(t, logged, "logger failed to log")
		assertReceive(t, reportedMetric, "metric failed to report")
	})
}

func TestGoWorker(t *testing.T) {
	// makeDoStart simulates a start function with the defined sequence of actions, whether to
	// do a "normal" run, waiting for the signal to stop, to "panic", immediately crashing,
	// or to unexpectedly "exit".
	makeDoStart := func(t *testing.T, sequence []string, started chan int, stopping, stopped chan bool) (func(), func(string, ...any), recovery.Metrics) {
		count := 0

		callback := func() {
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
				panic("callback panic")

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

		reportedFailures := 0
		metrics := &mockMetrics{callback: func(name string) {
			assert.Equal(t, "job", name)
			reportedFailures++
		}}

		t.Cleanup(func() {
			expectedFailures := 0
			for _, event := range sequence {
				switch event {
				case "panic", "exit":
					expectedFailures++
				}
			}
			assert.Equal(t, expectedFailures, reportedFailures, "metrics did not capture expected failures")
		})

		return callback, logError, metrics
	}

	t.Run("quitting normally does not recover", func(t *testing.T) {
		started := make(chan int)
		stopping := make(chan bool)
		stopped := make(chan bool)

		callback, logError, metrics := makeDoStart(t, []string{"normal"}, started, stopping, stopped)
		isQuitting := func() bool {
			select {
			case <-stopping:
				return true
			default:
				return false
			}
		}

		recovery.GoWorker("job", logError, metrics, isQuitting, callback)
		assertReceive(t, started, "callback failed to start")
		close(stopping)
		assertReceive(t, stopped, "callback failed to finish")
	})

	t.Run("single panic recovers", func(t *testing.T) {
		started := make(chan int)
		stopping := make(chan bool)
		stopped := make(chan bool)

		callback, logError, metrics := makeDoStart(t, []string{"panic", "normal"}, started, stopping, stopped)
		isQuitting := func() bool {
			select {
			case <-stopping:
				return true
			default:
				return false
			}
		}

		recovery.GoWorker("job", logError, metrics, isQuitting, callback)
		assertReceive(t, started, "callback failed to start")
		assertReceive(t, started, "callback failed to start the second time")
		close(stopping)
		assertReceive(t, stopped, "callback failed to finish")
	})

	t.Run("unexpected exit recovers", func(t *testing.T) {
		started := make(chan int)
		stopping := make(chan bool)
		stopped := make(chan bool)

		callback, logError, metrics := makeDoStart(t, []string{"exit", "normal"}, started, stopping, stopped)
		isQuitting := func() bool {
			select {
			case <-stopping:
				return true
			default:
				return false
			}
		}

		recovery.GoWorker("job", logError, metrics, isQuitting, callback)
		assertReceive(t, started, "callback failed to start")
		assertReceive(t, started, "callback failed to start the second time")
		close(stopping)
		assertReceive(t, stopped, "callback failed to finish")
	})

	t.Run("multiple panics and unexpected exits recover", func(t *testing.T) {
		started := make(chan int)
		stopping := make(chan bool)
		stopped := make(chan bool)

		callback, logError, metrics := makeDoStart(t, []string{"panic", "panic", "exit", "normal"}, started, stopping, stopped)
		isQuitting := func() bool {
			select {
			case <-stopping:
				return true
			default:
				return false
			}
		}

		recovery.GoWorker("job", logError, metrics, isQuitting, callback)
		assertReceive(t, started, "callback failed to start")
		assertReceive(t, started, "callback failed to start the second time")
		assertReceive(t, started, "callback failed to start the third time")
		assertReceive(t, started, "callback failed to start the fourth time")
		close(stopping)
		assertReceive(t, stopped, "callback failed to finish")
	})
}
