package kuiperbelt

import (
	"bytes"
	"errors"
	log "gopkg.in/Sirupsen/logrus.v0"
	"io"
	"net/http"

	"golang.org/x/net/websocket"
)

const (
	ENDPOINT_HEADER_NAME = "X-Kuiperbelt-Endpoint"
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
	http.HandleFunc("/connect", s.Handler)
}

type WebSocketSession struct {
	*websocket.Conn
	key     string
	closeCh chan struct{}
	Config  Config
}

func (s *WebSocketServer) ConnectCallbackHandler(w http.ResponseWriter, r *http.Request) (*http.Response, error) {
	callbackRequest, err := http.NewRequest(r.Method, s.Config.Callback.Connect, r.Body)
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
		return nil, ConnectCallbackError{http.StatusBadRequest, sessionKeyNotExistError}
	}

	return resp, nil
}

func (s *WebSocketServer) NewWebSocketHandler(resp *http.Response) func(ws *websocket.Conn) {
	return func(ws *websocket.Conn) {
		session, err := s.NewWebSocketSession(resp, ws)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err.Error(),
			}).Error("connect error after upgrade")
			return
		}
		AddSession(session)
		defer DelSession(session.Key())
		log.WithFields(log.Fields{
			"session_key": session.Key(),
		}).Info("connected session")
		go session.WatchClose()
		session.WaitClose()
	}
}

func (s *WebSocketServer) NewWebSocketSession(resp *http.Response, ws *websocket.Conn) (*WebSocketSession, error) {
	key := resp.Header.Get(s.Config.SessionHeader)
	defer resp.Body.Close()
	closeCh := make(chan struct{})
	session := &WebSocketSession{ws, key, closeCh, s.Config}
	io.Copy(session, resp.Body)

	return session, nil
}

func (s *WebSocketSession) Key() string {
	return s.key
}

func (s *WebSocketSession) Close() error {
	err := DelSession(s.Key())
	if err != nil {
		return err
	}
	err = s.Conn.Close()
	if err != nil {
		return err
	}
	s.closeCh <- struct{}{}
	return nil
}

func (s *WebSocketSession) WaitClose() {
	<-s.closeCh
}

func (s *WebSocketSession) WatchClose() {
	defer s.Close()
	_, err := io.Copy(new(blackholeWriter), s)
	if err == io.EOF {
		return
	}
	if err != nil {
		log.WithFields(log.Fields{
			"error": err.Error(),
		}).Error("watch close frame error")
	}
}

type blackholeWriter struct {
}

func (bw *blackholeWriter) Write(p []byte) (int, error) {
	return len(p), nil
}
