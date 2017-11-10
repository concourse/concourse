package db

import (
	"runtime/debug"
	"sync"
)

var GlobalConnectionTracker = NewConnectionTracker()

type ConnectionTracker struct {
	sessions  map[*ConnectionSession]struct{}
	sessionsL *sync.Mutex
}

func NewConnectionTracker() *ConnectionTracker {
	return &ConnectionTracker{
		sessions:  map[*ConnectionSession]struct{}{},
		sessionsL: &sync.Mutex{},
	}
}

func (tracker *ConnectionTracker) Track() *ConnectionSession {
	session := &ConnectionSession{
		tracker: tracker,
		stack:   string(debug.Stack()),
	}

	tracker.sessionsL.Lock()
	tracker.sessions[session] = struct{}{}
	tracker.sessionsL.Unlock()

	return session
}

func (tracker *ConnectionTracker) Current() []string {
	stacks := []string{}

	tracker.sessionsL.Lock()

	for session := range tracker.sessions {
		stacks = append(stacks, session.stack)
	}

	tracker.sessionsL.Unlock()

	return stacks
}

func (tracker *ConnectionTracker) remove(session *ConnectionSession) {
	tracker.sessionsL.Lock()
	delete(tracker.sessions, session)
	tracker.sessionsL.Unlock()
}

type ConnectionSession struct {
	tracker *ConnectionTracker
	stack   string
}

func (session *ConnectionSession) Release() {
	session.tracker.remove(session)
}
