package kuiperbelt

import (
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

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

	p := NewProxy(*c, st)
	p.Register()

	s := NewWebSocketServer(*c, st)
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

	startSignalHandler(ln)

	err = http.Serve(ln, nil)
	if err != nil {
		Log.Fatal("http serve error:", zap.Error(err))
	}
}

func startSignalHandler(ln net.Listener) {
	signalCh := make(chan os.Signal)
	signal.Notify(signalCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		for {
			s := <-signalCh
			if s == syscall.SIGTERM || s == syscall.SIGINT {
				Log.Info("received SIGTERM. shutting down...")
				ln.Close()
			}
		}
	}()
}
