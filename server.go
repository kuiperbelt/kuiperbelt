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

	"go.uber.org/zap"
	xws "golang.org/x/net/websocket"
)

const (
	ENDPOINT_HEADER_NAME               = "X-Kuiperbelt-Endpoint"
	CALLBACK_CLIENT_MAX_CONNS_PER_HOST = 32
)

var (
	callbackClient = new(http.Client)
	sendWsCodec    = xws.Codec{
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
		Log.Error("connect error before upgrade",
			zap.Error(err),
		)
		s.Stats.ConnectErrorEvent()
		return
	}

	handler, err := s.NewWebSocketHandler(resp)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	server := xws.Server{Handler: xws.Handler(handler)}
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
		Log.Error("stats dump failed", zap.Error(err))
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
			Log.Debug("set_request_header",
				zap.String("name", n), zap.String("value", value))
			callbackRequest.Header.Add(n, value)
		}
	}
	for name, value := range s.Config.ProxySetHeader {
		if value == "" {
			callbackRequest.Header.Del(name)
		} else {
			callbackRequest.Header.Set(name, value)
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

func (s *WebSocketServer) NewWebSocketHandler(resp *http.Response) (func(ws *xws.Conn), error) {
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
	return func(ws *xws.Conn) {
		// register a new websocket sesssion.
		session, err := s.NewWebSocketSession(key, ws)
		if err != nil {
			Log.Error("connect error after upgrade", zap.Error(err))
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

func (s *WebSocketServer) NewWebSocketSession(key string, ws *xws.Conn) (*WebSocketSession, error) {
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
	ws       *xws.Conn
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
		Log.Error("cannot create close callback request.",
			zap.Error(err),
			zap.String("session", s.Key()),
		)
		return
	}

	req.Header.Add(s.server.Config.SessionHeader, s.Key())
	for name, value := range s.server.Config.ProxySetHeader {
		if value == "" {
			req.Header.Del(name)
		} else {
			req.Header.Set(name, value)
		}
	}
	resp, err := callbackClient.Do(req)

	if err != nil {
		Log.Error("failed send close callback request.",
			zap.Error(err),
			zap.String("session", s.Key()),
		)
	}
	defer resp.Body.Close()

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Log.Error("invalid close callback status.",
			zap.String("session", s.Key()),
			zap.String("status", resp.Status),
			zap.String("error", err.Error()),
		)
		return
	}
	if resp.StatusCode != http.StatusOK {
		Log.Error("invalid close callback status.",
			zap.String("session", s.Key()),
			zap.String("status", resp.Status),
			zap.String("error", string(buf)),
		)
	}

	Log.Info("success close callback.",
		zap.String("session", s.Key()),
	)
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

	Log.Error("watch close frame error",
		zap.Error(err),
	)
}

func messageMarshal(v interface{}) ([]byte, byte, error) {
	message, ok := v.(Message)
	if !ok {
		return nil, 0x0, errors.New("kuiperbelt: value is not kuiperbelt.Message")
	}

	// parce Content-Type
	payloadType := byte(xws.TextFrame)
	contentType := message.ContentType
	if i := strings.Index(contentType, ";"); i >= 0 {
		contentType = contentType[0:i]
	}
	contentType = strings.TrimSpace(contentType)
	if strings.EqualFold(contentType, "application/octet-stream") {
		payloadType = xws.BinaryFrame
	}

	switch payloadType {
	case xws.TextFrame:
		Log.Debug("write messege to session",
			zap.String("session", message.Session),
			zap.String("message", string(message.Body)),
		)
	case xws.BinaryFrame:
		Log.Debug("write messege to session",
			zap.String("session", message.Session),
			zap.String("message", hex.EncodeToString(message.Body)),
		)
	}

	return message.Body, payloadType, nil
}
