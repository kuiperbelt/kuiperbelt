package kuiperbelt

import (
	"testing"
)

type TestSession struct {
	send chan Message
	key  string
}

func (s *TestSession) Key() string {
	return s.key
}

func (s *TestSession) Send() chan<- Message {
	return s.send
}

func (s *TestSession) Close() error {
	return nil
}

func (s *TestSession) Closed() <-chan struct{} {
	return nil
}

func TestSessionPool__DeleteUnknownKey(t *testing.T) {
	sp := &SessionPool{}
	err := sp.Delete("unknown_key")
	if err != errSessionNotFound {
		t.Error("return not errSessionNotFound error when Delete unknown key.")
	}
}
