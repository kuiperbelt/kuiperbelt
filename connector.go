package kuiperbelt

import (
	"encoding/json"
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
	connector         Connector
	sendChan          = make(chan []byte)
	broadcastNotifier *BroadcastNotifier
)

func InitConnector(connectorName string) {
	switch connectorName {
	case "comet":
		connector = NewCometConnector()
	default:
		log.Fatalf("not found connector: %s", connectorName)
	}
	broadcastNotifier = NewBroadcastNotifier(connector)

	go sendLoop()
	http.HandleFunc("/connect", ConnectorHandler)
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
