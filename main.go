package kuiperbelt

import (
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	log "gopkg.in/Sirupsen/logrus.v0"
)

var Version string

func Run(port, sock, configFilename string) {
	if port != "" && sock != "" {
		log.Fatal("port and sock option is duplicate.")
	}

	c, err := NewConfig(configFilename)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err.Error(),
		}).Fatal("load config error")
	}
	if sock != "" {
		c.Sock = sock
	} else if port != "" {
		c.Port = port
	}

	st := NewStats()

	p := NewProxy(*c, st)
	p.Register()

	s := NewWebSocketServer(*c, st)
	s.Register()

	var ln net.Listener
	if c.Sock != "" {
		ln, err = net.Listen("unix", c.Sock)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err.Error(),
				"sock":  c.Sock,
			}).Fatal("listen sock error")
		}
		log.WithFields(log.Fields{
			"sock": c.Sock,
		}).Info("listen start")
	} else {
		ln, err = net.Listen("tcp", ":"+c.Port)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err.Error(),
				"port":  c.Port,
			}).Fatal("listen port error")
		}
		log.WithFields(log.Fields{
			"port": c.Port,
		}).Info("listen start")

	}

	startSignalHandler(ln)

	err = http.Serve(ln, nil)
	if err != nil {
		log.Fatal("http serve error:", err)
	}
}

func startSignalHandler(ln net.Listener) {
	signalCh := make(chan os.Signal)
	signal.Notify(signalCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		for {
			s := <-signalCh
			if s == syscall.SIGTERM || s == syscall.SIGINT {
				log.Infof("received SIGTERM. shutting down...")
				ln.Close()
			}
		}
	}()
}
