package main

import (
	"flag"
	"fmt"

	log "gopkg.in/Sirupsen/logrus.v0"

	"github.com/google/gops/agent"
	"github.com/mackee/kuiperbelt"
)

func main() {
	if err := agent.Listen(nil); err != nil {
		log.Fatal(err)
	}
	var configFilename, logLevel, port, sock string
	var showVersion bool
	flag.StringVar(&configFilename, "config", "config.yml", "config path")
	flag.StringVar(&logLevel, "log-level", "", "log level")
	flag.StringVar(&port, "port", "", "launch port")
	flag.StringVar(&sock, "sock", "", "unix domain socket path")
	flag.BoolVar(&showVersion, "version", false, "show version")
	flag.Parse()

	if showVersion {
		fmt.Printf("ekbo version: %s\n", kuiperbelt.Version)
		return
	}

	if logLevel != "" {
		lvl, err := log.ParseLevel(logLevel)
		if err != nil {
			log.WithFields(log.Fields{
				"log_evel": logLevel,
			}).Fatal("cannot parse log level")
		}
		log.SetLevel(lvl)
	}
	kuiperbelt.Run(port, sock, configFilename)
}
