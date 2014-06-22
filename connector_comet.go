package kuiperbelt

import (
	"encoding/json"
	"net/http"
)

type CometConnector struct {
	Connector
	chans map[string]chan []byte
}

func NewCometConnector() Connector {
	return &CometConnector{chans: make(map[string]chan []byte)}
}

func (conn *CometConnector) GetChans() map[string]chan []byte {
	return conn.chans
}

func (conn *CometConnector) GetChan(uuid string) (chan []byte, bool) {
	connectorChan, ok := conn.chans[uuid]
	return connectorChan, ok
}

func (conn *CometConnector) NewConnection(w http.ResponseWriter, uuid string) chan int {
	helloMessage := HelloMessage{
		UUID: uuid,
	}
	helloBytes, _ := json.Marshal(helloMessage)

	writeMessage(w, helloBytes)
	connectorChan := make(chan []byte)
	closeConnectChan := make(chan int)
	conn.chans[uuid] = connectorChan
	go connectionLoop(w, connectorChan, closeConnectChan)

	return closeConnectChan
}

func connectionLoop(w http.ResponseWriter, connectorChan chan []byte, closeConnectChan chan int) {
	for {
		message, ok := <-connectorChan
		if !ok {
			break
		}
		writeMessage(w, message)
	}
	closeConnectChan <- 1
}

func writeMessage(w http.ResponseWriter, message []byte) {
	w.Write(append(message, []byte("\n")...))
	w.(http.Flusher).Flush()
}
