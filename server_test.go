package kuiperbelt

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

const (
	testRequestSessionHeader = "X-Kuiperbelt-Hogehoge"
	testSecWebSocketKey      = "AQIDBAUGBwgJCgsMDQ4PEA==" // from RFC sample
	testHelloMessage         = "hello"
)

type testSuccessConnectCallbackServer struct {
	IsCallbacked bool
	Header       http.Header
}

func (s *testSuccessConnectCallbackServer) SuccessHandler(w http.ResponseWriter, r *http.Request) {
	s.IsCallbacked = true
	s.Header = r.Header
	w.Header().Add(TestConfig.SessionHeader, "hogehoge")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, testHelloMessage)
}

func (s *testSuccessConnectCallbackServer) FailHandler(w http.ResponseWriter, r *http.Request) {
	s.IsCallbacked = true
	s.Header = r.Header
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
	if callbackServer.Header.Get(testRequestSessionHeader) != "hogehoge" {
		t.Error(
			"callback server doesn't receive session key:",
			callbackServer.Header.Get(testRequestSessionHeader),
		)
	}
	if callbackServer.Header.Get(ENDPOINT_HEADER_NAME) != c.Endpoint {
		t.Error(
			"callback server doesn't receive endpoint name:",
			callbackServer.Header.Get(testRequestSessionHeader),
		)
	}
}

func TestWebSocketServer__Handler__FailAuthorized(t *testing.T) {
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

func TestWebSocketServer__Handler__CloseByClient(t *testing.T) {
	callbackServer := new(testSuccessConnectCallbackServer)
	tcc := httptest.NewServer(http.HandlerFunc(callbackServer.SuccessHandler))

	c := TestConfig
	c.Callback.Connect = tcc.URL

	server := WebSocketServer{c}

	tc := httptest.NewServer(http.HandlerFunc(server.Handler))

	wsURL := strings.Replace(tc.URL, "http://", "ws://", -1)
	wsConfig, err := websocket.NewConfig(wsURL, "http://localhost/")
	if err != nil {
		t.Fatal("cannot create connection config error:", err)
	}
	wsConfig.Header.Add(testRequestSessionHeader, "hogehoge")
	conn, err := websocket.DialConfig(wsConfig)
	if err != nil {
		t.Fatal("cannot connect error:", err)
	}

	io.CopyN(new(blackholeWriter), conn, int64(len([]byte("hello"))))

	_, err = GetSession("hogehoge")
	if err != nil {
		t.Fatal("cannot get session error:", err)
	}

	_, err = io.WriteString(conn, "barbar")
	if err != nil {
		t.Fatal("cannot write to connection error:", err)
	}

	err = conn.Close()
	if err != nil {
		t.Fatal("cannot close connection error:", err)
	}

	for i := 0; i < 5; i++ {
		time.Sleep(10 * time.Millisecond)

		_, err = GetSession("hogehoge")
		if err != nil {
			break
		}
	}

	if err != sessionNotFoundError {
		t.Error("not removed session:", err)
	}

}
