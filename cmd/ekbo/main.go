package main

import (
	"flag"
	"github.com/mackee/kuiperbelt"
	"os"
)

var (
	connectorName string
	port          string
	isShowHelp    bool
)

func main() {
	flag.StringVar(&connectorName, "c", "comet", "see --connector")
	flag.StringVar(&connectorName, "connector", "comet", "connector protocol.")
	flag.StringVar(&port, "p", "8080", "see --port")
	flag.StringVar(&port, "port", "8080", "listen and Serve port.")
	flag.BoolVar(&isShowHelp, "h", false, "see --help")
	flag.BoolVar(&isShowHelp, "help", false, "show help.")
	flag.Parse()

	if isShowHelp {
		flag.PrintDefaults()
		os.Exit(0)
	}

	kuiperbelt.Run(port, connectorName)
}
