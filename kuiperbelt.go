package kuiperbelt

import (
	"log"
	"net/http"
)

func Run(port string, connectorName string) {
	InitConnector(connectorName)
	InitListener()
	log.Printf("Kuiperbelt start listen and serve on %s\n", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatalf("Can't start server. Check please port: %s", port)
	}
}
