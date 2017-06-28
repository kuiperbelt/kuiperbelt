package kuiperbelt

import (
	"reflect"
	"testing"
	"time"
)

var configData = []byte(`
session_header: "X-Kuiperbelt-Session-Key"
port: 12345
callback:
  connect: "http://localhost:12346/connect"
send_timeout: 1s
send_queue_size: 1
`)
var TestConfig = Config{
	Port:          ":12345",
	SessionHeader: "X-Kuiperbelt-Session-Key",
	Callback: Callback{
		Connect: "http://localhost:12346/connect",
		Close:   "",
	},
	SendTimeout:   time.Second,
	SendQueueSize: 1,
}

func TestConfig__Unmarshal(t *testing.T) {
	c, err := unmarshalConfig(configData)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	if reflect.DeepEqual(TestConfig, *c) {
		t.Errorf("unexpected config data:\n\t%+v\n\t%+v\n", TestConfig, *c)
	}
}
