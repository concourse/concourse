package db

import (
	"runtime/debug"
	"sync"
)

var GlobalConnectionTracker ConnectionTracker

func init() {
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
	sessions sync.Map
}

func InitConnectionTracker(on bool) {
	if on {
		GlobalConnectionTracker = &connectionTracker{}
	} else {
		GlobalConnectionTracker = fakeConnectionTracker{}
	}
}

func (tracker *connectionTracker) Track() ConnectionSession {
	session := &connectionSession{
		tracker: tracker,
		stack:   string(debug.Stack()),
	}

	tracker.sessions.Store(session, struct{}{})
	return session
}

func (tracker *connectionTracker) Current() []string {
	var stacks []string

	tracker.sessions.Range(func(key, value any) bool {
		if session, ok := key.(*connectionSession); ok {
			stacks = append(stacks, session.stack)
		}
		return true
	})

	return stacks
}

func (tracker *connectionTracker) remove(session *connectionSession) {
	tracker.sessions.Delete(session)
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
