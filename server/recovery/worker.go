package recovery

import (
	"fmt"
	"runtime/debug"
)

// GoWorker runs a goroutine with a recovery handler that both prevents the plugin from crashing
// in the face of a panic, and also restarts the goroutine powering a worker unless the termination
// was expected.
func GoWorker(name string, doStart func(), isQuitting func() bool, logError func(msg string, keyValuePairs ...any)) {
	var doRecoverableStart func()

	// doRecover is a helper function to recover from panics and restart the goroutine.
	doRecover := func() {
		if isQuitting() {
			return
		}

		if r := recover(); r != nil {
			logError(fmt.Sprintf("Recovering from panic in %s", name), "panic", r, "stack", string(debug.Stack()))
		} else {
			logError(fmt.Sprintf("Recovering from unexpected exit in %s", name), "stack", string(debug.Stack()))
		}

		go doRecoverableStart()
	}

	// doRecoverableStart delegates to doStart, automatically recovering from panics by
	// launching a fresh goroutine on doStart as needed.
	doRecoverableStart = func() {
		defer doRecover()
		doStart()
	}

	go doRecoverableStart()
}
