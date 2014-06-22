package kuiperbelt

import (
	"bytes"
	"io"
	"net/http"
)

const okResponse = "{ \"result\": \"ok\"}"

func InitListener() {
	http.HandleFunc("/broadcast", BroadcastHandler)
	http.HandleFunc("/send", SendHandler)
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
	broadcastChan <- buf.Bytes()
	io.WriteString(w, okResponse)
}
