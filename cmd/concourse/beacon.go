package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
)

const gardenForwardAddr = "0.0.0.0:7777"
const baggageclaimForwardAddr = "0.0.0.0:7788"

type Beacon struct {
	Logger lager.Logger
	Worker atc.Worker
	Config BeaconConfig
}

func (beacon *Beacon) Forward(signals <-chan os.Signal, ready chan<- struct{}) error {
	client, err := beacon.Config.Dial()
	if err != nil {
		return fmt.Errorf("failed to dial: %s", err)
	}

	defer client.Close()

	return beacon.run(
		"forward-worker "+
			"--garden "+gardenForwardAddr+" "+
			"--baggageclaim "+baggageclaimForwardAddr,
		client,
		signals,
		ready,
	)
}

func (beacon *Beacon) Register(signals <-chan os.Signal, ready chan<- struct{}) error {
	client, err := beacon.Config.Dial()
	if err != nil {
		return fmt.Errorf("failed to dial: %s", err)
	}

	defer client.Close()

	return beacon.run("register-worker", client, signals, ready)
}

func (beacon *Beacon) LandWorker(signals <-chan os.Signal, ready chan<- struct{}) error {
	client, err := beacon.Config.Dial()
	if err != nil {
		return fmt.Errorf("failed to dial: %s", err)
	}

	defer client.Close()

	return beacon.run("land-worker", client, signals, ready)
}

func (beacon *Beacon) run(command string, client *ssh.Client, signals <-chan os.Signal, ready chan<- struct{}) error {
	keepaliveFailed, cancelKeepalive := beacon.keepAlive(client)

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

	gardenRemoteListener, err := client.Listen("tcp", gardenForwardAddr)
	if err != nil {
		return fmt.Errorf("failed to listen remotely: %s", err)
	}

	go beacon.proxyListenerTo(gardenRemoteListener, beacon.Worker.GardenAddr)

	bcURL, err := url.Parse(beacon.Worker.BaggageclaimURL)
	if err != nil {
		return fmt.Errorf("failed to parse baggageclaim url: %s", err)
	}

	baggageclaimRemoteListener, err := client.Listen("tcp", baggageclaimForwardAddr)
	if err != nil {
		return fmt.Errorf("failed to listen remotely: %s", err)
	}

	go beacon.proxyListenerTo(baggageclaimRemoteListener, bcURL.Host)

	close(ready)

	exited := make(chan error, 1)

	go func() {
		exited <- sess.Wait()
	}()

	select {
	case <-signals:
		close(cancelKeepalive)
		sess.Close()

		<-exited

		// don't bother waiting for keepalive

		return nil
	case err := <-exited:
		return err
	case err := <-keepaliveFailed:
		return err
	}

	return nil
}

func (beacon *Beacon) keepAlive(conn ssh.Conn) (<-chan error, chan<- struct{}) {
	logger := beacon.Logger.Session("keepalive")

	errs := make(chan error, 1)

	kas := time.NewTicker(5 * time.Second)
	cancel := make(chan struct{})

	go func() {
		for {
			// ignore reply; server may just not have handled it, since there's no
			// standard keepalive request name

			_, _, err := conn.SendRequest("keepalive", true, []byte("sup"))
			if err != nil {
				logger.Error("failed", err)
				errs <- err
				return
			}

			logger.Debug("ok")

			select {
			case <-kas.C:
			case <-cancel:
				errs <- nil
				return
			}
		}
	}()

	return errs, cancel
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

	pipe := func(to io.WriteCloser, from io.ReadCloser) {
		// if either end breaks, close both ends to ensure they're both unblocked,
		// otherwise io.Copy can block forever if e.g. reading after write end has
		// gone away
		defer to.Close()
		defer from.Close()
		defer wg.Done()

		io.Copy(to, from)
	}

	wg.Add(1)
	go pipe(lConn, rConn)

	wg.Add(1)
	go pipe(rConn, lConn)

	wg.Wait()
}
