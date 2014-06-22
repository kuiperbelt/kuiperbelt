package kuiperbelt

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type Connector interface {
	NewConnection(w http.ResponseWriter, uuid string) chan int
	GetChan(uuid string) (chan []byte, bool)
	GetChans() map[string]chan []byte
}

type HelloMessage struct {
	UUID string
}

var (
	connector     Connector
	sendChan      = make(chan []byte)
	broadcastChan = make(chan []byte)
)

func InitConnector(connectorName string) {
	switch connectorName {
	case "comet":
		connector = NewCometConnector()
	default:
		log.Fatalf("not found connector: %s", connectorName)
	}

	go sendLoop()
	go broadcastLoop()
	http.HandleFunc("/connect", ConnectorHandler)
}

func ConnectorHandler(w http.ResponseWriter, req *http.Request) {
	closeConnectChan := newConnection(w)
	<-closeConnectChan
}

func newConnection(w http.ResponseWriter) chan int {
	uuid, _ := newUUID()
	closeConnectChan := connector.NewConnection(w, uuid)
	return closeConnectChan
}

func sendLoop() {
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
		connectorChan, existsConnector := connector.GetChan(uuid.(string))
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

func broadcastLoop() {
	for {
		message, ok := <-broadcastChan
		if !ok {
			log.Println("send destroy...")
			break
		}
		for _, connectorChan := range connector.GetChans() {
			connectorChan <- message
		}
	}
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
