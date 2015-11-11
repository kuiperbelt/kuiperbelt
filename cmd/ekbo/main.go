package main

import (
	"flag"
	log "gopkg.in/Sirupsen/logrus.v0"

	"github.com/mackee/kuiperbelt"
)

func main() {
	var configFilename, logLevel, port, sock string
	flag.StringVar(&configFilename, "config", "config.yml", "config path")
	flag.StringVar(&logLevel, "log-level", "", "log level")
	flag.StringVar(&port, "port", "", "launch port")
	flag.StringVar(&sock, "sock", "", "unix domain socket path")
	flag.Parse()

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
