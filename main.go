package kuiperbelt

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	var pool SessionPool

	p := NewProxy(*c, st, &pool)
	p.Register()

	s := NewWebSocketServer(*c, st, &pool)
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

	server := &http.Server{}
	go func() {
		err := server.Serve(ln)
		if err == http.ErrServerClosed {
			return
		}
		if err != nil {
			log.Fatal("http serve error:", err)
		}
	}()

	waitForSignal()

	// Shutdown gracefully
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server.Shutdown(ctx)
	s.Shutdown(ctx)
}

func waitForSignal() {
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(signalCh)

	for s := range signalCh {
		switch s {
		case syscall.SIGTERM:
			log.Infof("received SIGTERM. shutting down...")
			return
		case syscall.SIGINT:
			log.Infof("received SIGINT. shutting down...")
			return
		}
	}
}
