package recovery

import (
	"fmt"
	"runtime/debug"
)

// Wrap wraps a callback with a handler that recovers from a panic, logging and tracking metrics.
func Wrap(name string, logError func(msg string, keyValuePairs ...any), metrics Metrics, callback func()) func() {
	return func() {
		defer func() {
			if r := recover(); r != nil {
				metrics.ObserveGoroutineFailure(name)
				logError(fmt.Sprintf("Recovering from panic in %s", name), "panic", r, "stack", string(debug.Stack()))
			}
		}()

		callback()
	}
}

// Go wraps the given callback with a handler to recover from a panic, and then invokes the
// callback in a goroutine.
func Go(name string, logError func(msg string, keyValuePairs ...any), metrics Metrics, callback func()) {
	go Wrap(name, logError, metrics, callback)()
}

// GoWorker wraps and invokes the given callback in a goroutine, automatically restarting in a new
// goroutine on any unrecovered panic or unexpected termination.
func GoWorker(name string, logError func(msg string, keyValuePairs ...any), metrics Metrics, isQuitting func() bool, callback func()) {
	var doRecoverableStart func()

	// doRecover is a helper function to recover from panics and restart the goroutine.
	doRecover := func() {
		if isQuitting() {
			return
		}

		metrics.ObserveGoroutineFailure(name)
		if r := recover(); r != nil {
			logError(fmt.Sprintf("Recovering from panic in %s", name), "panic", r, "stack", string(debug.Stack()))
		} else {
			logError(fmt.Sprintf("Recovering from unexpected exit in %s", name), "stack", string(debug.Stack()))
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
