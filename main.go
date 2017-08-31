package kuiperbelt

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
)

var (
	Version string
	Log     *zap.Logger
)

func init() {
	Log, _ = zap.NewDevelopment()
}

func Run(port, sock, configFilename string) {
	if port != "" && sock != "" {
		Log.Fatal("port and sock option is duplicate.")
	}

	c, err := NewConfig(configFilename)
	if err != nil {
		Log.Fatal("load config error", zap.Error(err))
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
			Log.Fatal("listen sock error",
				zap.Error(err),
				zap.String("sock", c.Sock),
			)
		}
		Log.Info("listen start",
			zap.String("sock", c.Sock),
		)
	} else {
		ln, err = net.Listen("tcp", ":"+c.Port)
		if err != nil {
			Log.Fatal("listen port error",
				zap.Error(err),
				zap.String("port", c.Port),
			)
		}
		Log.Info("listen start",
			zap.String("port", c.Port),
		)
	}

	server := &http.Server{}
	go func() {
		err := server.Serve(ln)
		if err == http.ErrServerClosed {
			return
		}
		if err != nil {
			Log.Fatal("http serve error:", zap.Error(err))
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
			Log.Info("received SIGTERM. shutting down...")
			return
		case syscall.SIGINT:
			Log.Info("received SIGINT. shutting down...")
			return
		}
	}
}
