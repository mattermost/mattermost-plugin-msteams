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

// ObserveChangeEventQueueCapacity provides a mock function with given fields: count
func (_m *Metrics) ObserveChangeEventQueueCapacity(count int64) {
	_m.Called(count)
}

// ObserveChangeEventTotal provides a mock function with given fields: changeType
func (_m *Metrics) ObserveChangeEventTotal(changeType string) {
	_m.Called(changeType)
}

// ObserveConnectedUsersTotal provides a mock function with given fields: count
func (_m *Metrics) ObserveConnectedUsersTotal(count int64) {
	_m.Called(count)
}

// ObserveFileCount provides a mock function with given fields: action, source, discardedReason, isDirectMessage
func (_m *Metrics) ObserveFileCount(action string, source string, discardedReason string, isDirectMessage bool) {
	_m.Called(action, source, discardedReason, isDirectMessage)
}

// ObserveFilesCount provides a mock function with given fields: action, source, discardedReason, isDirectMessage, count
func (_m *Metrics) ObserveFilesCount(action string, source string, discardedReason string, isDirectMessage bool, count int64) {
	_m.Called(action, source, discardedReason, isDirectMessage, count)
}

// ObserveLifecycleEventTotal provides a mock function with given fields: lifecycleEventType
func (_m *Metrics) ObserveLifecycleEventTotal(lifecycleEventType string) {
	_m.Called(lifecycleEventType)
}

// ObserveLinkedChannelsTotal provides a mock function with given fields: count
func (_m *Metrics) ObserveLinkedChannelsTotal(count int64) {
	_m.Called(count)
}

// ObserveMessagesCount provides a mock function with given fields: action, source, isDirectMessage
func (_m *Metrics) ObserveMessagesCount(action string, source string, isDirectMessage bool) {
	_m.Called(action, source, isDirectMessage)
}

// ObserveProcessedChangeEventTotal provides a mock function with given fields: changeType, discardedReason
func (_m *Metrics) ObserveProcessedChangeEventTotal(changeType string, discardedReason string) {
	_m.Called(changeType, discardedReason)
}

// ObserveReactionsCount provides a mock function with given fields: action, source, isDirectMessage
func (_m *Metrics) ObserveReactionsCount(action string, source string, isDirectMessage bool) {
	_m.Called(action, source, isDirectMessage)
}

// ObserveSyntheticUsersTotal provides a mock function with given fields: count
func (_m *Metrics) ObserveSyntheticUsersTotal(count int64) {
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
