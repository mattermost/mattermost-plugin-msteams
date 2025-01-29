// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockMetrics struct {
	callback func()
}

func (m mockMetrics) ObserveGoroutineFailure() {
	m.callback()
}

func assertReceive[X any](t *testing.T, c chan X, failureMessage string) {
	select {
	case <-c:
	case <-time.After(5 * time.Second):
		require.Fail(t, failureMessage)
	}
}

func TestStartWorker(t *testing.T) {
	// makeDoStart simulates a start function with the defined sequence of actions, whether to
	// do a "normal" run, waiting for the signal to stop, to "panic", immediately crashing,
	// or to unexpectedly "exit".
	makeDoStart := func(t *testing.T, sequence []string, started chan int, stopping, stopped chan bool) (func(), func(), func(string, ...any), workerMetrics) {
		count := 0
		doStartStopped := make(chan bool)

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
				close(doStartStopped)

			case "panic":
				panic("doStart panic")

			case "exit":
				return

			default:
				require.Failf(t, "unexpected sequence", "got %s", sequence[count])
			}
		}

		doQuit := func() {
			select {
			case <-doStartStopped:
				// All good, we were asked to stop.
				close(stopped)
			default:
				require.Failf(t, "unexpected stop", "stopped not yet closed")
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
				require.Equal(t, "Recovering from panic", msg)
			case "exit":
				require.Equal(t, "Recovering from unexpected exit", msg)
			default:
				require.Failf(t, "unexpected sequence", "got %s", sequence[actualCount])
			}
		}

		reportedFailures := 0
		metrics := &mockMetrics{callback: func() {
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

		return doStart, doQuit, logError, metrics
	}

	t.Run("quitting normally does not recover", func(t *testing.T) {
		started := make(chan int)
		stopping := make(chan bool)
		stopped := make(chan bool)

		doStart, doQuit, logError, metrics := makeDoStart(t, []string{"normal"}, started, stopping, stopped)
		isQuitting := func() bool {
			select {
			case <-stopping:
				return true
			default:
				return false
			}
		}

		startWorker(logError, metrics, isQuitting, doStart, doQuit)
		assertReceive(t, started, "worker failed to start")
		close(stopping)
		assertReceive(t, stopped, "worker failed to finish")
	})

	t.Run("single panic recovers", func(t *testing.T) {
		started := make(chan int)
		stopping := make(chan bool)
		stopped := make(chan bool)

		doStart, doQuit, logError, metrics := makeDoStart(t, []string{"panic", "normal"}, started, stopping, stopped)
		isQuitting := func() bool {
			select {
			case <-stopping:
				return true
			default:
				return false
			}
		}

		startWorker(logError, metrics, isQuitting, doStart, doQuit)
		assertReceive(t, started, "worker failed to start")
		assertReceive(t, started, "worker failed to start the second time")
		close(stopping)
		assertReceive(t, stopped, "worker failed to finish")
	})

	t.Run("unexpected exit recovers", func(t *testing.T) {
		started := make(chan int)
		stopping := make(chan bool)
		stopped := make(chan bool)

		doStart, doQuit, logError, metrics := makeDoStart(t, []string{"exit", "normal"}, started, stopping, stopped)
		isQuitting := func() bool {
			select {
			case <-stopping:
				return true
			default:
				return false
			}
		}

		startWorker(logError, metrics, isQuitting, doStart, doQuit)
		assertReceive(t, started, "worker failed to start")
		assertReceive(t, started, "worker failed to start the second time")
		close(stopping)
		assertReceive(t, stopped, "worker failed to finish")
	})

	t.Run("multiple panics and unexpected exits recover", func(t *testing.T) {
		started := make(chan int)
		stopping := make(chan bool)
		stopped := make(chan bool)

		doStart, doQuit, logError, metrics := makeDoStart(t, []string{"panic", "panic", "exit", "normal"}, started, stopping, stopped)
		isQuitting := func() bool {
			select {
			case <-stopping:
				return true
			default:
				return false
			}
		}

		startWorker(logError, metrics, isQuitting, doStart, doQuit)
		assertReceive(t, started, "worker failed to start")
		assertReceive(t, started, "worker failed to start the second time")
		assertReceive(t, started, "worker failed to start the third time")
		assertReceive(t, started, "worker failed to start the fourth time")
		close(stopping)
		assertReceive(t, stopped, "worker failed to finish")
	})
}
