package recovery

type Metrics interface {
	ObserveGoroutineFailure(name string)
}
