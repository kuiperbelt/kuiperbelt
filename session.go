package kuiperbelt

import (
	"errors"
	"io"
	"sync"
)

var (
	sessionMap         = sync.Map{}
	errSessionNotFound = errors.New("kuiperbelt: session is not found")
)

type Session interface {
	io.ReadWriteCloser
	Key() string
	NotifiedClose(bool)
}

func AddSession(s Session) {
	sessionMap.Store(s.Key(), s)
}

func GetSession(key string) (Session, error) {
	s, ok := sessionMap.Load(key)
	if !ok {
		return nil, errSessionNotFound
	}
	return s.(Session), nil
}

func DelSession(key string) error {
	sessionMap.Delete(key)
	return nil
}
