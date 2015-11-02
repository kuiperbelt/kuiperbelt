package kuiperbelt

import (
	"bytes"
	"errors"
	"io"
	"log"
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
		if ce, ok := err.(ConnectCallbackError); ok {
			w.WriteHeader(ce.Status)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		io.WriteString(w, err.Error())
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
			log.Println("connect error:", err)
			return
		}
		AddSession(session)
		defer DelSession(session.Key())
		log.Println("connected key:", session.Key())
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
	err := s.Conn.Close()
	if err != nil {
		return err
	}
	s.closeCh <- struct{}{}
	return nil
}

func (s *WebSocketSession) WaitClose() {
	<-s.closeCh
}
