// +build go1.9

package kuiperbelt

import (
	"errors"
	"io"
	"sync"
)

var errSessionNotFound = errors.New("kuiperbelt: session is not found")

// SessionPool is a pool of sessions.
type SessionPool struct {
	m sync.Map
}

type Session interface {
	io.ReadWriteCloser
	Key() string
	NotifiedClose(bool)
}

// Add add new session into the SessionPool.
func (p *SessionPool) Add(s Session) {
	p.m.Store(s.Key(), s)
}

// Get gets a session from the SessionPool.
func (p *SessionPool) Get(key string) (Session, error) {
	s, ok := p.m.Load(key)
	if !ok {
		return nil, errSessionNotFound
	}
	return s.(Session), nil
}

// Delete deletes a session.
func (p *SessionPool) Delete(key string) error {
	p.m.Delete(key)
	return nil
}
