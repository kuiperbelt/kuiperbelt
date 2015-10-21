package kuiperbelt

import (
	"log"
	"net/http"
)

func Run(configFilename string) {
	c, err := NewConfig(configFilename)
	if err != nil {
		log.Fatal("load config error:", err)
	}
	p := &Proxy{*c}
	p.Register()

	s := &WebSocketServer{*c}
	s.Register()
	log.Fatal(http.ListenAndServe(":"+c.Port, nil))
}
