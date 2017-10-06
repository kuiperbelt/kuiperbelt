package kuiperbelt

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v1"
)

const (
	DefaultPort         = "9180"
	DefaultOriginPolicy = "none"
)

var (
	validOriginPolicies = []string{
		"same_origin",
		"same_hostname",
		"none",
	}
)

type Config struct {
	Callback        Callback          `yaml:"callback"`
	SessionHeader   string            `yaml:"session_header"`
	Port            string            `yaml:"port"`
	Sock            string            `yaml:"sock"`
	Endpoint        string            `yaml:"endpoint"`
	StrictBroadcast bool              `yaml:"strict_broadcast"`
	ProxySetHeader  map[string]string `yaml:"proxy_set_header"`
	SendTimeout     time.Duration     `yaml:"send_timeout"`
	SendQueueSize   int               `yaml:"send_queue_size"`
	OriginPolicy    string            `yaml:"origin_policy"`
	IdleTimeout     time.Duration     `yaml:"idle_timeout"`
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

		if config.Port == "" {
			config.Port = DefaultPort
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
	if config.OriginPolicy == "" {
		config.OriginPolicy = DefaultOriginPolicy
	}
	isValidOriginPolicy := false
	for _, valid := range validOriginPolicies {
		if config.OriginPolicy == valid {
			isValidOriginPolicy = true
			break
		}
	}
	if !isValidOriginPolicy {
		return nil, fmt.Errorf("origin_policy is invalid. availables: [%s] got: %s",
			strings.Join(validOriginPolicies, ", "),
			config.OriginPolicy,
		)
	}

	return &config, nil
}
