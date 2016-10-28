package kuiperbelt

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/net/websocket"
)

func TestProxySendHandlerFunc__BulkSend(t *testing.T) {
	s1 := &TestSession{new(bytes.Buffer), "hogehoge", false, false}
	s2 := &TestSession{new(bytes.Buffer), "fugafuga", false, false}

	AddSession(s1)
	AddSession(s2)

	tc := TestConfig
	p := NewProxy(tc, NewStats())
	ts := httptest.NewServer(http.HandlerFunc(p.SendHandlerFunc))
	defer ts.Close()

	req, err := http.NewRequest("POST", ts.URL, bytes.NewBufferString("test message"))
	if err != nil {
		t.Fatal("proxy handler new request unexpected error:", err)
	}
	req.Header.Add(tc.SessionHeader, "hogehoge")
	req.Header.Add(tc.SessionHeader, "fugafuga")

	client := new(http.Client)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal("proxy handler request unexpected error:", err)
	}
	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)
	result := struct {
		Result string `json:"result"`
	}{}
	err = dec.Decode(&result)
	if err != nil {
		t.Fatal("proxy handler response unexpected error:", err)
	}
	if result.Result != "OK" {
		t.Fatalf("proxy handler response unexpected response: %+v", result)
	}

	if s1.String() != "test message" {
		t.Fatalf("proxy handler s1 not receive message: %s", s1.String())
	}
	if s2.String() != "test message" {
		t.Fatalf("proxy handler s2 not receive message: %s", s2.String())
	}
}

func TestProxySendHandlerFunc__SendInBinary(t *testing.T) {
	callbackServer := new(testSuccessConnectCallbackServer)
	tcc := httptest.NewServer(http.HandlerFunc(callbackServer.SuccessHandler))

	tc := TestConfig
	tc.Callback.Connect = tcc.URL
	st := NewStats()
	p := NewProxy(tc, st)
	ts := httptest.NewServer(http.HandlerFunc(p.SendHandlerFunc))
	server := NewWebSocketServer(tc, st)
	th := httptest.NewServer(http.HandlerFunc(server.Handler))

	wsURL := strings.Replace(th.URL, "http://", "ws://", -1)
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

	codec := &websocket.Codec{
		Unmarshal: func(data []byte, payloadType byte, v interface{}) error {
			rb, _ := v.(*byte)
			*rb = payloadType
			return nil
		},
		Marshal: nil,
	}

	req, err := http.NewRequest("POST", ts.URL, bytes.NewBuffer([]byte("hogehoge")))
	if err != nil {
		t.Fatal("creadrequest unexpected error:", err)
	}
	req.Header.Add("Content-Type", "APPLICATION/octet-stream ;param=foobar")
	req.Header.Add(tc.SessionHeader, "hogehoge")
	_, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal("send request unexpected error:", err)
	}

	var rb byte
	codec.Receive(conn, &rb)
	if rb != websocket.BinaryFrame {
		t.Fatal("receved message is not binary frame:", rb)
	}
}

func TestProxyCloseHandlerFunc__BulkClose(t *testing.T) {
	s1 := &TestSession{new(bytes.Buffer), "hogehoge", false, false}
	s2 := &TestSession{new(bytes.Buffer), "fugafuga", false, false}

	AddSession(s1)
	AddSession(s2)

	tc := TestConfig
	p := NewProxy(tc, NewStats())
	ts := httptest.NewServer(http.HandlerFunc(p.CloseHandlerFunc))
	defer ts.Close()

	req, err := http.NewRequest("POST", ts.URL, bytes.NewBufferString("test message"))
	if err != nil {
		t.Fatal("proxy handler new request unexpected error:", err)
	}
	req.Header.Add(tc.SessionHeader, "hogehoge")
	req.Header.Add(tc.SessionHeader, "fugafuga")

	client := new(http.Client)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal("proxy handler request unexpected error:", err)
	}
	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)
	result := struct {
		Result string `json:"result"`
	}{}
	err = dec.Decode(&result)
	if err != nil {
		t.Fatal("proxy handler response unexpected error:", err)
	}
	if result.Result != "OK" {
		t.Fatalf("proxy handler response unexpected response: %+v", result)
	}

	if s1.String() != "test message" {
		t.Fatalf("proxy handler s1 is not receive message: %s", s1.String())
	}
	if s2.String() != "test message" {
		t.Fatalf("proxy handler s2 is not receive message: %s", s2.String())
	}

	if !s1.isClosed {
		t.Fatalf("proxy handler s1 is not closed")
	}
	if !s2.isClosed {
		t.Fatalf("proxy handler s1 is not closed")
	}
}

func TestProxySendHandlerFunc__StrictBroadcastFalse(t *testing.T) {
	s1 := &TestSession{new(bytes.Buffer), "hogehoge", false, false}
	AddSession(s1)

	tc := TestConfig // StrictBroadcast == false (default)
	p := Proxy{tc}
	ts := httptest.NewServer(http.HandlerFunc(p.SendHandlerFunc))
	defer ts.Close()

	req, err := http.NewRequest("POST", ts.URL, bytes.NewBufferString("test message"))
	if err != nil {
		t.Fatal("proxy handler new request unexpected error:", err)
	}
	req.Header.Add(tc.SessionHeader, "hogehog") // missing "e" invalid session
	client := new(http.Client)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal("proxy handler request unexpected error:", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// StrictBroadcast == false returns always OK
		t.Fatal("proxy handler response unexpected status:", resp.StatusCode)
	}
	dec := json.NewDecoder(resp.Body)
	result := struct {
		Result string        `json:"result"`
		Errors []interface{} `json:"errors,omitempty"`
	}{}
	err = dec.Decode(&result)
	if err != nil {
		t.Fatal("proxy handler response unexpected error:", err)
	}
	if result.Result != "OK" {
		t.Fatalf("proxy handler response unexpected response: %+v", result)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("proxy handler response unexpected response: %+v", result)
	}
	if s1.String() == "test message" {
		t.Fatalf("proxy handler s1 must not receive message: %s", s1.String())
	}
}

func TestProxySendHandlerFunc__StrictBroadcastTrue1(t *testing.T) {
	s1 := &TestSession{new(bytes.Buffer), "hogehoge", false, false}
	AddSession(s1)

	tc := TestConfig
	tc.StrictBroadcast = true
	p := Proxy{tc}
	ts := httptest.NewServer(http.HandlerFunc(p.SendHandlerFunc))
	defer ts.Close()

	req, err := http.NewRequest("POST", ts.URL, bytes.NewBufferString("test message"))
	if err != nil {
		t.Fatal("proxy handler new request unexpected error:", err)
	}
	req.Header.Add(tc.SessionHeader, "hogehog") // missing "e" invalid session
	client := new(http.Client)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal("proxy handler request unexpected error:", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		// StrictBroadcast == true returns Bad Request if requested sessions missing
		t.Fatal("proxy handler response unexpected status:", resp.StatusCode)
	}

	dec := json.NewDecoder(resp.Body)
	result := struct {
		Result string        `json:"result"`
		Errors []interface{} `json:"errors,omitempty"`
	}{}
	err = dec.Decode(&result)
	if err != nil {
		t.Fatal("proxy handler response unexpected error:", err)
	}
	if result.Result != "NG" {
		t.Fatalf("proxy handler response unexpected response: %+v", result)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("proxy handler response unexpected response: %+v", result)
	}
	if s1.String() == "test message" {
		t.Fatalf("proxy handler s1 must not receive message: %s", s1.String())
	}
}

func TestProxySendHandlerFunc__StrictBroadcastTrue2(t *testing.T) {
	s1 := &TestSession{new(bytes.Buffer), "hogehoge", false, false}
	s2 := &TestSession{new(bytes.Buffer), "fugafuga", false, false}
	AddSession(s1)
	AddSession(s2)

	tc := TestConfig
	tc.StrictBroadcast = true
	p := Proxy{tc}
	ts := httptest.NewServer(http.HandlerFunc(p.SendHandlerFunc))
	defer ts.Close()

	req, err := http.NewRequest("POST", ts.URL, bytes.NewBufferString("test message"))
	if err != nil {
		t.Fatal("proxy handler new request unexpected error:", err)
	}
	req.Header.Add(tc.SessionHeader, "hogehog") // missing "e" invalid session
	req.Header.Add(tc.SessionHeader, "fugafuga")
	client := new(http.Client)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal("proxy handler request unexpected error:", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		// StrictBroadcast == true returns Bad Request if requested sessions missing
		t.Fatal("proxy handler response unexpected status:", resp.StatusCode)
	}

	dec := json.NewDecoder(resp.Body)
	result := struct {
		Result string        `json:"result"`
		Errors []interface{} `json:"errors,omitempty"`
	}{}
	err = dec.Decode(&result)
	if err != nil {
		t.Fatal("proxy handler response unexpected error:", err)
	}
	if result.Result != "NG" {
		t.Fatalf("proxy handler response unexpected response: %+v", result)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("proxy handler response unexpected response: %+v", result)
	}
	if s1.String() == "test message" {
		t.Fatalf("proxy handler s1 must not receive message: %s", s1.String())
	}
	if s2.String() == "test message" {
		t.Fatalf("proxy handler s2 must not receive message: %s", s1.String())
	}
}

func TestProxyPingHandlerFunc(t *testing.T) {
	tc := TestConfig
	p := Proxy{tc}
	ts := httptest.NewServer(http.HandlerFunc(p.PingHandlerFunc))
	defer ts.Close()

	req, err := http.NewRequest("GET", ts.URL, nil)
	if err != nil {
		t.Fatal("proxy handler new request unexpected error:", err)
	}
	client := new(http.Client)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal("proxy handler request unexpected error:", err)
	}
	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)
	result := struct {
		Result string `json:"result"`
	}{}
	err = dec.Decode(&result)
	if err != nil {
		t.Fatal("proxy handler response unexpected error:", err)
	}
	if result.Result != "OK" {
		t.Fatalf("proxy handler response unexpected response: %+v", result)
	}
}
