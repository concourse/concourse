package vault

import (
	"fmt"
	"testing"
	"time"
)

type MockAuther struct {
	LoggedIn   chan bool
	Renewed    chan bool
	Delay      time.Duration
	LoginError error
	RenewError error
}

func (ma *MockAuther) Login() (time.Duration, error) {
	loggedIn := true
	if ma.LoginError != nil {
		loggedIn = false
	}

	ma.LoggedIn <- loggedIn
	return ma.Delay, ma.LoginError
}
func (ma *MockAuther) Renew() (time.Duration, error) {
	renewed := true
	if ma.RenewError != nil {
		renewed = false
	}

	ma.Renewed <- renewed
	return ma.Delay, ma.RenewError
}

func TestReAuther(t *testing.T) {
	testWithoutVaultErrors(t)
	testExponentialBackoff(t)
}

func testWithoutVaultErrors(t *testing.T) {
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

func testExponentialBackoff(t *testing.T) {
	maxRetryInterval := 2 * time.Second
	buffer := 300 * time.Millisecond

	ma := &MockAuther{
		LoggedIn:   make(chan bool, 1),
		Renewed:    make(chan bool, 1),
		Delay:      1 * time.Second,
		LoginError: fmt.Errorf("Could not login to Vault"),
	}
	ra := NewReAuther(ma, 0, 1*time.Second, maxRetryInterval)

	select {
	case <-ra.LoggedIn():
		t.Error("error, shouldn't have logged in")
	case <-time.After(1 * time.Second):
	}

	// Make enough login attempts to reach maxRetryInterval
	var lastRetryInterval time.Duration
	for i := 0; i < 4; i++ {
		start := time.Now()
		select {
		case li := <-ma.LoggedIn:
			lastRetryInterval = time.Now().Sub(start)
			if li {
				t.Error("error, shouldn't have logged in succesfully")
			}
		case <-time.After(maxRetryInterval * 2):
			t.Fatal("Took too long to retry login")
		}
	}

	if lastRetryInterval < (maxRetryInterval - buffer) {
		t.Error("maxRetryInterval reached, but login was reattempted before maxRetryInterval")
	}
}
