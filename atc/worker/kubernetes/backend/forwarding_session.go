package backend

import (
	"bytes"
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/tools/portforward"
)

type ForwardingSession struct {
	stopC chan struct{}
}

func (s ForwardingSession) Close() error {
	s.stopC <- struct{}{}
	return nil
}

func NewForwardingSession(
	dialer httpstream.Dialer,
	remotePort string,
) (sess ForwardingSession, port string, err error) {

	readyC := make(chan struct{}, 1)
	out, errOut := new(bytes.Buffer), new(bytes.Buffer)

	sess.stopC = make(chan struct{}, 1)

	fw, err := portforward.NewOnAddresses(
		dialer,
		[]string{"localhost"}, []string{"0:" + remotePort},
		sess.stopC, readyC,
		out, errOut,
	)
	if err != nil {
		err = fmt.Errorf("new on address: %w", err)
		return
	}

	go func() {
		err = fw.ForwardPorts()
		if err != nil {
			err = fmt.Errorf("forward ports: %w", err)
		}
	}()

	for range readyC {
	}

	if len(errOut.String()) != 0 {
		panic(errOut.String())
	} else if len(out.String()) != 0 {
		fmt.Println(out.String())
	}

	ports, err := fw.GetPorts()
	if err != nil {
		err = fmt.Errorf("get ports: %w", err)
		return
	}

	port = strconv.Itoa(int(ports[0].Local))

	return
}
