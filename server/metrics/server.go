package metrics

import (
	"net/http"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

// Service prometheus to run the server.
type Server struct {
	*http.Server
}

type ErrorLoggerWrapper struct {
}

func (el *ErrorLoggerWrapper) Println(v ...interface{}) {
	logrus.Warn("metric server error", v)
}

// NewMetricsHandler creates an HTTP handler to expose metrics.
func NewMetricsHandler(metricsService Metrics) http.Handler {
	return promhttp.HandlerFor(metricsService.GetRegistry(), promhttp.HandlerOpts{
		ErrorLog: &ErrorLoggerWrapper{},
	})
}

// Run will start the prometheus server.
func (h *Server) Run() error {
	return errors.Wrap(h.Server.ListenAndServe(), "prometheus ListenAndServe")
}

// Shutdown will shutdown the prometheus server.
func (h *Server) Shutdown() error {
	return errors.Wrap(h.Server.Close(), "prometheus Close")
}
