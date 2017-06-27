// +build !go1.9

package kuiperbelt

import (
	"errors"
	"sync"
)

var errSessionNotFound = errors.New("kuiperbelt: session is not found")

// SessionPool is a pool of sessions.
type SessionPool struct {
	mu sync.RWMutex
	m  map[string]Session
}

// Message is a message container for communicating through sessions.
type Message struct {
	Body        []byte
	ContentType string
	Session     string
}

// Session is an interface for sessions.
type Session interface {
	Send() chan<- Message
	Recv() <-chan Message
	Key() string
	Close() error
}

// Add add new session into the SessionPool.
func (p *SessionPool) Add(s Session) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.m == nil {
		p.m = make(map[string]Session)
	}
	p.m[s.Key()] = s
}

// Get gets a session from the SessionPool.
func (p *SessionPool) Get(key string) (Session, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	s, ok := p.m[key]
	if !ok {
		return nil, errSessionNotFound
	}
	return s, nil
}

// Delete deletes a session.
func (p *SessionPool) Delete(key string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.m == nil {
		return nil
	}
	delete(p.m, key)
	return nil
}
