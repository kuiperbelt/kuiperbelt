package main

import (
	"bytes"
	"fmt"
	"github.com/kayac/parallel-benchmark/benchmark"
	"io/ioutil"
	"log"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"
)

var count = int32(0)

type Worker struct {
	URL    string
	client *http.Client
}

func NewWorker() *Worker {
	return &Worker{
		URL:    "http://localhost:8080/broadcast",
		client: &http.Client{},
	}
}

func (w *Worker) Setup() {
}

func (*Worker) Teardown() {
}

func (w *Worker) Process() (subcore int) {
	atomic.AddInt32(&count, 1)
	resp, err := w.client.Post(
		w.URL,
		"application/json",
		bytes.NewReader([]byte(fmt.Sprintf(`{"ping":%d}`, atomic.LoadInt32(&count)))),
	)
	if err == nil {
		_, _ = ioutil.ReadAll(resp.Body)
		if resp.StatusCode == 200 {
			return 1
		} else {
			return 0
		}
	}
	log.Printf("%s", err)
	return 0
}

func main() {
	runtime.GOMAXPROCS(16)
	workers := make([]benchmark.Worker, 8)
	for i, _ := range workers {
		workers[i] = NewWorker()
	}
	result := benchmark.Run(
		workers,
		10*time.Second,
	)
	log.Printf("%#v", result)
}
