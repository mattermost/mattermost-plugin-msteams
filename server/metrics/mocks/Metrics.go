// Code generated by mockery v2.18.0. DO NOT EDIT.

package mocks

import (
	prometheus "github.com/prometheus/client_golang/prometheus"
	mock "github.com/stretchr/testify/mock"
)

// Metrics is an autogenerated mock type for the Metrics type
type Metrics struct {
	mock.Mock
}

// DecrementChangeEventQueueLength provides a mock function with given fields: changeType
func (_m *Metrics) DecrementChangeEventQueueLength(changeType string) {
	_m.Called(changeType)
}

// GetRegistry provides a mock function with given fields:
func (_m *Metrics) GetRegistry() *prometheus.Registry {
	ret := _m.Called()

	var r0 *prometheus.Registry
	if rf, ok := ret.Get(0).(func() *prometheus.Registry); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*prometheus.Registry)
		}
	}

	return r0
}

// IncrementChangeEventQueueLength provides a mock function with given fields: changeType
func (_m *Metrics) IncrementChangeEventQueueLength(changeType string) {
	_m.Called(changeType)
}

// IncrementHTTPErrors provides a mock function with given fields:
func (_m *Metrics) IncrementHTTPErrors() {
	_m.Called()
}

// IncrementHTTPRequests provides a mock function with given fields:
func (_m *Metrics) IncrementHTTPRequests() {
	_m.Called()
}

// ObserveAPIEndpointDuration provides a mock function with given fields: handler, method, statusCode, elapsed
func (_m *Metrics) ObserveAPIEndpointDuration(handler string, method string, statusCode string, elapsed float64) {
	_m.Called(handler, method, statusCode, elapsed)
}

// ObserveChangeEvent provides a mock function with given fields: changeType, discardedReason
func (_m *Metrics) ObserveChangeEvent(changeType string, discardedReason string) {
	_m.Called(changeType, discardedReason)
}

// ObserveChangeEventQueueCapacity provides a mock function with given fields: count
func (_m *Metrics) ObserveChangeEventQueueCapacity(count int64) {
	_m.Called(count)
}

// ObserveChangeEventQueueRejectedTotal provides a mock function with given fields:
func (_m *Metrics) ObserveChangeEventQueueRejectedTotal() {
	_m.Called()
}

// ObserveConfirmedMessage provides a mock function with given fields: source, isDirectMessage
func (_m *Metrics) ObserveConfirmedMessage(source string, isDirectMessage bool) {
	_m.Called(source, isDirectMessage)
}

// ObserveConnectedUsers provides a mock function with given fields: count
func (_m *Metrics) ObserveConnectedUsers(count int64) {
	_m.Called(count)
}

// ObserveFile provides a mock function with given fields: action, source, discardedReason, isDirectMessage
func (_m *Metrics) ObserveFile(action string, source string, discardedReason string, isDirectMessage bool) {
	_m.Called(action, source, discardedReason, isDirectMessage)
}

// ObserveFiles provides a mock function with given fields: action, source, discardedReason, isDirectMessage, count
func (_m *Metrics) ObserveFiles(action string, source string, discardedReason string, isDirectMessage bool, count int64) {
	_m.Called(action, source, discardedReason, isDirectMessage, count)
}

// ObserveLifecycleEvent provides a mock function with given fields: lifecycleEventType
func (_m *Metrics) ObserveLifecycleEvent(lifecycleEventType string) {
	_m.Called(lifecycleEventType)
}

// ObserveLinkedChannels provides a mock function with given fields: count
func (_m *Metrics) ObserveLinkedChannels(count int64) {
	_m.Called(count)
}

// ObserveMSGraphClientMethodDuration provides a mock function with given fields: method, success, elapsed
func (_m *Metrics) ObserveMSGraphClientMethodDuration(method string, success string, elapsed float64) {
	_m.Called(method, success, elapsed)
}

// ObserveMessage provides a mock function with given fields: action, source, isDirectMessage
func (_m *Metrics) ObserveMessage(action string, source string, isDirectMessage bool) {
	_m.Called(action, source, isDirectMessage)
}

// ObserveReaction provides a mock function with given fields: action, source, isDirectMessage
func (_m *Metrics) ObserveReaction(action string, source string, isDirectMessage bool) {
	_m.Called(action, source, isDirectMessage)
}

// ObserveStoreMethodDuration provides a mock function with given fields: method, success, elapsed
func (_m *Metrics) ObserveStoreMethodDuration(method string, success string, elapsed float64) {
	_m.Called(method, success, elapsed)
}

// ObserveSubscription provides a mock function with given fields: action
func (_m *Metrics) ObserveSubscription(action string) {
	_m.Called(action)
}

// ObserveSyntheticUsers provides a mock function with given fields: count
func (_m *Metrics) ObserveSyntheticUsers(count int64) {
	_m.Called(count)
}

// ObserveUpstreamUsers provides a mock function with given fields: count
func (_m *Metrics) ObserveUpstreamUsers(count int64) {
	_m.Called(count)
}

type mockConstructorTestingTNewMetrics interface {
	mock.TestingT
	Cleanup(func())
}

// NewMetrics creates a new instance of Metrics. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMetrics(t mockConstructorTestingTNewMetrics) *Metrics {
	mock := &Metrics{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
