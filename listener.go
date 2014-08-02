package kuiperbelt

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"net/http"
)

const okResponse = "{\"result\":\"ok\"}"

type Listener struct {
	Mux *http.ServeMux
}

func NewListener() *Listener {
	mux := http.NewServeMux()

	mux.HandleFunc("/connect", ConnectorHandler)
	mux.HandleFunc("/broadcast", BroadcastHandler)
	mux.HandleFunc("/send", SendHandler)

	return &Listener{
		Mux: mux,
	}
}

func (l *Listener) ListenAndServe(addr string) error {
	server := &http.Server{
		Addr:    addr,
		Handler: l.Mux,
	}

	server.SetKeepAlivesEnabled(true)
	return server.ListenAndServe()
}

func SendHandler(w http.ResponseWriter, req *http.Request) {
	var buf bytes.Buffer
	buf.ReadFrom(req.Body)
	sendChan <- buf.Bytes()
	io.WriteString(w, okResponse)
}

func BroadcastHandler(w http.ResponseWriter, req *http.Request) {
	var buf bytes.Buffer
	buf.ReadFrom(req.Body)
	broadcastNotifier.ReceiveMessage(buf.Bytes())
	io.WriteString(w, okResponse)
}

func ConnectorHandler(w http.ResponseWriter, req *http.Request) {
	closeConnectChan, err := newConnection(w)
	if err != nil {
		log.Println(err)
		return
	}
	<-closeConnectChan
}

func newConnection(w http.ResponseWriter) (chan int, error) {
	uuid, _ := newUUID()
	closeConnectChan := connector.NewConnection(w, uuid)
	return closeConnectChan, nil
}

// newUUID generates a random UUID according to RFC 4122
// http://play.golang.org/p/4FkNSiUDMg
func newUUID() (string, error) {
	uuid := make([]byte, 16)
	n, err := io.ReadFull(rand.Reader, uuid)
	if n != len(uuid) || err != nil {
		return "", err
	}
	// variant bits; see section 4.1.1
	uuid[8] = uuid[8]&^0xc0 | 0x80
	// version 4 (pseudo-random); see section 4.1.3
	uuid[6] = uuid[6]&^0xf0 | 0x40
	return fmt.Sprintf("%x-%x-%x-%x-%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:]), nil
}
