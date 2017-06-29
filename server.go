package kuiperbelt

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/net/websocket"
	log "gopkg.in/Sirupsen/logrus.v0"
)

const (
	ENDPOINT_HEADER_NAME               = "X-Kuiperbelt-Endpoint"
	CALLBACK_CLIENT_MAX_CONNS_PER_HOST = 32
)

var (
	callbackClient = new(http.Client)
	sendWsCodec    = websocket.Codec{
		Marshal:   messageMarshal,
		Unmarshal: nil,
	}
)

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
		log.WithFields(log.Fields{
			"error": err.Error(),
		}).Error("connect error before upgrade")
		s.Stats.ConnectErrorEvent()
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
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return nil, err
	}
	callback.RawQuery = r.URL.RawQuery
	callbackRequest, err := http.NewRequest(r.Method, callback.String(), r.Body)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return nil, err
	}
	// copy all headers except "Connection", "Upgrade" and "Sec-Websocket*"
	for n, values := range r.Header {
		n := strings.ToLower(n)
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
		http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
		resp.Body.Close()
		return nil, errors.New(http.StatusText(http.StatusInternalServerError))
	}
	key := resp.Header.Get(s.Config.SessionHeader)
	if key == "" {
		resp.Body.Close()
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return nil, errors.New("kuiperbelt: session key header is not exist")
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
	message := Message{
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
		session.recvMessages()
	}, nil
}

func (s *WebSocketServer) NewWebSocketSession(key string, ws *websocket.Conn) (*WebSocketSession, error) {
	send := make(chan Message, s.Config.SendQueueSize)
	session := &WebSocketSession{
		ws:       ws,
		key:      key,
		server:   s,
		send:     send,
		closedch: make(chan struct{}),
	}

	return session, nil
}

func (s *WebSocketServer) Shutdown(ctx context.Context) error {
	msg := Message{LastWord: true}
	sessions := s.Pool.List()
	for _, s := range sessions {
		q := s.Send()
		if q == nil {
			continue
		}
		go func() {
			select {
			case q <- msg:
			case <-ctx.Done():
			}
		}()
	}

	for s.Stats.Connections() > 0 && s.Stats.ClosingConnections() > 0 {
		select {
		default:
		case <-ctx.Done():
			return ctx.Err()
		}
		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

type WebSocketSession struct {
	ws       *websocket.Conn
	key      string
	server   *WebSocketServer
	send     chan Message
	closed   uint32 // accessed atomically
	closedch chan struct{}
}

// Key returns the session key.
func (s *WebSocketSession) Key() string {
	return s.key
}

// Send returns the channel for sending messages.
func (s *WebSocketSession) Send() chan<- Message {
	if atomic.LoadUint32(&s.closed) != 0 {
		return nil
	}
	return s.send
}

// Close closes the session.
func (s *WebSocketSession) Close() error {
	if atomic.SwapUint32(&s.closed, 1) != 0 {
		return nil
	}
	s.server.Pool.Delete(s.key)
	close(s.closedch)
	if s.server.Config.Callback.Close != "" {
		s.server.Stats.ClosingEvent()
		go s.sendCloseCallback()
	}
	return s.ws.Close()
}

func (s *WebSocketSession) Closed() <-chan struct{} {
	return s.closedch
}

func (s *WebSocketSession) sendCloseCallback() {
	defer s.server.Stats.ClosedEvent()
	req, err := http.NewRequest("POST", s.server.Config.Callback.Close, nil)
	if err != nil {
		log.WithFields(log.Fields{
			"session": s.Key(),
			"error":   err.Error(),
		}).Error("cannot create close callback request.")
		return
	}
	req.Header.Add(s.server.Config.SessionHeader, s.Key())
	resp, err := callbackClient.Do(req)
	if err != nil {
		log.WithFields(log.Fields{
			"session": s.Key(),
			"error":   err.Error(),
		}).Error("failed send close callback request.")
	}
	defer resp.Body.Close()

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.WithFields(log.Fields{
			"session": s.Key(),
			"status":  resp.Status,
			"error":   err.Error(),
		}).Error("invalid close callback status.")
		return
	}
	if resp.StatusCode != http.StatusOK {
		log.WithFields(log.Fields{
			"session": s.Key(),
			"status":  resp.Status,
			"error":   string(buf),
		}).Error("invalid close callback status.")
	}
	log.WithFields(log.Fields{
		"session": s.Key(),
	}).Info("success close callback.")
}

func (s *WebSocketSession) sendMessages() {
	for {
		select {
		case msg := <-s.send:
			if err := sendWsCodec.Send(s.ws, msg); err != nil {
				s.server.Stats.MessageErrorEvent()
				s.Close()
				return
			}
			if msg.LastWord {
				s.Close()
				return
			}
		case <-s.closedch:
			return
		}
	}
}

func (s *WebSocketSession) recvMessages() {
	defer s.Close()
	buf := make([]byte, ioBufferSize)
	_, err := io.CopyBuffer(ioutil.Discard, s.ws, buf)
	if err == nil {
		return
	}
	// ignore closed session error
	if atomic.LoadUint32(&s.closed) != 0 {
		return
	}

	log.WithFields(log.Fields{
		"error": err.Error(),
	}).Error("watch close frame error")
}

func messageMarshal(v interface{}) ([]byte, byte, error) {
	message, ok := v.(Message)
	if !ok {
		return nil, 0x0, errors.New("kuiperbelt: value is not kuiperbelt.Message")
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
