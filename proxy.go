package kuiperbelt

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/mackee/kuiperbelt/plugin"

	"github.com/dullgiulio/pingo"
	log "gopkg.in/Sirupsen/logrus.v0"
)

var (
	preHookError           = errors.New("invalid request.")
	cannotSendMessageError = errors.New("cannot send messages.")
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
	Plugin *pingo.Plugin
	Stats  *Stats
}

func NewProxy(c Config, s *Stats, p *pingo.Plugin) *Proxy {
	return &Proxy{
		Config: c,
		Stats:  s,
		Plugin: p,
	}
}

func (p *Proxy) Register() {
	mux := http.NewServeMux()
	mux.HandleFunc("/send", p.SendHandlerFunc)
	mux.HandleFunc("/close", p.CloseHandlerFunc)
	mux.HandleFunc("/ping", p.PingHandlerFunc)
	l := NewLoggingHandler(mux)
	http.Handle("/", l)

	if p.Plugin != nil {
		go p.pollMessageFromPlugin()
	}
}

func (p *Proxy) pollMessageFromPlugin() {
	for {
		message := plugin.ReceivedMessage{}
		err := p.Plugin.Call("Plugin.ReceiveMessage", plugin.RelayArgs{}, &message)
		if err != nil {
			log.WithError(err).Fatalln("plugin receive message error")
		}
		log.WithFields(log.Fields{
			"message": message,
		}).Infoln("plugin received message")
		for _, key := range message.Keys {
			session, err := GetSession(key)
			if err != nil {
				log.WithError(err).Warnln("get session from plugin error")
			}
			b := bytes.NewBuffer(message.Message)
			sendingMessage := &Message{
				buf: b,
			}
			go p.sendMessage(session, sendingMessage)
		}
	}
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
	se := make(cannotFindSessionKeysError, 0, len(keys))
	for _, key := range keys {
		s, err := GetSession(key)
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

func (p *Proxy) tryRelayMessages(b []byte, se cannotFindSessionKeysError) error {
	var keys []string
	for _, s := range se {
		keys = append(keys, s.Session)
	}
	args := plugin.RelayArgs{Keys: keys, Message: b}
	resp := plugin.RelayResp{}
	p.Plugin.Call("Plugin.Relay", args, &resp)
	if len(resp.NotExistsKeys) > 0 {
		se = cannotFindSessionKeysError{}
		errKeys := resp.NotExistsKeys
		for _, key := range errKeys {
			se = append(
				se,
				sessionError{
					Error:   sessionNotFoundError.Error(),
					Session: key,
				},
			)
		}
		return se
	}
	return nil
}

func (p *Proxy) SendHandlerFunc(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	b := new(bytes.Buffer)
	_, err := io.Copy(b, r.Body)
	if err != nil {
		log.WithFields(log.Fields{
			"session": r.Body,
			"error":   err.Error(),
		}).Error("copy from body error")
		return
	}

	message := &Message{
		buf:         b,
		contentType: r.Header.Get("Content-Type"),
	}
	ss, err := p.handlerPreHook(w, r)
	if se, ok := err.(cannotFindSessionKeysError); ok {
		if p.Plugin != nil {
			err := p.tryRelayMessages(b.Bytes(), se)
			if tryError, ok := err.(cannotFindSessionKeysError); ok {
				se = tryError
			} else if err != nil {
				log.WithError(err).Error("try relay message error")
				return
			}
		}
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
	_, err = io.Copy(b, r.Body)
	if err != nil {
	}
	message := &Message{
		buf:         b,
		contentType: r.Header.Get("Content-Type"),
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

func (p *Proxy) sendMessage(s Session, message *Message) error {
	p.Stats.MessageEvent()
	var err error
	if wss, ok := s.(*WebSocketSession); ok {
		err = wss.SendMessage(message)
	} else {
		var nw int
		nw, err = s.Write(message.buf.Bytes())
		if nw != message.buf.Len() {
			p.Stats.MessageErrorEvent()
			log.WithFields(log.Fields{
				"session":      s.Key(),
				"write_bytes":  message.buf.Len(),
				"return_bytes": nw,
			}).Error("write to session is short")
			return cannotSendMessageError
		}
	}
	if err != nil {
		p.Stats.MessageErrorEvent()
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

	return nil
}

func (p *Proxy) closeSession(s Session, message *Message) {
	err := p.sendMessage(s, message)
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
