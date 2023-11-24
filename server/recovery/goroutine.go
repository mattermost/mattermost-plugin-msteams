package recovery

import (
	"fmt"
	"runtime/debug"
)

// Go runs a goroutine with a recovery handler that prevents the plugin from crashing while logging
// the panic and traceback.
func Go(name string, logError func(msg string, keyValuePairs ...any), callback func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logError(fmt.Sprintf("Recovering from panic in %s", name), "panic", r, "stack", string(debug.Stack()))
			}
		}()

		callback()
	}()
}
