package kuiperbelt

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	testRequestSessionHeader = "X-Kuiperbelt-Hogehoge"
	testSecWebSocketKey      = "AQIDBAUGBwgJCgsMDQ4PEA==" // from RFC sample
)

type testSuccessConnectCallbackServer struct {
	SessionKey   string
	IsCallbacked bool
}

func (s *testSuccessConnectCallbackServer) SuccessHandler(w http.ResponseWriter, r *http.Request) {
	s.IsCallbacked = true
	key := r.Header.Get(testRequestSessionHeader)
	s.SessionKey = key
	w.Header().Add(TestConfig.SessionHeader, "hogehoge")
	w.WriteHeader(http.StatusOK)
}

func (s *testSuccessConnectCallbackServer) FailHandler(w http.ResponseWriter, r *http.Request) {
	s.IsCallbacked = true
	key := r.Header.Get(testRequestSessionHeader)
	s.SessionKey = key
	w.WriteHeader(http.StatusForbidden)
	io.WriteString(w, "fail authorization!")
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
	req.Header.Add(testRequestSessionHeader, "hogehoge")

	return req, nil
}

func TestWebSocketServer__Handler__SuccessAuthorized(t *testing.T) {
	callbackServer := new(testSuccessConnectCallbackServer)
	tcc := httptest.NewServer(http.HandlerFunc(callbackServer.SuccessHandler))

	c := TestConfig
	c.Callback.Connect = tcc.URL

	server := WebSocketServer{c}

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

	if !callbackServer.IsCallbacked {
		t.Error("callback server doesn't receive request")
	}
	if callbackServer.SessionKey != "hogehoge" {
		t.Error("callback server doesn't receive session key:", callbackServer.SessionKey)
	}
}

func TestWebSocketServer__Handler__Failuthorized(t *testing.T) {
	callbackServer := new(testSuccessConnectCallbackServer)
	tcc := httptest.NewServer(http.HandlerFunc(callbackServer.FailHandler))

	c := TestConfig
	c.Callback.Connect = tcc.URL

	server := WebSocketServer{c}

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
		t.Error("callback message is not in response")
	}
}
