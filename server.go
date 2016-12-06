package kuiperbelt

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
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
}

func NewWebSocketServer(c Config, s *Stats) *WebSocketServer {
	return &WebSocketServer{
		Config: c,
		Stats:  s,
	}
}

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
		http.Error(w, `{"result":"ERROR"}`, http.StatusInternalServerError)
	}
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

func (s *WebSocketServer) NewWebSocketHandler(resp *http.Response) func(ws *websocket.Conn) {
	defer resp.Body.Close()
	key := resp.Header.Get(s.Config.SessionHeader)
	b := new(bytes.Buffer)
	io.Copy(b, resp.Body)
	message := &Message{
		buf:         b,
		contentType: resp.Header.Get("Content-Type"),
	}
	return func(ws *websocket.Conn) {
		session, err := s.NewWebSocketSession(key, ws)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err.Error(),
			}).Error("connect error after upgrade")
			s.Stats.ConnectErrorEvent()
			return
		}
		AddSession(session)
		if message.buf.Len() > 0 {
			s.Stats.MessageEvent()
			session.SendMessage(message)
		}

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
	buf := make([]byte, ioBufferSize)
	_, err := io.CopyBuffer(new(blackholeWriter), s, buf)
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

func (s *WebSocketSession) SendMessage(message *Message) error {
	message.session = s.key
	return sendWsCodec.Send(s.Conn, message)
}

type Message struct {
	buf         *bytes.Buffer
	contentType string
	session     string
}

func messageMarshal(v interface{}) ([]byte, byte, error) {
	message, ok := v.(*Message)
	if !ok {
		return nil, 0x0, errors.New("value is not *kuiperbelt.Message.")
	}

	payloadType := byte(websocket.TextFrame)
	contentType := message.contentType
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
			"session": message.session,
			"message": message.buf.String(),
		}).Debug("write messege to session")
	case websocket.BinaryFrame:
		log.WithFields(log.Fields{
			"session": message.session,
			"message": hex.EncodeToString(message.buf.Bytes()),
		}).Debug("write messege to session")
	}

	return message.buf.Bytes(), payloadType, nil
}

type blackholeWriter struct {
}

func (bw *blackholeWriter) Write(p []byte) (int, error) {
	return len(p), nil
}
