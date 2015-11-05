package kuiperbelt

import (
	"net/http"

	log "gopkg.in/Sirupsen/logrus.v0"
)

type loggingHandler struct {
	handler http.Handler
}

type responseLogger struct {
	w      http.ResponseWriter
	status int
	size   int
}

func (l *responseLogger) Header() http.Header {
	return l.w.Header()
}

func (l *responseLogger) Write(b []byte) (int, error) {
	if l.status == 0 {
		// The status will be StatusOK if WriteHeader has not been called yet
		l.status = http.StatusOK
	}
	size, err := l.w.Write(b)
	l.size += size
	return size, err
}

func (l *responseLogger) WriteHeader(s int) {
	l.w.WriteHeader(s)
	l.status = s
}

func NewLoggingHandler(h http.Handler) http.Handler {
	return loggingHandler{handler: h}
}

func (h loggingHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	logger := &responseLogger{w: w}
	h.handler.ServeHTTP(logger, req)

	url := *req.URL
	method := req.Method

	log.WithFields(log.Fields{
		"method": method,
		"url":    url.String(),
		"status": logger.status,
		"size":   logger.size,
	}).Info("access")
}
