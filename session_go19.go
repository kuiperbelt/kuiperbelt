// +build go1.9

package kuiperbelt

import (
	"errors"
	"sync"
)

var errSessionNotFound = errors.New("kuiperbelt: session is not found")

// SessionPool is a pool of sessions.
type SessionPool struct {
	m sync.Map
}

// Message is a message container for communicating through sessions.
type Message struct {
	Body        []byte
	ContentType string
	Session     string
	LastWord    bool
}

// Session is an interface for sessions.
type Session interface {
	Send() chan<- Message
	Key() string
	Close() error
	Closed() <-chan struct{}
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

// List returns a slice of all sessions in the pool.
func (p *SessionPool) List() []Session {
	sessions := make([]Session)
	p.m.Range(func(key, value interface{}{
		sessions = append(sessions, value.(Session))
	})

	return sessions
}
