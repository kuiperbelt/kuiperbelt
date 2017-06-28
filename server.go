package kuiperbelt

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/websocket"

	"io"
	"io/ioutil"

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
	sendWsCodec                        = websocket.Codec{
		Marshal:   messageMarshal,
		Unmarshal: nil,
	}
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
	Stats  *Stats
	Pool   *SessionPool
}

func NewWebSocketServer(c Config, s *Stats, p *SessionPool) *WebSocketServer {
	return &WebSocketServer{
		Config: c,
		Stats:  s,
		Pool:   p,
	}
}

// Handler handles websocket connection requests.
func (s *WebSocketServer) Handler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	s.Stats.ConnectEvent()
	defer s.Stats.DisconnectEvent()

	resp, err := s.ConnectCallbackHandler(w, r)
	if err != nil {
		defer s.Stats.ConnectErrorEvent()
		status := http.StatusInternalServerError
		if ce, ok := err.(ConnectCallbackError); ok {
			status = ce.Status
		}
		http.Error(w, http.StatusText(status), status)

		log.WithFields(log.Fields{
			"status": status,
			"error":  err.Error(),
		}).Error("connect error before upgrade")
		return
	}

	handler, err := s.NewWebSocketHandler(resp)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	server := websocket.Server{Handler: websocket.Handler(handler)}
	server.ServeHTTP(w, r)
}

func (s *WebSocketServer) Register() {
	callbackClient.Transport = &http.Transport{
		MaxIdleConnsPerHost: CALLBACK_CLIENT_MAX_CONNS_PER_HOST,
	}
	http.HandleFunc("/connect", s.Handler)
	http.HandleFunc("/stats", s.StatsHandler)
}

func (s *WebSocketServer) StatsHandler(w http.ResponseWriter, r *http.Request) {
	var err error

	switch r.FormValue("format") {
	case "tsv", "txt", "text":
		w.Header().Set("Content-Type", "text/plain")
		err = s.Stats.DumpText(w)
	default:
		w.Header().Set("Content-Type", "application/json")
		err = s.Stats.Dump(w)
	}
	if err != nil {
		log.WithFields(log.Fields{
			"error": err.Error(),
		}).Error("stats dump failed")
		panic(http.ErrAbortHandler)
	}
}

func (s *WebSocketServer) ConnectCallbackHandler(w http.ResponseWriter, r *http.Request) (*http.Response, error) {
	callback, err := url.ParseRequestURI(s.Config.Callback.Connect)
	if err != nil {
		return nil, ConnectCallbackError{http.StatusInternalServerError, err}
	}
	callback.RawQuery = r.URL.RawQuery
	callbackRequest, err := http.NewRequest(r.Method, callback.String(), r.Body)
	if err != nil {
		return nil, ConnectCallbackError{http.StatusInternalServerError, err}
	}
	// copy all headers except "Connection", "Upgrade" and "Sec-Websocket*"
	for _n, values := range r.Header {
		n := strings.ToLower(_n)
		if n == "connection" || n == "upgrade" || strings.HasPrefix(n, "sec-websocket") {
			continue
		}
		for _, value := range values {
			callbackRequest.Header.Add(n, value)
		}
	}
	callbackRequest.Header.Add(ENDPOINT_HEADER_NAME, s.Config.Endpoint)
	resp, err := callbackClient.Do(callbackRequest)
	if err != nil {
		return nil, ConnectCallbackError{
			http.StatusBadGateway,
			fmt.Errorf("%s %s", connectCallbackIsNotAvailableError, err),
		}
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

func (s *WebSocketServer) NewWebSocketHandler(resp *http.Response) (func(ws *websocket.Conn), error) {
	defer resp.Body.Close()
	key := resp.Header.Get(s.Config.SessionHeader)
	var b bytes.Buffer
	if _, err := b.ReadFrom(resp.Body); err != nil {
		return nil, err
	}
	message := &Message{
		Body:        b.Bytes(),
		ContentType: resp.Header.Get("Content-Type"),
		Session:     key,
	}
	return func(ws *websocket.Conn) {
		// register a new websocket sesssion.
		session, err := s.NewWebSocketSession(key, ws)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err.Error(),
			}).Error("connect error after upgrade")
			s.Stats.ConnectErrorEvent()
			return
		}
		s.Pool.Add(session)
		defer s.Pool.Delete(session.Key())

		// send the first message.
		if len(message.Body) > 0 {
			s.Stats.MessageEvent()
			if err := sendWsCodec.Send(ws, message); err != nil {
				s.Stats.MessageErrorEvent()
				return
			}
		}
		go session.sendMessages()
		go session.recvMessages()
		session.waitForClose()
	}, nil
}

func (s *WebSocketServer) NewWebSocketSession(key string, ws *websocket.Conn) (*WebSocketSession, error) {
	send := make(chan Message, 4)
	session := &WebSocketSession{
		ws:     ws,
		key:    key,
		server: s,
		send:   send,
		closed: make(chan struct{}),
	}

	return session, nil
}

type WebSocketSession struct {
	ws     *websocket.Conn
	key    string
	server *WebSocketServer
	send   chan Message
	closed chan struct{}
}

// Key returns the session key.
func (s *WebSocketSession) Key() string {
	return s.key
}

// Send returns the channel for sending messages.
func (s *WebSocketSession) Send() chan<- Message {
	return s.send
}

// Close closes the session.
func (s *WebSocketSession) Close() error {
	close(s.closed)
	return s.ws.Close()
}

func (s *WebSocketSession) sendMessages() {
	for {
		select {
		case msg := <-s.send:
			if err := sendWsCodec.Send(s.ws, msg); err != nil {
				s.Close()
				return
			}
		case <-s.closed:
			return
		}
	}
}

func (s *WebSocketSession) recvMessages() {
	buf := make([]byte, ioBufferSize)
	io.CopyBuffer(ioutil.Discard, s.ws, buf)
}

func (s *WebSocketSession) waitForClose() {
	<-s.closed
}

func messageMarshal(v interface{}) ([]byte, byte, error) {
	message, ok := v.(*Message)
	if !ok {
		return nil, 0x0, errors.New("kuiperbelt: value is not *kuiperbelt.Message")
	}

	// parce Content-Type
	payloadType := byte(websocket.TextFrame)
	contentType := message.ContentType
	if i := strings.Index(contentType, ";"); i >= 0 {
		contentType = contentType[0:i]
	}
	contentType = strings.TrimSpace(contentType)
	if strings.EqualFold(contentType, "application/octet-stream") {
		payloadType = websocket.BinaryFrame
	}

	switch payloadType {
	case websocket.TextFrame:
		log.WithFields(log.Fields{
			"session": message.Session,
			"message": string(message.Body),
		}).Debug("write messege to session")
	case websocket.BinaryFrame:
		log.WithFields(log.Fields{
			"session": message.Session,
			"message": hex.EncodeToString(message.Body),
		}).Debug("write messege to session")
	}

	return message.Body, payloadType, nil
}
