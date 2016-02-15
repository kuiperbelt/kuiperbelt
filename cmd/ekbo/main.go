package main

import (
	"flag"
	"fmt"

	"github.com/dullgiulio/pingo"
	log "gopkg.in/Sirupsen/logrus.v0"

	"github.com/mackee/kuiperbelt"
)

func main() {
	var configFilename, logLevel, port, sock, plugin string
	flag.StringVar(&configFilename, "config", "config.yml", "config path")
	flag.StringVar(&logLevel, "log-level", "", "log level")
	flag.StringVar(&port, "port", "", "launch port")
	flag.StringVar(&sock, "sock", "", "unix domain socket path")
	flag.StringVar(&plugin, "plugin", "", "plugin name")
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
	var p *pingo.Plugin
	if plugin != "" {
		pluginName := fmt.Sprintf("ekbo-plugin-%s", plugin)
		p = pingo.NewPlugin("tcp", pluginName)
	}

	kuiperbelt.Run(port, sock, configFilename, p)
}
