package kuiperbelt

import (
	"io/ioutil"
	"net"
	"os"
	"strconv"

	"gopkg.in/yaml.v1"
)

type Config struct {
	Callback        Callback `yaml:"callback"`
	SessionHeader   string   `yaml:"session_header"`
	Port            string   `yaml:"port"`
	Sock            string   `yaml:"sock"`
	Endpoint        string   `yaml:"endpoint"`
	StrictBroadcast bool     `yaml:"strict_broadcast"`
}

type Callback struct {
	Connect string `yaml:"connect"`
	Close   string `yaml:"close"`
}

func NewConfig(filename string) (*Config, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return unmarshalConfig(b)
}

func unmarshalConfig(b []byte) (*Config, error) {
	var config Config
	err := yaml.Unmarshal(b, &config)
	if err != nil {
		return nil, err
	}
	if config.SessionHeader == "" {
		config.SessionHeader = "X-Kuiperbelt-Session"
	}
	if config.Endpoint == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, err
		}

		p, err := strconv.Atoi(config.Port)
		if err != nil {
			return nil, err
		}

		if p <= 1023 {
			config.Endpoint = hostname
		} else {
			config.Endpoint = net.JoinHostPort(hostname, config.Port)
		}
	}

	return &config, nil
}
