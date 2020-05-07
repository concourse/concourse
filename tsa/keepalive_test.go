package tsa_test

import (
	"context"
	"errors"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
	"github.com/concourse/concourse/tsa"
	"github.com/concourse/concourse/tsa/tsafakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("KeepAlive", func() {
	var (
		ctx context.Context
		sshClient *ssh.Client
		fakeConn *tsafakes.FakeConn
		tcpConn *net.TCPConn
	)

	BeforeEach(func(){
		ctx = context.Background()

		fakeConn = &tsafakes.FakeConn{}

		sshClient = &ssh.Client{
			Conn: fakeConn,
		}

		tcpConn = &net.TCPConn{}

	})

	JustBeforeEach(func(){
		go tsa.KeepAlive(ctx, sshClient, tcpConn, time.Millisecond * 50, time.Millisecond * 50)

	})

	Context("when SendRequest fails", func(){
		BeforeEach(func(){
			fakeConn.SendRequestReturns(false, []byte{}, errors.New("some foo error"))
		})
		It("closes the connection", func(){
			Eventually(fakeConn.CloseCallCount).Should(Equal(1))
		})
	})

	Context("when SendRequest times out", func(){
		BeforeEach(func(){
			fakeConn.SendRequestStub = func(name string, wantReply bool, payload []byte) (bool, []byte, error){
				time.Sleep(time.Millisecond * 500)
				return true, []byte{}, nil
			}
		})
		It("closes the connection", func(){
			Eventually(fakeConn.CloseCallCount).Should(Equal(1))
		})
	})

})

