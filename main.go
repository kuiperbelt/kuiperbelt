package kuiperbelt

import (
	log "gopkg.in/Sirupsen/logrus.v0"
	"net"
	"net/http"
)

func Run(configFilename string) {
	c, err := NewConfig(configFilename)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err.Error(),
		}).Fatal("load config error")
	}
	p := &Proxy{*c}
	p.Register()

	s := &WebSocketServer{*c}
	s.Register()

	ln, err := net.Listen("tcp", ":"+c.Port)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err.Error(),
		}).Fatal("listen port error")
	}
	log.WithFields(log.Fields{
		"port": c.Port,
	}).Info("listen start")
	log.Fatal(http.Serve(ln, nil))
}
