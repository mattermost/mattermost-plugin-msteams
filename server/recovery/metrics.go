package recovery

type Metrics interface {
	ObserveGoroutineFailure(name string)
}

type mockMetrics struct {
	callback func(name string)
}

func (m mockMetrics) ObserveGoroutineFailure(name string) {
	m.callback(name)
}
