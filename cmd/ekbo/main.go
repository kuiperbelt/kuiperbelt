package main

import (
	"flag"

	"github.com/mackee/kuiperbelt"
)

func main() {
	var configFilename string
	flag.StringVar(&configFilename, "config", "config.yml", "config path")
	flag.Parse()

	kuiperbelt.Run(configFilename)
}
