package tsa

import (
	"code.cloudfoundry.org/lager/lagerctx"
	"context"
	"errors"
	"golang.org/x/crypto/ssh"
	"net"
	"time"
)

//
func KeepAlive(ctx context.Context, sshClient *ssh.Client, tcpConn *net.TCPConn, interval time.Duration, timeout time.Duration){
	logger := lagerctx.WithSession(ctx, "keepalive")
	keepAliveTicker := time.NewTicker(interval)

	for {

		sendKeepAliveRequest := make(chan error,1)
		go func (){
			defer close(sendKeepAliveRequest)
			// ignore reply; server may just not have handled it, since there's no
			// standard keepalive request name
			_, _, err := sshClient.Conn.SendRequest("keepalive", true, []byte("sup"))
			sendKeepAliveRequest <- err
		}()

		select {
		case <-time.After(timeout):
			logger.Error("timeout", errors.New("timed out sending keepalive request"))
			sshClient.Close()
			return
		case err := <-sendKeepAliveRequest:
			if err != nil {
				logger.Error("failed sending keepalive request", err)
				sshClient.Close()
				return
			}
		}

		select {
		case <-keepAliveTicker.C:
			logger.Debug("tick")

		case <-ctx.Done():
			logger.Debug("stopping")

			if err := tcpConn.SetKeepAlive(false); err != nil {
				logger.Error("failed-to-disable-keepalive", err)
				return
			}

			return
		}
	}
}