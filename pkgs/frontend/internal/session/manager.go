package session

import "sync"

// A Manager manages sessions and keep
// track of them.
type Manager struct {
	sessions map[string]*Session
	sync.RWMutex
}

// NewManager returns a new *session.Manager.
func NewManager() *Manager {
	return &Manager{
		sessions: map[string]*Session{},
	}
}

// Acquire acquires and returns the session with the given sid.
// It returns nil if not session with that sid is found.
// In addition, if ch is not nil, it will be registered as a read hook.
func (p *Manager) Acquire(sid string, ch chan []byte) *Session {

	p.RLock()
	defer p.RUnlock()

	s := p.sessions[sid]
	if s != nil {
		s.acquire()
		if ch != nil {
			s.register(ch)
		}
	}

	return s
}

// Release sessions releases an acquired session.
// if the session is not acquired by anything, the ws connection
// will be closed, and the session deleted.
// In addition, if ch is not nil, it will be unregstered as a read hook.
func (p *Manager) Release(sid string, ch chan []byte) {

	p.Lock()
	defer p.Unlock()

	s := p.sessions[sid]
	if s == nil {
		return
	}

	if ch != nil {
		s.unregister(ch)
	}

	if closed := s.release(); closed {
		delete(p.sessions, sid)
	}
}

func (p *Manager) Register(s *Session) {
	p.Lock()
	defer p.Unlock()

	p.sessions[s.ID()] = s
}
