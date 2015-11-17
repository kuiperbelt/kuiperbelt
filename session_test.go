package kuiperbelt

import (
	"bytes"
)

type TestSession struct {
	*bytes.Buffer
	key             string
	isClosed        bool
	isNotifiedClose bool
}

func (s *TestSession) Key() string {
	return s.key
}

func (s *TestSession) Close() error {
	s.isClosed = true
	return nil
}

func (s *TestSession) NotifiedClose(isNotified bool) {
	s.isNotifiedClose = isNotified
}
