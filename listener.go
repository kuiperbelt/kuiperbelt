package main

import (
	"bytes"
	"io"
	"net/http"
)

var okResponse = "{ \"result\": \"ok\"}"

func InitListener() {
	http.HandleFunc("/broadcast", BroadcastHandler)
	http.HandleFunc("/send", SendHandler)
	http.HandleFunc("/eof", BroadcastHandler)
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

func EofHandler(w http.ResponseWriter, req *http.Request) {
	eofChan <- 1
	io.WriteString(w, okResponse)
}
