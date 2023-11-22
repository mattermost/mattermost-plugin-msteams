package handlers

import (
	"fmt"
	"runtime/debug"
)

func goWithRecovery(name string, doStart func(), isQuitting func() bool, logError func(msg string, keyValuePairs ...any)) {
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
