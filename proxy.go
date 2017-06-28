package kuiperbelt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sync"
)

const (
	ioBufferSize = 4096
)

type sessionErrors []sessionError

func (e sessionErrors) Error() string {
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
		w.Header().Add("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		io.WriteString(w, `{"errors":[{"error":"required POST method"}],"result":"NG"}`)
		return nil, errors.New("kuiperbelt: method not allowed")
	}
	keys, ok := r.Header[p.Config.SessionHeader]
	if !ok || len(keys) == 0 {
		w.Header().Add("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"errors":[{"error":"session header is missing"}],"result":"NG"}`)
		return nil, errors.New("kuiperbelt: session header is missing")
	}
	ss := make([]Session, 0, len(keys))
	se := make(sessionErrors, 0, len(keys))
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

func (p *Proxy) sessionKeysErrorHandler(w http.ResponseWriter, se sessionErrors, ss []Session) {
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

	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.Encode(res)
}

// SendHandlerFunc handles POST /send request.
func (p *Proxy) SendHandlerFunc(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	ss, err := p.handlerPreHook(w, r)
	se, ok := err.(sessionErrors)
	if ok {
		if p.Config.StrictBroadcast {
			p.sessionKeysErrorHandler(w, se, ss)
			return
		}
	} else if err != nil {
		return
	}

	// XXX: meybe need limit?
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.Header().Add("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `{"result":"NG"}`)
		return
	}
	message := Message{
		Body:        buf,
		ContentType: r.Header.Get("Content-Type"),
	}

	var ctx context.Context
	var cancel context.CancelFunc
	if p.Config.SendTimeout != 0 {
		ctx, cancel = context.WithTimeout(r.Context(), p.Config.SendTimeout)
	} else {
		ctx, cancel = context.WithCancel(r.Context())
	}
	defer cancel()

	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(len(ss))
	for _, s := range ss {
		s := s
		go func() {
			defer wg.Done()
			if err := p.sendMessage(ctx, s, message); err != nil {
				mu.Lock()
				se = append(se, sessionError{
					Error:   err.Error(),
					Session: s.Key(),
				})
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if len(se) > 0 {
		p.sessionKeysErrorHandler(w, se, ss)
		return
	}

	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, `{"result":"OK"}`)
}

// CloseHandlerFunc handles POST /close request.
func (p *Proxy) CloseHandlerFunc(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	ss, err := p.handlerPreHook(w, r)
	se, ok := err.(sessionErrors)
	if ok && p.Config.StrictBroadcast {
		p.sessionKeysErrorHandler(w, se, ss)
		return
	}
	if err != nil {
		return
	}

	// XXX: meybe need limit?
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.Header().Add("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, `{"result":"NG"}`)
		return
	}
	message := Message{
		Body:        buf,
		ContentType: r.Header.Get("Content-Type"),
		LastWord:    true,
	}

	var ctx context.Context
	var cancel context.CancelFunc
	if p.Config.SendTimeout != 0 {
		ctx, cancel = context.WithTimeout(r.Context(), p.Config.SendTimeout)
	} else {
		ctx, cancel = context.WithCancel(r.Context())
	}
	defer cancel()

	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(len(ss))
	for _, s := range ss {
		s := s
		go func() {
			defer wg.Done()
			if err := p.sendMessage(ctx, s, message); err != nil {
				mu.Lock()
				se = append(se, sessionError{
					Error:   err.Error(),
					Session: s.Key(),
				})
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if len(se) > 0 {
		p.sessionKeysErrorHandler(w, se, ss)
		return
	}

	w.Header().Add("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, `{"result":"OK"}`)
}

// PingHandlerFunc handles ping request.
func (p *Proxy) PingHandlerFunc(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	io.WriteString(w, `{"result":"OK"}`)
}

type sessionError struct {
	Error   string `json:"error"`
	Session string `json:"session"`
}

func (p *Proxy) sendMessage(ctx context.Context, s Session, message Message) error {
	p.Stats.MessageEvent()
	q := s.Send()
	if q == nil {
		return errors.New("kuiperbelt: session is closed")
	}
	select {
	case q <- message:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}
