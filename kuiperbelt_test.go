package kuiperbelt

import (
	"bytes"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestRun(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Error("can't listen anonymouse port")
	}
	addr := l.Addr().String()
	log.Printf(addr)
	port := strings.Split(addr, ":")[1]

	go Run(port, "comet")

	for i := 0; i < 10; i++ {
		isRan := func() bool {
			resp, err := http.Get("http://" + addr)
			if err == nil {
				defer resp.Body.Close()
				return true
			}
			return false
		}()

		if isRan {
			break
		}
		time.Sleep(1 * time.Second)
	}

	reqBody := bytes.NewReader([]byte(`{"UUID":"hogehoge"}`))
	resp, err := http.Post("http://"+addr+"/send", "application/json", reqBody)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Error("status code is invalid: %d", resp.StatusCode)
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Error("response body read error: %s", err)
		}

		if string(body[:]) != `{"result":"ok"}` {
			t.Error("invalid regexp for HelloMessage")
		}
	} else {
		t.Error("connect post has error: %s", err)
	}
}
