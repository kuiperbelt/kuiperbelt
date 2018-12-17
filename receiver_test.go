package kuiperbelt

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

func TestDiscardReceiver(t *testing.T) {
	receiver := newDiscardReceiver()

	m := strings.NewReader("hello upstream callback")
	msg := newReceivedMessage(
		websocket.TextMessage,
		http.Header{
			"X-Kuiperbelt-Session": {"session_uuid"},
		},
		m,
	)
	ctx := context.Background()
	err := receiver.Receive(ctx, msg)
	if err != nil {
		t.Errorf("unexpected error from Receive(): %s", err)
	}
}

func TestCallbackReceiver__Success(t *testing.T) {
	okBuf := &bytes.Buffer{}
	okServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err := io.Copy(okBuf, r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				t.Errorf("unexpected error at okHandler: %s", err)
				return
			}
			if r.Header.Get("X-Kuiperbelt-Session") != "session_uuid" {
				w.WriteHeader(http.StatusUnauthorized)
				t.Error("session key is not included at okHandler")
				return

			}
			w.WriteHeader(http.StatusOK)
		}),
	)

	u, err := url.Parse(okServer.URL)
	if err != nil {
		t.Fatalf("unexpected error at Parse okServer.URL: %s", err)
	}
	receiver := newCallbackReceiver(http.DefaultClient, u, TestConfig)

	m := strings.NewReader("hello upstream callback")
	msg := newReceivedMessage(
		websocket.TextMessage,
		http.Header{
			"X-Kuiperbelt-Session": {"session_uuid"},
		},
		m,
	)
	ctx := context.Background()
	err = receiver.Receive(ctx, msg)
	if err != nil {
		t.Errorf("unexpected error from Receive(): %s", err)
	}

	if okBuf.String() != "hello upstream callback" {
		t.Errorf("calling back message is not match: %s", okBuf.String())
	}
}

func TestCallbackReceiver__Unauthorized(t *testing.T) {
	ngBuf := &bytes.Buffer{}
	ngServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err := io.Copy(ngBuf, r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				t.Errorf("unexpected error at ngHandler: %s", err)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
		}),
	)

	u, err := url.Parse(ngServer.URL)
	if err != nil {
		t.Fatalf("unexpected error at Parse ngServer.URL: %s", err)
	}
	receiver := newCallbackReceiver(http.DefaultClient, u, TestConfig)

	m := strings.NewReader("hello upstream callback")
	msg := newReceivedMessage(
		websocket.TextMessage,
		http.Header{
			"X-Kuiperbelt-Session": {"session_uuid"},
		},
		m,
	)
	ctx := context.Background()
	err = receiver.Receive(ctx, msg)
	if err == nil {
		t.Errorf("Receive() success")
	}
	cerr, ok := errors.Cause(err).(errCallbackResponseNotOK)
	if !ok {
		t.Errorf("callback error is not errCallbackResponseNotOk: %+v", err)
	}
	if cerr != http.StatusUnauthorized {
		t.Errorf("callback error is not StatusUnauthorized: %+v", cerr)
	}

	if ngBuf.String() != "hello upstream callback" {
		t.Errorf("calling back message is not match: %s", ngBuf.String())
	}
}
