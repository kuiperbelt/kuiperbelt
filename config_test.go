package kuiperbelt

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"
	"time"
)

var configData = `
session_header: "X-Kuiperbelt-Session-Key"
port: {{ env "EKBO_PORT" "9180" }}
callback:
  connect: "http://localhost:12346/connect"
  timeout: 5s
send_timeout: 1s
send_queue_size: 1
endpoint: "localhost"
proxy_set_header:
  X-Foo: "Foo"
  X-Forwarded-For: ""
`
var TestConfig = Config{
	Port:          "12345",
	SessionHeader: "X-Kuiperbelt-Session-Key",
	Callback: Callback{
		Connect: "http://localhost:12346/connect",
		Close:   "",
		Timeout: time.Second * 5,
	},
	SendTimeout:   time.Second,
	SendQueueSize: 1,
	ProxySetHeader: map[string]string{
		"X-Foo":           "Foo",
		"X-Forwarded-For": "", // will be removed
	},
	Endpoint:     "localhost",
	OriginPolicy: DefaultOriginPolicy,
}

func TestConfig__NewConfig(t *testing.T) {
	os.Setenv("EKBO_PORT", "12345")
	cf, err := ioutil.TempFile("", "ekbo-config")
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	defer cf.Close()

	_, err = cf.WriteString(configData)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	cf.Close()

	c, err := NewConfig(cf.Name())
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	if !reflect.DeepEqual(TestConfig, *c) {
		t.Errorf("unexpected config data:\n\t%+v\n\t%+v\n", TestConfig, *c)
	}
}
