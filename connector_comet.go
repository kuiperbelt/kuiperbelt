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
	closeChan := make(chan int)
	go conn.loop(w, uuid, closeChan)

	return closeChan
}

func (conn *CometConnector) loop(w http.ResponseWriter, uuid string, closeChan chan int) {
	notifyChan := w.(http.CloseNotifier).CloseNotify()
	connChan := make(chan []byte)
	conn.chans[uuid] = connChan
Loop:
	for {
		select {
		case message, ok := <-connChan:
			if !ok {
				break Loop
			}
			writeMessage(w, message)
		case <-notifyChan:
			conn.removeChan(uuid)
			break Loop
		}
	}
	closeChan <- 1
}

func (conn *CometConnector) removeChan(uuid string) {
	close(conn.chans[uuid])
	delete(conn.chans, uuid)
}

func writeMessage(w http.ResponseWriter, message []byte) {
	w.Write(append(message, []byte("\n")...))
	w.(http.Flusher).Flush()
}
