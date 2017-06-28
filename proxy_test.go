package kuiperbelt

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/net/websocket"
)

func TestProxySendHandlerFunc__BulkSend(t *testing.T) {
	var pool SessionPool
	s1 := &TestSession{
		key:  "hogehoge",
		send: make(chan Message, 4),
	}
	s2 := &TestSession{
		key:  "fugafuga",
		send: make(chan Message, 4),
	}

	pool.Add(s1)
	pool.Add(s2)

	tc := TestConfig
	p := NewProxy(tc, NewStats(), &pool)
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

	msg1 := <-s1.send
	if string(msg1.Body) != "test message" {
		t.Errorf("proxy handler s1 not receive message: %s", string(msg1.Body))
	}
	if msg1.LastWord {
		t.Error("s1 receives last word")
	}
	msg2 := <-s2.send
	if string(msg2.Body) != "test message" {
		t.Errorf("proxy handler s2 not receive message: %s", string(msg2.Body))
	}
	if msg2.LastWord {
		t.Error("s2 receives last word")
	}
}

func TestProxySendHandlerFunc__SendInBinary(t *testing.T) {
	var pool SessionPool
	callbackServer := new(testSuccessConnectCallbackServer)
	tcc := httptest.NewServer(http.HandlerFunc(callbackServer.SuccessHandler))

	tc := TestConfig
	tc.Callback.Connect = tcc.URL
	st := NewStats()
	p := NewProxy(tc, st, &pool)
	ts := httptest.NewServer(http.HandlerFunc(p.SendHandlerFunc))
	server := NewWebSocketServer(tc, st, &pool)
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

	// ignore hello message
	var hello string
	websocket.Message.Receive(conn, &hello)

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
	var pool SessionPool
	s1 := &TestSession{
		key:  "hogehoge",
		send: make(chan Message, 4),
	}
	s2 := &TestSession{
		key:  "fugafuga",
		send: make(chan Message, 4),
	}

	pool.Add(s1)
	pool.Add(s2)

	tc := TestConfig
	p := NewProxy(tc, NewStats(), &pool)
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

	msg1 := <-s1.send
	if string(msg1.Body) != "test message" {
		t.Errorf("proxy handler s1 not receive message: %s", string(msg1.Body))
	}
	if !msg1.LastWord {
		t.Error("s1 does not receive last word")
	}
	msg2 := <-s2.send
	if string(msg2.Body) != "test message" {
		t.Errorf("proxy handler s2 not receive message: %s", string(msg2.Body))
	}
	if !msg2.LastWord {
		t.Error("s2 does not receive last word")
	}
}

func TestProxySendHandlerFunc__StrictBroadcastFalse(t *testing.T) {
	var pool SessionPool
	s1 := &TestSession{
		key:  "hogehoge",
		send: make(chan Message, 4),
	}
	pool.Add(s1)

	tc := TestConfig // StrictBroadcast == false (default)
	p := NewProxy(tc, NewStats(), &pool)
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
	select {
	case msg := <-s1.send:
		t.Fatalf("proxy handler s1 must not receive message: %#v", msg)
	default:
	}
}

func TestProxySendHandlerFunc__StrictBroadcastTrue1(t *testing.T) {
	var pool SessionPool
	s1 := &TestSession{
		key:  "hogehoge",
		send: make(chan Message, 4),
	}
	pool.Add(s1)

	tc := TestConfig
	tc.StrictBroadcast = true
	p := NewProxy(tc, NewStats(), &pool)
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
	select {
	case msg := <-s1.send:
		t.Fatalf("proxy handler s1 must not receive message: %#v", msg)
	default:
	}
}

func TestProxySendHandlerFunc__StrictBroadcastTrue2(t *testing.T) {
	var pool SessionPool
	s1 := &TestSession{
		key:  "hogehoge",
		send: make(chan Message, 4),
	}
	s2 := &TestSession{
		key:  "fugafuga",
		send: make(chan Message, 4),
	}
	pool.Add(s1)
	pool.Add(s2)

	tc := TestConfig
	tc.StrictBroadcast = true
	p := NewProxy(tc, NewStats(), &pool)
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
	select {
	case msg := <-s1.send:
		t.Fatalf("proxy handler s1 must not receive message: %#v", msg)
	default:
	}
	select {
	case msg := <-s2.send:
		t.Fatalf("proxy handler s2 must not receive message: %#v", msg)
	default:
	}
}

func TestProxyPingHandlerFunc(t *testing.T) {
	var pool SessionPool
	tc := TestConfig
	p := NewProxy(tc, NewStats(), &pool)
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

func TestProxySendHandlerFunc__Timeout(t *testing.T) {
	var pool SessionPool
	s1 := &TestSession{
		key:  "hogehoge",
		send: make(chan Message),
	}

	pool.Add(s1)

	tc := TestConfig
	p := NewProxy(tc, NewStats(), &pool)
	ts := httptest.NewServer(http.HandlerFunc(p.SendHandlerFunc))
	defer ts.Close()

	req, err := http.NewRequest("POST", ts.URL, bytes.NewBufferString("test message"))
	if err != nil {
		t.Fatal("proxy handler new request unexpected error:", err)
	}
	req.Header.Add(tc.SessionHeader, "hogehoge")

	client := new(http.Client)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal("proxy handler request unexpected error:", err)
	}
	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)
	result := struct {
		Result string        `json:"result"`
		Errors []interface{} `json:"errors"`
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
}
