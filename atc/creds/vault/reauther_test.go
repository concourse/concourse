package vault

import (
	"fmt"
	"testing"
	"time"

	"code.cloudfoundry.org/lager/lagertest"

	"github.com/cenkalti/backoff"
)

type MockAuther struct {
	LoginAttempt chan bool
	Renewed      chan bool
	Delay        time.Duration
	LoginError   error
	RenewError   error
}

func (ma *MockAuther) Login() (time.Duration, error) {
	loggedIn := true
	if ma.LoginError != nil {
		loggedIn = false
	}

	ma.LoginAttempt <- loggedIn
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
		LoginAttempt: make(chan bool, 1),
		Renewed:      make(chan bool, 1),
		Delay:        1 * time.Second,
	}
	logger := lagertest.NewTestLogger("vault-test")
	ra := NewReAuther(logger, ma, 10*time.Second, 1*time.Second, 64*time.Second)
	select {
	case <-ra.LoggedIn():
	case <-time.After(1 * time.Second):
		t.Fatal("Didn't issue login within timeout")
	}

	select {
	case li := <-ma.LoginAttempt:
		if !li {
			t.Error("Error, should have logged in after loggedin closed")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Didn't issue login within timeout")
	}

	select {
	case <-ma.LoginAttempt:
		t.Error("Should not have logged in again")
	case r := <-ma.Renewed:
		if !r {
			t.Error("Error, should have renewed in before the delay")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Didn't issue login within timeout")
	}

	select {
	case <-ma.LoginAttempt:
		t.Error("Should not have logged in again")
	case r := <-ma.Renewed:
		if !r {
			t.Error("Error, should have renewed again in before the delay")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Didn't issue login within timeout")
	}

	select {
	case <-ma.LoginAttempt:
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

	ma := &MockAuther{
		LoginAttempt: make(chan bool, 1),
		Renewed:      make(chan bool, 1),
		Delay:        1 * time.Second,
		LoginError:   fmt.Errorf("Could not login to Vault"),
	}
	logger := lagertest.NewTestLogger("vault-test")
	ra := NewReAuther(logger, ma, 0, 1*time.Second, maxRetryInterval)

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
		case li := <-ma.LoginAttempt:
			lastRetryInterval = time.Since(start)
			if li {
				t.Error("error, shouldn't have logged in succesfully")
			}
		case <-time.After(maxRetryInterval * 2):
			t.Fatal("Took too long to retry login")
		}
	}

	// default randomization factor is 0.5
	smallestMaxRandomizedInterval := float64(maxRetryInterval) * (1 - backoff.DefaultRandomizationFactor)

	if float64(lastRetryInterval) < smallestMaxRandomizedInterval {
		t.Error("maxRetryInterval reached, but login was reattempted before maxRetryInterval")
	}
}
