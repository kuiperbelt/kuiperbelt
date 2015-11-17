package kuiperbelt

import (
	"errors"
	"io"
	"sync"
)

var (
	sessionMap           = map[string]Session{}
	sessionMapLocker     = new(sync.Mutex)
	sessionNotFoundError = errors.New("session is not found.")
)

type Session interface {
	io.ReadWriteCloser
	Key() string
	NotifiedClose(bool)
}

func AddSession(s Session) {
	sessionMapLocker.Lock()
	defer sessionMapLocker.Unlock()
	sessionMap[s.Key()] = s
}

func GetSession(key string) (Session, error) {
	sessionMapLocker.Lock()
	defer sessionMapLocker.Unlock()
	s, ok := sessionMap[key]
	if !ok {
		return nil, sessionNotFoundError
	}
	return s, nil
}

func DelSession(key string) error {
	sessionMapLocker.Lock()
	defer sessionMapLocker.Unlock()
	if _, ok := sessionMap[key]; !ok {
		return sessionNotFoundError
	}
	delete(sessionMap, key)
	return nil
}
