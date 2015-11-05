package main

import (
	"flag"
	log "gopkg.in/Sirupsen/logrus.v0"

	"github.com/mackee/kuiperbelt"
)

func main() {
	var configFilename, logLevel string
	flag.StringVar(&configFilename, "config", "config.yml", "config path")
	flag.StringVar(&logLevel, "log-level", "", "log level")
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

	kuiperbelt.Run(configFilename)
}
