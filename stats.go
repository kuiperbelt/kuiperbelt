package kuiperbelt

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync/atomic"
	"time"
)

type doNotCopy struct{}

func (*doNotCopy) Lock() {}

type Stats struct {
	connections      int64
	totalConnections int64
	totalMessages    int64
	connectErrors    int64
	messageErrors    int64
	doNotCopy        doNotCopy
}

func NewStats() *Stats {
	return &Stats{}
}

func (s *Stats) Connections() int64 {
	return atomic.LoadInt64(&s.connections)
}

func (s *Stats) TotalConnections() int64 {
	return atomic.LoadInt64(&s.totalConnections)
}

func (s *Stats) TotalMessages() int64 {
	return atomic.LoadInt64(&s.totalMessages)
}

func (s *Stats) ConnectErrors() int64 {
	return atomic.LoadInt64(&s.connectErrors)
}

func (s *Stats) MessageErrors() int64 {
	return atomic.LoadInt64(&s.messageErrors)
}

func (s *Stats) Dump(w io.Writer) error {
	return json.NewEncoder(w).Encode(struct {
		Connections      int64 `json:"connections"`
		TotalConnections int64 `json:"total_connections"`
		TotalMessages    int64 `json:"total_messages"`
		ConnectErrors    int64 `json:"connect_errors"`
		MessageErrors    int64 `json:"message_errors"`
	}{
		Connections:      s.Connections(),
		TotalConnections: s.TotalConnections(),
		TotalMessages:    s.TotalMessages(),
		ConnectErrors:    s.ConnectErrors(),
		MessageErrors:    s.MessageErrors(),
	})
}

func (s *Stats) DumpText(w io.Writer) error {
	now := time.Now().Unix()
	_w := bufio.NewWriter(w)
	fmt.Fprintf(_w, "kuiperbelt.connections       %19d %19d\n", s.Connections(), now)
	fmt.Fprintf(_w, "kuiperbelt.total_connections %19d %19d\n", s.TotalConnections(), now)
	fmt.Fprintf(_w, "kuiperbelt.total_messages    %19d %19d\n", s.TotalMessages(), now)
	fmt.Fprintf(_w, "kuiperbelt.connect_errors    %19d %19d\n", s.ConnectErrors(), now)
	fmt.Fprintf(_w, "kuiperbelt.message_errors    %19d %19d\n", s.MessageErrors(), now)
	return _w.Flush()
}

func (s *Stats) ConnectEvent() {
	atomic.AddInt64(&s.totalConnections, 1)
	atomic.AddInt64(&s.connections, 1)
}

func (s *Stats) ConnectErrorEvent() {
	atomic.AddInt64(&s.connectErrors, 1)
}

func (s *Stats) DisconnectEvent() {
	atomic.AddInt64(&s.connections, -1)
}

func (s *Stats) MessageEvent() {
	atomic.AddInt64(&s.totalMessages, 1)
}

func (s *Stats) MessageErrorEvent() {
	atomic.AddInt64(&s.messageErrors, 1)
}
