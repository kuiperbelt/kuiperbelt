package kuiperbelt

import (
	"bytes"
	"encoding/json"
	"errors"
	log "gopkg.in/Sirupsen/logrus.v0"
	"io"
	"net/http"
)

var (
	preHookError           = errors.New("invalid request.")
	cannotSendMessageError = errors.New("cannot send messages.")
)

type Proxy struct {
	Config Config
}

func (p *Proxy) Register() {
	mux := http.NewServeMux()
	mux.HandleFunc("/send", p.SendHandlerFunc)
	mux.HandleFunc("/close", p.CloseHandlerFunc)
	l := NewLoggingHandler(mux)
	http.Handle("/", l)
}

func (p *Proxy) handlerPreHook(w http.ResponseWriter, r *http.Request) ([]Session, error) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		io.WriteString(w, "Required POST method.")
		return nil, preHookError
	}
	keys, ok := r.Header[p.Config.SessionHeader]
	if !ok || len(keys) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, "Session is not found.")
		return nil, preHookError
	}
	ss := make([]Session, 0, len(keys))
	se := make([]sessionError, 0, len(keys))
	for _, key := range keys {
		s, err := GetSession(key)
		if err != nil {
			se = append(se, sessionError{err.Error(), key})
		}
		ss = append(ss, s)
	}
	if len(se) > 0 {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Add("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.Encode(struct {
			Errors []sessionError `json:"errors"`
		}{
			Errors: se,
		})
		return nil, preHookError
	}

	return ss, nil
}

func (p *Proxy) SendHandlerFunc(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	ss, err := p.handlerPreHook(w, r)
	if err != nil {
		return
	}

	b := new(bytes.Buffer)
	_, err = io.Copy(b, r.Body)
	if err != nil {
	}
	bs := b.Bytes()

	for _, s := range ss {
		go p.sendMessage(s, bs)
	}

	w.WriteHeader(http.StatusOK)
	io.WriteString(w, `{"result":"OK"}`)
}

func (p *Proxy) CloseHandlerFunc(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	ss, err := p.handlerPreHook(w, r)
	if err != nil {
		return
	}

	b := new(bytes.Buffer)
	_, err = io.Copy(b, r.Body)
	if err != nil {
	}
	bs := b.Bytes()

	for _, s := range ss {
		go p.closeSession(s, bs)
	}

	w.WriteHeader(http.StatusOK)
	io.WriteString(w, `{"result":"OK"}`)
}

type sessionError struct {
	Error   string `json:"error"`
	Session string `json:"session"`
}

func (p *Proxy) sendMessage(s Session, bs []byte) error {
	nw, err := s.Write(bs)
	if err != nil {
		log.WithFields(log.Fields{
			"session": s.Key(),
			"error":   err.Error(),
		}).Error("write to session error")
		err = s.Close()
		if err != nil {
			log.WithFields(log.Fields{
				"session": s.Key(),
				"error":   err.Error(),
			}).Error("close session error")
		}
		return cannotSendMessageError
	}
	if nw != len(bs) {
		log.WithFields(log.Fields{
			"session":      s.Key(),
			"write_bytes":  len(bs),
			"return_bytes": nw,
		}).Error("write to session is short")
		return cannotSendMessageError
	}

	return nil
}

func (p *Proxy) closeSession(s Session, bs []byte) {
	err := p.sendMessage(s, bs)
	if err != nil {
		return
	}
	s.NotifiedClose(true)
	err = s.Close()
	if err != nil {
		log.WithFields(log.Fields{
			"session": s.Key(),
			"error":   err.Error(),
		}).Error("cannnot close session error")
		return
	}
}
