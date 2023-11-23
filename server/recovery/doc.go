/*
Package recovery implements a set of helpers to guard against a panicking callback from
terminating the entire process. At its core, it uses the familiar Go paradigm to catch panics
in a deferred function call:

	func main() {
		go callMethodThatMightPanic()
	}

	func callMethodThatMightPanic() {
		defer func() {
			if r := recover(); r != nil {
				// log the panic and update related metrics
				return
			}
		}

		panic("more will, less might")
	}

This package primarily helps standardize around what to log and which metrics to update, while also
enabling certain advanced use cases.

When starting a goroutine that might panic, replace the go keyword with a call to recovery.Go:

	recovery.Go("callback_name", p.API.LogError, p.GetMetrics(), callMethodThatMightPanic)

When passing a callback to some other scheduling system, wrap with recovery.Wrap:

	job, err := cluster.Schedule(
		p.API,
		"job_name",
		cluster.MakeWaitForRoundedInterval(5 * time.Minute),
		recovery.Wrap("job_name", p.API.LogError, p.GetMetrics(), periodicJob),
	)

When starting a worker that should restart on failure instead of simply terminating, wrap
with recovery.GoWorker, passing an additional callback to help deterine when the worker is quitting
intentionally:

	recovery.GoWorker("worker_name", p.API.LogError, p.GetMetrics(), isQuitting, callback)
*/
package recovery
