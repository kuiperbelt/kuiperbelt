package kuiperbelt

import (
	"log"
	"runtime"
)

func Run(port string, connectorName string) {
	runtime.GOMAXPROCS(runtime.NumCPU())
	InitConnector(connectorName)
	l := NewListener()
	log.Printf("Kuiperbelt start listen and serve on %s\n", port)
	err := l.ListenAndServe(":" + port)
	if err != nil {
		log.Fatalf("Can't start server. Check please port: %s", port)
	}
}
