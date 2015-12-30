package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/concourse/atc"
)

type Beacon struct {
	Worker atc.Worker
	Config BeaconConfig
}

func (beacon *Beacon) Forward() error {
	return nil
}

func (beacon *Beacon) Register(signals <-chan os.Signal, ready chan<- struct{}) error {
	client, err := beacon.Config.Dial()
	if err != nil {
		return fmt.Errorf("failed to dial: %s", err)
	}

	sess, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %s", err)
	}

	defer sess.Close()

	workerPayload, err := json.Marshal(beacon.Worker)
	if err != nil {
		return err
	}

	sess.Stdin = bytes.NewBuffer(workerPayload)
	sess.Stdout = os.Stdout
	sess.Stderr = os.Stderr

	err = sess.Start("register-worker")
	if err != nil {
		return err
	}

	close(ready)

	exited := make(chan error, 1)

	go func() {
		exited <- sess.Wait()
	}()

	select {
	case <-signals:
		sess.Close()
		return nil
	case err := <-exited:
		return err
	}

	return nil
}
