package kuiperbelt

import (
	"io"
	"net/http"
)

type Proxy struct {
	Config Config
}

func (p *Proxy) Register() {
	http.HandleFunc("/send", p.HandlerFunc)
}

func (p *Proxy) HandlerFunc(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		io.WriteString(w, "Required POST method.")
		return
	}
	key := r.Header.Get(p.Config.SessionHeader)
	s, err := GetSession(key)
	if err == sessionNotFoundError {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, "Session is not found.")
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, err.Error())
		return
	}

	_, err = io.Copy(s, r.Body)
	defer r.Body.Close()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "OK")
}
