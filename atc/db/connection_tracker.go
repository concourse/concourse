package db

import (
	"runtime/debug"
	"sync"
)

var GlobalConnectionTracker ConnectionTracker

func init()  {
	InitConnectionTracker(false)
}

type ConnectionTracker interface {
	Track() ConnectionSession
	Current() []string
}

type ConnectionSession interface {
	Release()
}

type connectionTracker struct {
	sessions  map[*connectionSession]struct{}
	sessionsL *sync.Mutex
}

func InitConnectionTracker(on bool) {
	if on {
		GlobalConnectionTracker = &connectionTracker{
			sessions:  map[*connectionSession]struct{}{},
			sessionsL: &sync.Mutex{},
		}
	} else {
		GlobalConnectionTracker = fakeConnectionTracker{}
	}
}

func (tracker *connectionTracker) Track() ConnectionSession {
	session := &connectionSession{
		tracker: tracker,
		stack:   string(debug.Stack()),
	}

	tracker.sessionsL.Lock()
	tracker.sessions[session] = struct{}{}
	tracker.sessionsL.Unlock()

	return session
}

func (tracker *connectionTracker) Current() []string {
	stacks := []string{}

	tracker.sessionsL.Lock()

	for session := range tracker.sessions {
		stacks = append(stacks, session.stack)
	}

	tracker.sessionsL.Unlock()

	return stacks
}

func (tracker *connectionTracker) remove(session *connectionSession) {
	tracker.sessionsL.Lock()
	delete(tracker.sessions, session)
	tracker.sessionsL.Unlock()
}

type connectionSession struct {
	tracker *connectionTracker
	stack   string
}

func (session *connectionSession) Release() {
	session.tracker.remove(session)
}

type fakeConnectionTracker struct {
}

func (t fakeConnectionTracker) Track() ConnectionSession {
	return fakeConnectionSession{}
}

func (t fakeConnectionTracker) Current() []string {
	return []string{"connection tracker is turned off"}
}

type fakeConnectionSession struct {
}

func (s fakeConnectionSession) Release() {
}
