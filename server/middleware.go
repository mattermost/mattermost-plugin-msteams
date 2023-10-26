package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

type StatusRecorder struct {
	http.ResponseWriter
	Status int
}

func (r *StatusRecorder) WriteHeader(status int) {
	r.Status = status
	r.ResponseWriter.WriteHeader(status)
}

func (a *API) metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.p.GetMetrics().IncrementHTTPRequests()
		recorder := &StatusRecorder{
			ResponseWriter: w,
			Status:         http.StatusOK,
		}

		now := time.Now()
		next.ServeHTTP(recorder, r)
		elapsed := float64(time.Since(now)) / float64(time.Second)

		if recorder.Status < 200 || recorder.Status > 299 {
			a.p.GetMetrics().IncrementHTTPErrors()
		}

		var routeMatch mux.RouteMatch
		a.router.Match(r, &routeMatch)
		endpoint := "unknown"
		if routeMatch.Route != nil {
			var err error
			endpoint, err = routeMatch.Route.GetPathTemplate()
			if err != nil {
				endpoint = "unknown"
			}
		}
		a.p.GetMetrics().ObserveAPIEndpointDuration(endpoint, r.Method, strconv.Itoa(recorder.Status), elapsed)
	})
}
