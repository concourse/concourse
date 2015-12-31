package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"

	"golang.org/x/crypto/ssh"

	"github.com/concourse/atc"
)

type Beacon struct {
	Worker atc.Worker
	Config BeaconConfig
}

func (beacon *Beacon) Forward(signals <-chan os.Signal, ready chan<- struct{}) error {
	client, err := beacon.Config.Dial()
	if err != nil {
		return fmt.Errorf("failed to dial: %s", err)
	}

	defer client.Close()

	return beacon.run("forward-worker", client, signals, ready)
}

func (beacon *Beacon) Register(signals <-chan os.Signal, ready chan<- struct{}) error {
	client, err := beacon.Config.Dial()
	if err != nil {
		return fmt.Errorf("failed to dial: %s", err)
	}

	defer client.Close()

	return beacon.run("register-worker", client, signals, ready)
}

func (beacon *Beacon) run(command string, client *ssh.Client, signals <-chan os.Signal, ready chan<- struct{}) error {
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

	err = sess.Start(command)
	if err != nil {
		return err
	}

	gardenRemoteListener, err := client.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		return fmt.Errorf("failed to listen remotely: %s", err)
	}

	go beacon.proxyListenerTo(gardenRemoteListener, beacon.Worker.GardenAddr)

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

func (beacon *Beacon) proxyListenerTo(listener net.Listener, addr string) {
	for {
		rConn, err := listener.Accept()
		if err != nil {
			break
		}

		go beacon.handleForwardedConn(rConn, addr)
	}
}

func (beacon *Beacon) handleForwardedConn(rConn net.Conn, addr string) {
	defer rConn.Close()

	lConn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Println("failed to forward remote connection:", err)
		return
	}

	wg := new(sync.WaitGroup)

	wg.Add(1)
	go func() {
		defer wg.Done()
		io.Copy(lConn, rConn)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		io.Copy(rConn, lConn)
	}()

	wg.Wait()
}
