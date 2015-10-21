package kuiperbelt

import (
	"reflect"
	"testing"
)

var configData = []byte(`
session_header: "X-Kuiperbelt-Session-Key"
port: 12345
callback:
  connect: "http://localhost:12346/connect"
  receive: "http://localhost:12346/receive"
`)
var expectedConfig = Config{
	Port:          ":12345",
	SessionHeader: "X-Kuiperbelt-Session-Key",
	Callback: Callback{
		Connect: "http://localhost:12346/connect",
		Receive: "http://localhost:12346/receive",
	},
}

func TestConfig(t *testing.T) {
	c, err := unmarshalConfig(configData)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	if reflect.DeepEqual(expectedConfig, *c) {
		t.Errorf("unexpected config data:\n\t%+v\n\t%+v\n", expectedConfig, *c)
	}
}
