package session

import (
	"log/slog"
	"maps"
	"sync"
	"time"

	"go.acuvity.ai/wsc"
)

const defaultDeadlineDuration = 10 * time.Second

// A Session represents an active agent session.
//
// It contains the underlying websocket to communicate
// with the correct MCP server instance running in minibridge backend.
//
// The session must be acquired through a session manager when used
// then a channel must be registered to receive the data coming from the
// backend.
//
// When not in use, the Session must be released though the session manager
// and all hooks must be unregistered.
// When the the count is at 1 (only the initial aqcuirement), the session will
// linger for the duration of deadline, after which the websocket will be closed
// and so the MCP server running in the backend will be temrinated.
type Session struct {
	closeCh      chan error
	count        int
	countLock    sync.RWMutex
	nextDeadline time.Duration
	deadline     time.Time
	deadlineLock sync.RWMutex
	h            uint64
	hookLock     sync.RWMutex
	hooks        map[chan []byte]struct{}
	id           string
	ws           wsc.Websocket
}

// New retutns a new session backed by the given websocket.
func New(ws wsc.Websocket, credsHash uint64, sid string) *Session {
	return newSession(ws, credsHash, sid, defaultDeadlineDuration)
}

func newSession(ws wsc.Websocket, credsHash uint64, sid string, deadline time.Duration) *Session {

	s := &Session{
		ws:           ws,
		h:            credsHash,
		count:        1,
		id:           sid,
		deadline:     time.Now().Add(deadline),
		nextDeadline: deadline,
		closeCh:      make(chan error),
		hooks:        map[chan []byte]struct{}{},
	}

	slog.Debug("session created", "sid", s.id, "c", s.count)
	s.start()

	return s
}

// ID returns the session ID.
func (s *Session) ID() string {
	return s.id
}

// Write writes to the underlying websocket and
// advances the deadline
func (s *Session) Write(data []byte) {
	s.setDeadline(time.Now().Add(s.nextDeadline))
	s.ws.Write(data)
}

// Done returns a channel that will receive
// an error (or nil) when the websocket closes
// for any reason.
func (s *Session) Done() chan error {
	return s.closeCh
}

// ValidateHash validates the session hash.
func (s *Session) ValidateHash(h uint64) bool {
	return h == s.h
}

// Close closes the websocket
func (s *Session) Close() {
	s.ws.Close(1001)
}

func (s *Session) acquire() {
	s.countLock.Lock()
	defer s.countLock.Unlock()

	s.count++

	s.setDeadline(time.Now().Add(s.nextDeadline))
	slog.Debug("session acquired", "sid", s.id, "c", s.count)
}

func (s *Session) release() bool {
	s.countLock.Lock()
	defer s.countLock.Unlock()

	s.count--
	slog.Debug("session released", "sid", s.id, "c", s.count, "deleted", s.count <= 0)

	if s.count <= 0 {
		s.Close()
		return true
	}

	return false
}

func (s *Session) register(c chan []byte) {
	s.hookLock.Lock()
	defer s.hookLock.Unlock()

	s.hooks[c] = struct{}{}
}

func (s *Session) unregister(c chan []byte) {
	s.hookLock.Lock()
	defer s.hookLock.Unlock()

	delete(s.hooks, c)
}

func (s *Session) start() {

	go func() {

		ticker := time.NewTicker(time.Second)

		for {
			select {

			case data := <-s.ws.Read():

				for c := range s.getHooks() {
					select {
					case c <- data:
					default:
						slog.Error("Session sent data to inactive hook")
					}
				}

				s.setDeadline(time.Now().Add(s.nextDeadline))

			case err := <-s.ws.Done():
				s.closeCh <- err
				return

			case <-ticker.C:
				if time.Now().After(s.getDeadline()) && s.getCount() <= 1 {
					slog.Debug("session terminated: deadline exceeded", "sid", s.ID())
					s.release()
				}
			}
		}
	}()
}

func (s *Session) getDeadline() time.Time {
	s.deadlineLock.RLock()
	defer s.deadlineLock.RUnlock()
	return s.deadline
}

func (s *Session) setDeadline(deadline time.Time) {
	s.deadlineLock.Lock()
	defer s.deadlineLock.Unlock()
	s.deadline = deadline
}

func (s *Session) getCount() int {
	s.countLock.RLock()
	defer s.countLock.RUnlock()
	return s.count
}

func (s *Session) getHooks() map[chan []byte]struct{} {
	s.hookLock.RLock()
	defer s.hookLock.RUnlock()
	return maps.Clone(s.hooks)
}
