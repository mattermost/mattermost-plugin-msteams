package main

import (
	"net/http"
)

type StatusRecorder struct {
	http.ResponseWriter
	Status int
}

func (r *StatusRecorder) WriteHeader(status int) {
	r.Status = status
	r.ResponseWriter.WriteHeader(status)
}

func (p *Plugin) metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p.metricsService != nil {
			p.metricsService.IncrementHTTPRequests()
			recorder := &StatusRecorder{
				ResponseWriter: w,
				Status:         200,
			}
			next.ServeHTTP(recorder, r)
			p.metricsService.IncrementHTTPErrors()
		} else {
			next.ServeHTTP(w, r)
		}
	})
}
