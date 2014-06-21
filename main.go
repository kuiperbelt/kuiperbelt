package main

import (
	"log"
	"net/http"
)

var port = "8080"

func main() {
	InitConnector()
	InitListener()
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatalf("Can't start server. Check please port: %s", port)
	}
}
