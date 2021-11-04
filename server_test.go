package kuiperbelt

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const (
	testRequestSessionHeader = "X-Kuiperbelt-Hogehoge"
	testSecWebSocketKey      = "AQIDBAUGBwgJCgsMDQ4PEA==" // from RFC sample
	testHelloMessage         = "hello"
)

type testSuccessConnectCallbackServer struct {
	mu           sync.Mutex
	isCallbacked bool
	isClosed     bool
	header       http.Header
}

func (s *testSuccessConnectCallbackServer) IsCallbacked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isCallbacked
}

func (s *testSuccessConnectCallbackServer) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isClosed
}

func (s *testSuccessConnectCallbackServer) Header() http.Header {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.header
}

func (s *testSuccessConnectCallbackServer) SuccessHandler(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.isCallbacked = true
	s.header = r.Header
	s.mu.Unlock()

	if r.Header.Get("Connection") == "upgrade" {
		http.Error(w, "Connection header is upgrade", http.StatusBadRequest)
		return
	}
	if r.Header.Get("Upgrade") != "" {
		http.Error(w, "Upgrade header is sent", http.StatusBadRequest)
		return
	}
	for n := range r.Header {
		if strings.HasPrefix(strings.ToLower(n), "sec-websocket") {
			http.Error(w, n+" header is sent", http.StatusBadRequest)
			return
		}
	}
	if r.Header.Get("X-Forwarded-For") != "" {
		http.Error(w, "X-Forwarded-For must be removed", http.StatusBadRequest)
		return
	}
	if r.Header.Get("X-Foo") != "Foo" {
		http.Error(w, "X-Foo is unexpected", http.StatusBadRequest)
		return
	}

	w.Header().Add(TestConfig.SessionHeader, "hogehoge")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, testHelloMessage)
}

func (s *testSuccessConnectCallbackServer) FailHandler(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.isCallbacked = true
	s.header = r.Header
	s.mu.Unlock()
	w.WriteHeader(http.StatusForbidden)
	io.WriteString(w, "fail authorization!")
}

func (s *testSuccessConnectCallbackServer) CloseHandler(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.isClosed = true
	s.header = r.Header
	s.mu.Unlock()
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "")
}

func (s *testSuccessConnectCallbackServer) SlowHandler(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.isCallbacked = true
	s.header = r.Header
	s.mu.Unlock()
	time.Sleep(10 * time.Second)
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "slow response")
}

func newTestWebSocketRequest(url string) (*http.Request, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Upgrade", "websocket")
	req.Header.Add("Connection", "upgrade")
	req.Header.Add("Sec-WebSocket-Protocol", "kuiperbelt")
	req.Header.Add("Sec-WebSocket-Version", "13")
	req.Header.Add("Sec-WebSocket-Key", testSecWebSocketKey)
	req.Header.Add("X-Forwarded-For", "192.168.1.1")
	req.Header.Add(testRequestSessionHeader, "hogehoge")

	return req, nil
}

func TestWebSocketServer__Handler__SuccessAuthorized(t *testing.T) {
	var pool SessionPool
	callbackServer := new(testSuccessConnectCallbackServer)
	tcc := httptest.NewServer(http.HandlerFunc(callbackServer.SuccessHandler))

	c := TestConfig
	c.Callback.Connect = tcc.URL

	server := NewWebSocketServer(c, NewStats(), &pool)

	tc := httptest.NewServer(http.HandlerFunc(server.Handler))

	req, err := newTestWebSocketRequest(tc.URL)
	if err != nil {
		t.Fatal("cannot create request error:", err)
	}

	client := new(http.Client)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal("to server upgrade request unexpected error:", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Error("unexpected status code:", resp.StatusCode)
	}

	if !callbackServer.IsCallbacked() {
		t.Error("callback server doesn't receive request")
	}
	if callbackServer.Header().Get(testRequestSessionHeader) != "hogehoge" {
		t.Error(
			"callback server doesn't receive session key:",
			callbackServer.Header().Get(testRequestSessionHeader),
		)
	}
	if callbackServer.Header().Get(ENDPOINT_HEADER_NAME) != c.Endpoint {
		t.Error(
			"callback server doesn't receive endpoint name:",
			callbackServer.Header().Get(testRequestSessionHeader),
		)
	}
}

func TestWebSocketServer__Handler__FailAuthorized(t *testing.T) {
	var pool SessionPool
	callbackServer := new(testSuccessConnectCallbackServer)
	tcc := httptest.NewServer(http.HandlerFunc(callbackServer.FailHandler))

	c := TestConfig
	c.Callback.Connect = tcc.URL

	server := NewWebSocketServer(c, NewStats(), &pool)

	tc := httptest.NewServer(http.HandlerFunc(server.Handler))

	req, err := newTestWebSocketRequest(tc.URL)
	if err != nil {
		t.Fatal("cannot create request error:", err)
	}

	client := new(http.Client)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal("to server upgrade request unexpected error:", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Error("unexpected status code:", resp.StatusCode)
	}
	b := new(bytes.Buffer)
	b.ReadFrom(resp.Body)

	if b.String() != "fail authorization!" {
		t.Errorf("callback message is not in response: %s", b.String())
	}
}

func TestWebSocketServer__Handler__CloseByClient(t *testing.T) {
	var pool SessionPool
	callbackServer := new(testSuccessConnectCallbackServer)
	tcc1 := httptest.NewServer(http.HandlerFunc(callbackServer.SuccessHandler))
	tcc2 := httptest.NewServer(http.HandlerFunc(callbackServer.CloseHandler))

	c := TestConfig
	c.Callback.Connect = tcc1.URL
	c.Callback.Close = tcc2.URL

	server := NewWebSocketServer(c, NewStats(), &pool)

	tc := httptest.NewServer(http.HandlerFunc(server.Handler))

	dialer := websocket.Dialer{}
	wsURL := strings.Replace(tc.URL, "http://", "ws://", -1)
	conn, _, err := dialer.Dial(wsURL, http.Header{testRequestSessionHeader: []string{"hogehoge"}})
	if err != nil {
		t.Fatal("cannot connect error:", err)
	}

	conn.ReadMessage() // pull and drop initial message
	_, err = pool.Get("hogehoge")
	if err != nil {
		t.Fatal("cannot get session error:", err)
	}

	err = conn.WriteMessage(websocket.TextMessage, []byte("barbar"))
	if err != nil {
		t.Fatal("cannot write to connection error:", err)
	}

	err = conn.Close()
	if err != nil {
		t.Fatal("cannot close connection error:", err)
	}

	for i := 0; i < 5; i++ {
		time.Sleep(10 * time.Millisecond)

		_, err = pool.Get("hogehoge")
		if err != nil {
			break
		}
	}

	if err != errSessionNotFound {
		t.Error("not removed session:", err)
	}

	if !callbackServer.IsClosed() {
		t.Error("not receive close callback")
	}

}

func TestWebSocketSession__IdleTimeout(t *testing.T) {
	closeHandlerEvent := make(chan struct{})

	callbackServer := new(testSuccessConnectCallbackServer)
	tccConnect := httptest.NewServer(http.HandlerFunc(callbackServer.SuccessHandler))
	tccClose := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callbackServer.CloseHandler(w, r)
			closeHandlerEvent <- struct{}{}
		}),
	)

	c := TestConfig
	c.IdleTimeout = time.Second * 2
	c.Callback.Connect = tccConnect.URL
	c.Callback.Close = tccClose.URL

	var pool SessionPool
	server := NewWebSocketServer(c, NewStats(), &pool)
	tc := httptest.NewServer(http.HandlerFunc(server.Handler))

	dialer := websocket.Dialer{}
	wsURL := strings.Replace(tc.URL, "http://", "ws://", -1)
	conn, _, err := dialer.Dial(wsURL, http.Header{testRequestSessionHeader: []string{"hogehoge"}})
	if err != nil {
		t.Fatal("cannot connect error:", err)
	}
	receivedPongCh := make(chan struct{}, 3)
	conn.SetPongHandler(func(msg string) error {
		receivedPongCh <- struct{}{}
		if msg != "hello" {
			t.Errorf("pong response is invalid: got: %s, expected: hello", msg)
			return fmt.Errorf("pong response is invalid: got: %s, expected: hello", msg)
		}
		return nil
	})

	go func() {
		var err error
		for err == nil {
			_, _, err = conn.ReadMessage() // drop messages
		}
	}()

	// Must not reach connection timeout when 1sec + ping(reset deadline) + 1sec
	time.Sleep(time.Second * 1)
	err = conn.WriteControl(websocket.PingMessage, []byte("hello"), time.Now().Add(time.Second))
	if err != nil {
		t.Errorf("fail send ping error: %s", err)
	}
	time.Sleep(time.Second * 1)
	if len(receivedPongCh) != 1 {
		t.Errorf("not receive pong: %d", len(receivedPongCh))
	}
	if len(closeHandlerEvent) != 0 {
		t.Fatal("rearch connection timeout with ping")
	}

	select {
	case <-closeHandlerEvent:
	case <-time.After(time.Millisecond * 1500):
		t.Error("connection timeout is not working")
	}
}

func TestWebSocketServer__Handler__SlowCallback(t *testing.T) {
	var pool SessionPool
	callbackServer := new(testSuccessConnectCallbackServer)
	tcc := httptest.NewServer(http.HandlerFunc(callbackServer.SlowHandler))

	c := TestConfig
	c.Callback.Connect = tcc.URL

	server := NewWebSocketServer(c, NewStats(), &pool)

	tc := httptest.NewServer(http.HandlerFunc(server.Handler))

	req, err := newTestWebSocketRequest(tc.URL)
	if err != nil {
		t.Fatal("cannot create request error:", err)
	}

	client := new(http.Client)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal("to server upgrade request unexpected error:", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Error("unexpected status code:", resp.StatusCode)
	}
}

func TestWebSocketSession__CallbackReceiver(t *testing.T) {
	c := TestConfig

	receiveHandlerEvent := make(chan struct{})
	receiveBuf := &bytes.Buffer{}
	callbackServer := new(testSuccessConnectCallbackServer)
	tccConnect := httptest.NewServer(http.HandlerFunc(callbackServer.SuccessHandler))
	tccReceive := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Content-Type") != "text/plain" {
				t.Errorf("received message Content-Type is not text/plain: %s", r.Header.Get("Content-Type"))
			}
			if r.Header.Get(c.SessionHeader) != "hogehoge" {
				t.Errorf("received message Session Key is not match: %s", r.Header.Get(c.SessionHeader))
			}
			io.Copy(receiveBuf, r.Body)
			defer r.Body.Close()
			receiveHandlerEvent <- struct{}{}
		}),
	)

	c.Callback.Connect = tccConnect.URL
	c.Callback.Receive = tccReceive.URL

	var pool SessionPool
	server := NewWebSocketServer(c, NewStats(), &pool)
	tc := httptest.NewServer(http.HandlerFunc(server.Handler))

	dialer := websocket.Dialer{}
	wsURL := strings.Replace(tc.URL, "http://", "ws://", -1)
	conn, _, err := dialer.Dial(wsURL, http.Header{testRequestSessionHeader: []string{"hogehoge"}})
	if err != nil {
		t.Fatal("cannot connect error:", err)
	}

	go func() {
		var err error
		for err == nil {
			_, _, err = conn.ReadMessage() // drop messages
		}
	}()

	err = conn.WriteMessage(websocket.TextMessage, []byte("hello receive message"))
	if err != nil {
		t.Errorf("unexpected error on write message from client: %s", err)
	}
	<-receiveHandlerEvent

	if receiveBuf.String() != "hello receive message" {
		t.Errorf("calling back message is not match: %s", receiveBuf.String())
	}
}

func TestWebSocketSession__Handler__SuccessEstablished(t *testing.T) {
	c := TestConfig

	establishHandlerEvent := make(chan struct{}, 1)
	callbackServer := new(testSuccessConnectCallbackServer)
	tccConnect := httptest.NewServer(http.HandlerFunc(callbackServer.SuccessHandler))
	tccEstablish := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get(c.SessionHeader) != "hogehoge" {
				t.Errorf("received message Session Key is not match: %s", r.Header.Get(c.SessionHeader))
			}
			establishHandlerEvent <- struct{}{}
		}),
	)

	c.Callback.Connect = tccConnect.URL
	c.Callback.Establish = tccEstablish.URL

	var pool SessionPool
	server := NewWebSocketServer(c, NewStats(), &pool)
	tc := httptest.NewServer(http.HandlerFunc(server.Handler))

	dialer := websocket.Dialer{}
	wsURL := strings.Replace(tc.URL, "http://", "ws://", -1)
	conn, _, err := dialer.Dial(wsURL, http.Header{testRequestSessionHeader: []string{"fugafuga"}})
	if err != nil {
		t.Fatal("cannot connect error:", err)
	}

	_, p, _ := conn.ReadMessage()
	if string(p) != testHelloMessage {
		t.Fatal("cannot receive the first message:", err)
	}
	select {
	case <- establishHandlerEvent:
	default:
		t.Fatal("establish callback server doesn't receive request:")
	}

	_, err = pool.Get("hogehoge")
	if err != nil {
		t.Fatal("cannot get session error:", err)
	}

	err = conn.WriteMessage(websocket.TextMessage, []byte("barbar"))
	if err != nil {
		t.Fatal("cannot write to connection error:", err)
	}

	if !callbackServer.IsCallbacked() {
		t.Error("callback server doesn't receive request")
	}
	if callbackServer.Header().Get(testRequestSessionHeader) != "fugafuga" {
		t.Error(
			"callback server doesn't receive session key:",
			callbackServer.Header().Get(testRequestSessionHeader),
		)
	}
	if callbackServer.Header().Get(ENDPOINT_HEADER_NAME) != c.Endpoint {
		t.Error(
			"callback server doesn't receive endpoint name:",
			callbackServer.Header().Get(testRequestSessionHeader),
		)
	}
}

func TestWebSocketSession__Handler__FailEstablished(t *testing.T) {
	c := TestConfig

	callbackServer := new(testSuccessConnectCallbackServer)
	tccConnect := httptest.NewServer(http.HandlerFunc(callbackServer.SuccessHandler))
	tccEstablish := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get(c.SessionHeader) != "hogehoge" {
				t.Errorf("received message Session Key is not match: %s", r.Header.Get(c.SessionHeader))
			}
			w.WriteHeader(http.StatusInternalServerError)
		}),
	)

	c.Callback.Connect = tccConnect.URL
	c.Callback.Establish = tccEstablish.URL

	var pool SessionPool
	server := NewWebSocketServer(c, NewStats(), &pool)
	tc := httptest.NewServer(http.HandlerFunc(server.Handler))

	dialer := websocket.Dialer{}
	wsURL := strings.Replace(tc.URL, "http://", "ws://", -1)
	conn, _, err := dialer.Dial(wsURL, http.Header{testRequestSessionHeader: []string{"fugafuga"}})
	if err != nil {
		t.Fatal("cannot connect error:", err)
	}

	_, _, err = conn.ReadMessage()
	if !websocket.IsCloseError(err, websocket.CloseAbnormalClosure) {
		t.Fatal("should be closed error:", err)
	}
	_, err = pool.Get("hogehoge")
	if err != errSessionNotFound {
		t.Fatal("shouldn't save session error:", err)
	}
}
