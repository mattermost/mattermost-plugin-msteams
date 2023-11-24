package handlers

import (
	"runtime/debug"
)

type workerMetrics interface {
	ObserveGoroutineFailure()
}

// startWorker wraps and invokes the given callback in a goroutine, automatically restarting in a
// new goroutine on any unrecovered panic or unexpected termination.
func startWorker(logError func(msg string, keyValuePairs ...any), metrics workerMetrics, isQuitting func() bool, callback func()) {
	var doRecoverableStart func()

	// doRecover is a helper function to recover from panics and restart the goroutine.
	doRecover := func() {
		if isQuitting() {
			return
		}

		metrics.ObserveGoroutineFailure()
		if r := recover(); r != nil {
			logError("Recovering from panic", "panic", r, "stack", string(debug.Stack()))
		} else {
			logError("Recovering from unexpected exit", "stack", string(debug.Stack()))
		}

		go doRecoverableStart()
	}

	// doRecoverableStart delegates to callback, automatically recovering from panics by
	// launching a fresh goroutine on callback as needed.
	doRecoverableStart = func() {
		defer doRecover()
		callback()
	}

	go doRecoverableStart()
}
