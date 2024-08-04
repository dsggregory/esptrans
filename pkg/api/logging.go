package api

import (
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

type statusRecorder struct {
	http.ResponseWriter
	Status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.Status = status
	r.ResponseWriter.WriteHeader(status)
}

type loggingMiddleware struct {
	handler http.Handler
	log     *logrus.Entry
}

// LoggingResponseStatus contains the status info from a http request
type LoggingResponseStatus struct {
	Status int
	Dur    time.Duration
}

func getRemote(r *http.Request) string {
	remote := r.RemoteAddr
	// Azure containers gateway includes this header from envoy
	if x := r.Header.Get("X-Envoy-External-Address"); x != "" {
		remote = x
	}
	return remote
}

// ServeLogging shared with other logging middlewares to call the handler and measure response time
func (l *loggingMiddleware) ServeLogging(w http.ResponseWriter, r *http.Request) *LoggingResponseStatus {
	start := time.Now()
	recorder := statusRecorder{
		ResponseWriter: w,
		Status:         http.StatusOK,
	}
	// call the route which is now wrapped in the status recorder
	l.handler.ServeHTTP(&recorder, r)
	lr := LoggingResponseStatus{
		Status: recorder.Status,
		Dur:    time.Since(start),
	}
	l.log.WithFields(logrus.Fields{
		"httpMethod":     r.Method,
		"httpPath":       r.URL.Path,
		"httpStatus":     lr.Status,
		"httpRemoteAddr": getRemote(r),
		"httpReferer":    r.Referer(),
		"dur":            lr.Dur,
	}).Info("http request log")

	return &lr
}

// ServeHTTP satisfies the http.Handler interface of logAuditMiddleware and performs request logging
func (l *loggingMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l.log.WithFields(logrus.Fields{
		"httpMethod":     r.Method,
		"httpPath":       r.URL.Path,
		"httpRemoteAddr": getRemote(r),
		"httpReferer":    r.Referer(),
	}).Debug("http new request")

	_ = l.ServeLogging(w, r)
}

// NewLoggingMiddleware create a new logging middleware to log the request status.
//
// Example:
//
//	mux.Handle("/endpoint", NewLoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
//	  w.WriteHeader(http.StatusNotImplemented)
//	})))
func NewLoggingMiddleware(handlerToWrap http.Handler) http.Handler {
	mw := &loggingMiddleware{
		handler: handlerToWrap,
		log:     logrus.WithField("state", "reqlog"),
	}
	return http.HandlerFunc(mw.ServeHTTP)
}
