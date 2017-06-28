package kuiperbelt

import (
	"bytes"
	"sync"
	"testing"
)

func TestStats(t *testing.T) {
	s := NewStats()
	if s.Connections() != 0 || s.TotalConnections() != 0 || s.TotalMessages() != 0 || s.ConnectErrors() != 0 || s.MessageErrors() != 0 {
		t.Fatalf("Stats invalid initialized %#v", s)
	}

	for i := 0; i < 10; i++ {
		s.ConnectEvent()
	}
	if s.Connections() != 10 || s.TotalConnections() != 10 {
		t.Errorf("invalid connetions count %#v", s)
	}
	for i := 0; i < 5; i++ {
		s.DisconnectEvent()
	}
	for i := 0; i < 3; i++ {
		s.ConnectErrorEvent()
	}
	for i := 0; i < 4; i++ {
		s.MessageEvent()
	}
	for i := 0; i < 2; i++ {
		s.MessageErrorEvent()
	}
	if s.Connections() != 5 || s.TotalConnections() != 10 || s.ConnectErrors() != 3 || s.TotalMessages() != 4 || s.MessageErrors() != 2 {
		t.Errorf("invalid connetions count %#v", s)
	}

	out := new(bytes.Buffer)
	err := s.Dump(out)
	if err != nil {
		t.Errorf("stats dump failed %s", err)
	}
	if out.String() != `{"connections":5,"total_connections":10,"total_messages":4,"connect_errors":3,"message_errors":2,"closing_connections":0}`+"\n" {
		t.Errorf("unexpected dump JSON %s", out.String())
	}

	text := new(bytes.Buffer)
	err = s.DumpText(text)
	if err != nil {
		t.Errorf("stats dump text failed %s", err)
	}
	t.Log(text.String())
}

func TestStatsRace(t *testing.T) {
	s := NewStats()
	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(st *Stats) {
			st.ConnectEvent()
			for j := 0; j < 10; j++ {
				st.MessageEvent()
			}
			st.DisconnectEvent()
			wg.Done()
		}(s)

		wg.Add(1)
		go func(st *Stats) {
			var text bytes.Buffer
			s.DumpText(&text)
			wg.Done()
		}(s)

		wg.Add(1)
		go func(st *Stats) {
			var text bytes.Buffer
			s.Dump(&text)
			wg.Done()
		}(s)
	}
	wg.Wait()
	if s.Connections() != 0 || s.TotalConnections() != 1000 || s.TotalMessages() != 10000 {
		t.Errorf("invalid stats %#v", s)
	}
}
