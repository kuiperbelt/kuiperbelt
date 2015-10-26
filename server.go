package kuiperbelt

import (
	"errors"
	"io"
	"log"
	"net/http"

	"golang.org/x/net/websocket"
)

var (
	sessionKeyNotExistError            = errors.New("session key is not exist.")
	connectCallbackIsNotAvailableError = errors.New("connect callback is not available.")
	callbackClient                     = new(http.Client)
)

type WebSocketServer struct {
	Config Config
}

func (s *WebSocketServer) WebSocketHandler(ws *websocket.Conn) {
	session, err := s.NewWebSocketSession(ws)
	if err != nil {
		log.Println("connect error:", err)
		return
	}
	AddSession(session)
	defer DelSession(session.Key())
	log.Println("connected key:", session.Key())
	session.WaitClose()
}

func (s *WebSocketServer) Register() {
	http.HandleFunc("/connect", func(w http.ResponseWriter, r *http.Request) {
		server := websocket.Server{Handler: websocket.Handler(s.WebSocketHandler)}
		server.ServeHTTP(w, r)
	})
}

type WebSocketSession struct {
	*websocket.Conn
	key     string
	closeCh chan struct{}
	Config  Config
}

func (s *WebSocketServer) NewWebSocketSession(ws *websocket.Conn) (*WebSocketSession, error) {
	req := ws.Request()
	defer req.Body.Close()
	callbackRequest, err := http.NewRequest(req.Method, s.Config.Callback.Connect, req.Body)
	if err != nil {
		return nil, err
	}
	callbackRequest.Header = req.Header
	resp, err := callbackClient.Do(callbackRequest)
	if err != nil {
		return nil, connectCallbackIsNotAvailableError
	}
	defer resp.Body.Close()
	key := resp.Header.Get(s.Config.SessionHeader)
	if key == "" {
		return nil, sessionKeyNotExistError
	}
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
