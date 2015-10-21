package kuiperbelt

import (
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v1"
)

type Config struct {
	Callback      Callback `yaml:"callback"`
	SessionHeader string   `yaml:"session_header"`
	Port          string   `yaml:"port"`
}

type Callback struct {
	Connect string `yaml:"connect"`
	Receive string `yaml:"receive"`
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
	return &config, nil
}
