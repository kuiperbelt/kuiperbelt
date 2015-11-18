package kuiperbelt

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/net/websocket"

	log "gopkg.in/Sirupsen/logrus.v0"
)

const (
	ENDPOINT_HEADER_NAME               = "X-Kuiperbelt-Endpoint"
	CALLBACK_CLIENT_MAX_CONNS_PER_HOST = 32
)

var (
	sessionKeyNotExistError            = errors.New("session key is not exist.")
	connectCallbackIsNotAvailableError = errors.New("connect callback is not available.")
	callbackClient                     = new(http.Client)
)

type ConnectCallbackError struct {
	Status int
	error  error
}

func (e ConnectCallbackError) Error() string {
	return e.error.Error()
}

type WebSocketServer struct {
	Config Config
}

func (s *WebSocketServer) Handler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	resp, err := s.ConnectCallbackHandler(w, r)
	if err != nil {
		status := http.StatusInternalServerError
		if ce, ok := err.(ConnectCallbackError); ok {
			status = ce.Status
		}
		w.WriteHeader(status)
		io.WriteString(w, err.Error())

		log.WithFields(log.Fields{
			"status": status,
			"error":  err.Error(),
		}).Error("connect error before upgrade")
		return
	}

	server := websocket.Server{Handler: websocket.Handler(s.NewWebSocketHandler(resp))}
	server.ServeHTTP(w, r)
}

func (s *WebSocketServer) Register() {
	callbackClient.Transport = &http.Transport{
		MaxIdleConnsPerHost: CALLBACK_CLIENT_MAX_CONNS_PER_HOST,
	}

	http.HandleFunc("/connect", s.Handler)
}

func (s *WebSocketServer) ConnectCallbackHandler(w http.ResponseWriter, r *http.Request) (*http.Response, error) {
	callbackUrl, err := url.ParseRequestURI(s.Config.Callback.Connect)
	if err != nil {
		return nil, ConnectCallbackError{http.StatusInternalServerError, err}
	}
	callbackUrl.RawQuery = r.URL.RawQuery
	callbackRequest, err := http.NewRequest(r.Method, callbackUrl.String(), r.Body)
	if err != nil {
		return nil, ConnectCallbackError{http.StatusInternalServerError, err}
	}
	callbackRequest.Header = r.Header
	callbackRequest.Header.Add(ENDPOINT_HEADER_NAME, s.Config.Endpoint)
	resp, err := callbackClient.Do(callbackRequest)
	if err != nil {
		return nil, ConnectCallbackError{http.StatusBadGateway, connectCallbackIsNotAvailableError}
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		b := new(bytes.Buffer)
		b.ReadFrom(resp.Body)
		return nil, ConnectCallbackError{resp.StatusCode, errors.New(b.String())}
	}
	key := resp.Header.Get(s.Config.SessionHeader)
	if key == "" {
		resp.Body.Close()
		return nil, ConnectCallbackError{http.StatusBadRequest, sessionKeyNotExistError}
	}

	return resp, nil
}

func (s *WebSocketServer) NewWebSocketHandler(resp *http.Response) func(ws *websocket.Conn) {
	defer resp.Body.Close()
	key := resp.Header.Get(s.Config.SessionHeader)
	b := new(bytes.Buffer)
	io.Copy(b, resp.Body)
	return func(ws *websocket.Conn) {
		session, err := s.NewWebSocketSession(key, ws)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err.Error(),
			}).Error("connect error after upgrade")
			return
		}
		AddSession(session)
		io.Copy(session, b)

		defer DelSession(session.Key())
		log.WithFields(log.Fields{
			"session": session.Key(),
		}).Info("connected session")
		go session.WatchClose()
		session.WaitClose()
	}
}

func (s *WebSocketServer) NewWebSocketSession(key string, ws *websocket.Conn) (*WebSocketSession, error) {
	closeCh := make(chan struct{})
	onceClose := sync.Once{}
	session := &WebSocketSession{ws, key, closeCh, s.Config, onceClose, new(atomic.Value)}

	return session, nil
}

type WebSocketSession struct {
	*websocket.Conn
	key             string
	closeCh         chan struct{}
	Config          Config
	onceClose       sync.Once
	isNotifiedClose *atomic.Value
}

func (s *WebSocketSession) Key() string {
	return s.key
}

func (s *WebSocketSession) NotifiedClose(isNotified bool) {
	s.isNotifiedClose.Store(isNotified)
}

func (s *WebSocketSession) Close() error {
	var err error
	s.onceClose.Do(func() {
		err = DelSession(s.Key())
		if err != nil {
			return
		}
		err = s.Conn.Close()
		if err != nil {
			return
		}
		s.closeCh <- struct{}{}
	})
	return nil
}

func (s *WebSocketSession) WaitClose() {
	<-s.closeCh
}

func (s *WebSocketSession) WatchClose() {
	defer s.Close()
	defer func() { go s.SendCloseCallback() }()
	_, err := io.Copy(new(blackholeWriter), s)
	if err == nil {
		return
	}
	// ignore closed session error
	if strings.HasSuffix(err.Error(), "use of closed network connection") {
		return
	}

	log.WithFields(log.Fields{
		"error": err.Error(),
	}).Error("watch close frame error")
}

func (s *WebSocketSession) SendCloseCallback() {
	// cancel sending when not set callback url
	if s.Config.Callback.Close == "" {
		return
	}

	// cancel sending when notified closed already
	if isNotified, ok := s.isNotifiedClose.Load().(bool); ok && isNotified {
		return
	}

	callbackRequest, err := http.NewRequest("POST", s.Config.Callback.Close, nil)
	if err != nil {
		log.WithFields(log.Fields{
			"session": s.Key(),
			"error":   err.Error(),
		}).Error("cannot create close callback request.")
		return
	}

	callbackRequest.Header.Add(s.Config.SessionHeader, s.Key())
	resp, err := callbackClient.Do(callbackRequest)
	if err != nil {
		log.WithFields(log.Fields{
			"session": s.Key(),
			"error":   err.Error(),
		}).Error("failed send close callback request.")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b := new(bytes.Buffer)
		b.ReadFrom(resp.Body)
		log.WithFields(log.Fields{
			"session": s.Key(),
			"status":  resp.Status,
			"error":   b.String(),
		}).Error("invalid close callback status.")
		return
	}

	log.WithFields(log.Fields{
		"session": s.Key(),
	}).Info("success close callback.")
}

type blackholeWriter struct {
}

func (bw *blackholeWriter) Write(p []byte) (int, error) {
	return len(p), nil
}
