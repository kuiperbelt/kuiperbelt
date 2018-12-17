package kuiperbelt

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kayac/go-config"
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
	Callback          Callback          `yaml:"callback"`
	SessionHeader     string            `yaml:"session_header"`
	Port              string            `yaml:"port"`
	Sock              string            `yaml:"sock"`
	Endpoint          string            `yaml:"endpoint"`
	StrictBroadcast   bool              `yaml:"strict_broadcast"`
	ProxySetHeader    map[string]string `yaml:"proxy_set_header"`
	SendTimeout       time.Duration     `yaml:"send_timeout"`
	SendQueueSize     int               `yaml:"send_queue_size"`
	OriginPolicy      string            `yaml:"origin_policy"`
	IdleTimeout       time.Duration     `yaml:"idle_timeout"`
	SuppressAccessLog bool              `yaml:"suppress_access_log"`
	Path              Path              `yaml:"path"`
}

type Callback struct {
	Connect string        `yaml:"connect"`
	Close   string        `yaml:"close"`
	Timeout time.Duration `yaml:"timeout"`
	Receive string        `yaml:"receive"`
}

type Path struct {
	Connect string `yaml:"connect"`
	Close   string `yaml:"close"`
	Stats   string `yaml:"stats"`
	Send    string `yaml:"send"`
	Ping    string `yaml:"ping"`
}

func NewConfig(filename string) (*Config, error) {
	var c Config
	err := config.LoadWithEnv(&c, filename)
	if err != nil {
		return nil, err
	}
	return tryBindDefaultToConfig(&c)
}

func tryBindDefaultToConfig(c *Config) (*Config, error) {
	if c.SessionHeader == "" {
		c.SessionHeader = "X-Kuiperbelt-Session"
	}
	if c.Endpoint == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, err
		}

		if c.Port == "" {
			c.Port = DefaultPort
		}
		p, err := strconv.Atoi(c.Port)
		if err != nil {
			return nil, err
		}

		if p <= 1023 {
			c.Endpoint = hostname
		} else {
			c.Endpoint = net.JoinHostPort(hostname, c.Port)
		}
	}
	if c.OriginPolicy == "" {
		c.OriginPolicy = DefaultOriginPolicy
	}
	isValidOriginPolicy := false
	for _, valid := range validOriginPolicies {
		if c.OriginPolicy == valid {
			isValidOriginPolicy = true
			break
		}
	}
	if !isValidOriginPolicy {
		return nil, fmt.Errorf("origin_policy is invalid. availables: [%s] got: %s",
			strings.Join(validOriginPolicies, ", "),
			c.OriginPolicy,
		)
	}

	if c.Path.Connect == "" {
		c.Path.Connect = "/connect"
	}
	if c.Path.Close == "" {
		c.Path.Close = "/close"
	}
	if c.Path.Stats == "" {
		c.Path.Stats = "/stats"
	}
	if c.Path.Send == "" {
		c.Path.Send = "/send"
	}
	if c.Path.Ping == "" {
		c.Path.Ping = "/ping"
	}

	return c, nil
}
