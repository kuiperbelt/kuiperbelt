package kuiperbelt

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync/atomic"
	"time"
)

type Stats struct {
	Connections      int64 `json:"connections"`
	TotalConnections int64 `json:"total_connections"`
	TotalMessages    int64 `json:"total_messages"`
	ConnectErrors    int64 `json:"connect_errors"`
	MessageErrors    int64 `json:"message_errors"`
}

func NewStats() *Stats {
	return &Stats{}
}

func (s *Stats) Dump(w io.Writer) error {
	return json.NewEncoder(w).Encode(s)
}

func (s *Stats) DumpText(w io.Writer) error {
	now := time.Now().Unix()
	_w := bufio.NewWriter(w)
	fmt.Fprintf(_w, "kuiperbelt.connections\t%d\t%d\n", s.Connections, now)
	fmt.Fprintf(_w, "kuiperbelt.total_connections\t%d\t%d\n", s.TotalConnections, now)
	fmt.Fprintf(_w, "kuiperbelt.total_messages\t%d\t%d\n", s.TotalMessages, now)
	fmt.Fprintf(_w, "kuiperbelt.connect_errors\t%d\t%d\n", s.ConnectErrors, now)
	fmt.Fprintf(_w, "kuiperbelt.message_errors\t%d\t%d\n", s.MessageErrors, now)
	return _w.Flush()
}

func (s *Stats) ConnectEvent() {
	atomic.AddInt64(&s.TotalConnections, 1)
	atomic.AddInt64(&s.Connections, 1)
}

func (s *Stats) ConnectErrorEvent() {
	atomic.AddInt64(&s.ConnectErrors, 1)
}

func (s *Stats) DisconnectEvent() {
	atomic.AddInt64(&s.Connections, -1)
}

func (s *Stats) MessageEvent() {
	atomic.AddInt64(&s.TotalMessages, 1)
}

func (s *Stats) MessageErrorEvent() {
	atomic.AddInt64(&s.MessageErrors, 1)
}
