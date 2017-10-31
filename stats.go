package kuiperbelt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sync/atomic"
	"time"
)

// macopy may be embedded into structs which must not be copied
// after the first use.
// See https://github.com/golang/go/issues/8005#issuecomment-190753527
// for details.
type macopy struct{}

func (*macopy) Lock() {}

type Stats struct {
	connections        int64
	totalConnections   int64
	totalMessages      int64
	connectErrors      int64
	messageErrors      int64
	closingConnections int64
	noCopy             macopy
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

func (s *Stats) ClosingConnections() int64 {
	return atomic.LoadInt64(&s.closingConnections)
}

func (s *Stats) Dump(w io.Writer) error {
	return json.NewEncoder(w).Encode(struct {
		Connections        int64 `json:"connections"`
		TotalConnections   int64 `json:"total_connections"`
		TotalMessages      int64 `json:"total_messages"`
		ConnectErrors      int64 `json:"connect_errors"`
		MessageErrors      int64 `json:"message_errors"`
		ClosingConnections int64 `json:"closing_connections"`
	}{
		Connections:        s.Connections(),
		TotalConnections:   s.TotalConnections(),
		TotalMessages:      s.TotalMessages(),
		ConnectErrors:      s.ConnectErrors(),
		MessageErrors:      s.MessageErrors(),
		ClosingConnections: s.ClosingConnections(),
	})
}

func (s *Stats) DumpText(w io.Writer) error {
	now := time.Now().Unix()
	buf := new(bytes.Buffer)
	buf.Grow(512)
	fmt.Fprintf(buf, "kuiperbelt.conn.current\t%d\t%d\n", s.Connections(), now)
	fmt.Fprintf(buf, "kuiperbelt.conn.total\t%d\t%d\n", s.TotalConnections(), now)
	fmt.Fprintf(buf, "kuiperbelt.conn.errors\t%d\t%d\n", s.ConnectErrors(), now)
	fmt.Fprintf(buf, "kuiperbelt.conn.closing\t%d\t%d\n", s.ClosingConnections(), now)
	fmt.Fprintf(buf, "kuiperbelt.messages.total\t%d\t%d\n", s.TotalMessages(), now)
	fmt.Fprintf(buf, "kuiperbelt.messages.errors\t%d\t%d\n", s.MessageErrors(), now)
	_, err := buf.WriteTo(w)
	return err
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

func (s *Stats) ClosingEvent() {
	atomic.AddInt64(&s.closingConnections, 1)
}

func (s *Stats) ClosedEvent() {
	atomic.AddInt64(&s.closingConnections, -1)
}
