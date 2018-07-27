package vault

import (
	"testing"
	"time"
)

type MockAuther struct {
	LoggedIn chan bool
	Renewed  chan bool
	Delay    time.Duration
}

func (ma *MockAuther) Login() (time.Duration, error) {
	ma.LoggedIn <- true
	return ma.Delay, nil
}
func (ma *MockAuther) Renew() (time.Duration, error) {
	ma.Renewed <- true
	return ma.Delay, nil
}

func TestReAuther(t *testing.T) {
	ma := &MockAuther{
		LoggedIn: make(chan bool, 1),
		Renewed:  make(chan bool, 1),
		Delay:    1 * time.Second,
	}
	ra := NewReAuther(ma, 10*time.Second, 1*time.Second, 64*time.Second)
	select {
	case <-ra.LoggedIn():
	case <-time.After(1 * time.Second):
		t.Fatal("Didn't issue login within timeout")
	}

	select {
	case li := <-ma.LoggedIn:
		if !li {
			t.Error("Error, should have logged in after loggedin closed")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Didn't issue login within timeout")
	}

	select {
	case <-ma.LoggedIn:
		t.Error("Should not have logged in again")
	case r := <-ma.Renewed:
		if !r {
			t.Error("Error, should have renewed in before the delay")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Didn't issue login within timeout")
	}

	select {
	case <-ma.LoggedIn:
		t.Error("Should not have logged in again")
	case r := <-ma.Renewed:
		if !r {
			t.Error("Error, should have renewed again in before the delay")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Didn't issue login within timeout")
	}

	select {
	case <-ma.LoggedIn:
	case r := <-ma.Renewed:
		if !r {
			t.Error("Error, should have logged in again after the maxTTL")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Didn't issue login within timeout")
	}

}
