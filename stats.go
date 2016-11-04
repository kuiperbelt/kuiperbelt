package kuiperbelt

import (
	"encoding/json"
	"io"
	"sync/atomic"
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
