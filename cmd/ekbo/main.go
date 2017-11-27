package main

import (
	"flag"
	"fmt"
	"log"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/google/gops/agent"
	"github.com/mackee/kuiperbelt"
)

func main() {
	var configFilename, logLevel, port, sock, gopsAddr string
	var showVersion bool
	var err error
	flag.StringVar(&configFilename, "config", "config.yml", "config path")
	flag.StringVar(&logLevel, "log-level", "info", "log level")
	flag.StringVar(&port, "port", "", "launch port")
	flag.StringVar(&gopsAddr, "gops-addr", "localhost:9181", "gops address")
	flag.StringVar(&sock, "sock", "", "unix domain socket path")
	flag.BoolVar(&showVersion, "version", false, "show version")
	flag.Parse()

	if showVersion {
		fmt.Printf("ekbo version: %s\n", kuiperbelt.Version)
		return
	}

	if err := agent.Listen(&agent.Options{Addr: gopsAddr}); err != nil {
		log.Fatal(err)
	}

	conf := zap.NewDevelopmentConfig()
	conf.DisableStacktrace = true
	switch logLevel {
	case "debug":
		conf.DisableStacktrace = false
		conf.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	case "info":
		conf.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	case "warn":
		conf.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
	case "error":
		conf.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	}

	kuiperbelt.Log, err = conf.Build()
	if err != nil {
		panic(err)
	}

	kuiperbelt.Run(port, sock, configFilename)
}

func buildLevel(level string) zapcore.Level {
	return zapcore.InfoLevel
}
