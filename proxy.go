package kuiperbelt

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	log "gopkg.in/Sirupsen/logrus.v0"
)

var (
	preHookError           = errors.New("invalid request.")
	cannotSendMessageError = errors.New("cannot send messages.")
)

const (
	ioBufferSize = 4096
)

type cannotFindSessionKeysError []sessionError

func (e cannotFindSessionKeysError) Error() string {
	keys := make([]string, 0, len(e))
	for _, s := range e {
		keys = append(keys, s.Session)
	}

	return fmt.Sprintf("cannot find session keys error: %v", keys)
}

type Proxy struct {
	Config Config
	Stats  *Stats
	Pool   *SessionPool
}

func NewProxy(c Config, s *Stats, p *SessionPool) *Proxy {
	return &Proxy{
		Config: c,
		Stats:  s,
		Pool:   p,
	}
}

func (p *Proxy) Register() {
	mux := http.NewServeMux()
	mux.HandleFunc("/send", p.SendHandlerFunc)
	mux.HandleFunc("/close", p.CloseHandlerFunc)
	mux.HandleFunc("/ping", p.PingHandlerFunc)
	l := NewLoggingHandler(mux)
	http.Handle("/", l)
}

func (p *Proxy) handlerPreHook(w http.ResponseWriter, r *http.Request) ([]Session, error) {
	if r.Method != "POST" {
		http.Error(w, "Required POST method.", http.StatusMethodNotAllowed)
		return nil, preHookError
	}
	keys, ok := r.Header[p.Config.SessionHeader]
	if !ok || len(keys) == 0 {
		http.Error(w, "Session is not found.", http.StatusBadRequest)
		return nil, preHookError
	}
	ss := make([]Session, 0, len(keys))
	se := make(cannotFindSessionKeysError, 0, len(keys))
	for _, key := range keys {
		s, err := p.Pool.Get(key)
		if err != nil {
			se = append(se, sessionError{err.Error(), key})
			continue
		}
		ss = append(ss, s)
	}
	if len(se) > 0 {
		return ss, se
	}

	return ss, nil
}

func (p *Proxy) sessionKeysErrorHandler(w http.ResponseWriter, se cannotFindSessionKeysError, ss []Session) {
	res := struct {
		Errors []sessionError `json:"errors"`
		Result string         `json:"result"`
	}{
		Errors: se,
	}

	if p.Config.StrictBroadcast && len(se) > 0 {
		w.WriteHeader(http.StatusBadRequest)
		res.Result = "NG"
	} else {
		w.WriteHeader(http.StatusOK)
		res.Result = "OK"
	}

	w.Header().Add("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.Encode(res)
}

func (p *Proxy) SendHandlerFunc(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	ss, err := p.handlerPreHook(w, r)
	if se, ok := err.(cannotFindSessionKeysError); ok {
		p.sessionKeysErrorHandler(w, se, ss)
		if p.Config.StrictBroadcast {
			return
		}
	} else if err != nil {
		return
	} else {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"result":"OK"}`)
	}

	b := new(bytes.Buffer)
	buf := make([]byte, ioBufferSize)
	_, err = io.CopyBuffer(b, r.Body, buf)
	if err != nil {
	}
	message := Message{
		Body:        b.Bytes(),
		ContentType: r.Header.Get("Content-Type"),
	}

	for _, s := range ss {
		go p.sendMessage(s, message)
	}
}

func (p *Proxy) CloseHandlerFunc(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	ss, err := p.handlerPreHook(w, r)
	if se, ok := err.(cannotFindSessionKeysError); ok {
		p.sessionKeysErrorHandler(w, se, ss)
		if p.Config.StrictBroadcast {
			return
		}
	} else if err != nil {
		return
	} else {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"result":"OK"}`)
	}

	b := new(bytes.Buffer)
	buf := make([]byte, ioBufferSize)
	_, err = io.CopyBuffer(b, r.Body, buf)
	if err != nil {
	}
	message := Message{
		Body:        b.Bytes(),
		ContentType: r.Header.Get("Content-Type"),
	}

	for _, s := range ss {
		go p.closeSession(s, message)
	}
}

func (p *Proxy) PingHandlerFunc(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, `{"result":"OK"}`)
}

type sessionError struct {
	Error   string `json:"error"`
	Session string `json:"session"`
}

func (p *Proxy) sendMessage(s Session, message Message) error {
	p.Stats.MessageEvent()
	s.Send() <- message
	return nil
}

func (p *Proxy) closeSession(s Session, message Message) {
	err := p.sendMessage(s, message)
	if err != nil {
		return
	}
	err = s.Close()
	if err != nil {
		log.WithFields(log.Fields{
			"session": s.Key(),
			"error":   err.Error(),
		}).Error("cannnot close session error")
		return
	}
}
