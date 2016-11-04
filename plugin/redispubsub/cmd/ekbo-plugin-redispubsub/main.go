package main

import (
	"github.com/mackee/kuiperbelt/plugin/redispubsub"

	"github.com/dullgiulio/pingo"
	log "gopkg.in/Sirupsen/logrus.v0"
)

func main() {
	p, err := redispubsub.NewPlugin()
	if err != nil {
		log.Fatalf("redispubsub new plugin error: %s", err)
	}
	pingo.Register(p)
	pingo.Run()
}
