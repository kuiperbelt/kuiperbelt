package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type HelloMessage struct {
	UUID string
}

var sendChan = make(chan []byte)

var connectorChans = make(map[string]chan []byte)

var broadcastChan = make(chan []byte)
var eofChan = make(chan int)

func InitConnector() {
	go SendLoop()
	go BroadcastLoop()
	http.HandleFunc("/connect", ConnectorHandler)
}

func ConnectorHandler(w http.ResponseWriter, req *http.Request) {
	closeConnectChan := NewConnection(w)
	<-closeConnectChan
}

func NewConnection(w http.ResponseWriter) chan int {
	uuid, _ := NewUUID()
	helloMessage := HelloMessage{
		UUID: uuid,
	}
	helloBytes, _ := json.Marshal(helloMessage)

	WriteMessage(w, helloBytes)
	connectorChan := make(chan []byte)
	closeConnectChan := make(chan int)
	connectorChans[uuid] = connectorChan
	go ConnectionLoop(w, connectorChan, closeConnectChan)

	return closeConnectChan
}

func WriteMessage(w http.ResponseWriter, message []byte) {
	w.Write(append(message, []byte("\n")...))
	w.(http.Flusher).Flush()
}

func ConnectionLoop(w http.ResponseWriter, connectorChan chan []byte, closeConnectChan chan int) {
	for {
		message, ok := <-connectorChan
		if !ok {
			break
		}
		WriteMessage(w, message)
	}
	closeConnectChan <- 1
}

func SendLoop() {
	for {
		message, ok := <-sendChan
		if !ok {
			log.Println("send destroy...")
			break
		}
		var jsonMessage = make(map[string]interface{})
		json.Unmarshal(message, &jsonMessage)
		uuid, existsUuid := jsonMessage["UUID"]
		if !existsUuid {
			log.Println("no UUID message")
			continue
		}
		connectorChan, existsConnector := connectorChans[uuid.(string)]
		if !existsConnector {
			log.Printf("has not UUID: %s\n", uuid)
			continue
		}

		connectorChan <- message
		if !ok {
			log.Println("send destroy...")
			break
		}

	}
}

func BroadcastLoop() {
	for {
		message, ok := <-broadcastChan
		if !ok {
			log.Println("send destroy...")
			break
		}
		for _, connectorChan := range connectorChans {
			connectorChan <- message
		}
	}
}

// newUUID generates a random UUID according to RFC 4122
// http://play.golang.org/p/4FkNSiUDMg
func NewUUID() (string, error) {
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
