package backend

import (
	"fmt"
	"net/http"

	"code.cloudfoundry.org/garden"
	log "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/client-go/util/exec"
)

type Container struct {
	ns, pod, ip string
	clientset   *kubernetes.Clientset
	cfg         *rest.Config
}

func NewContainer(
	ns, pod, ip string,
	clientset *kubernetes.Clientset,
	cfg *rest.Config,
) *Container {
	return &Container{
		ns:        ns,
		pod:       pod,
		ip:        ip,
		clientset: clientset,
		cfg:       cfg,
	}
}

func (c *Container) Handle() string {
	return c.pod
}

func (c *Container) IP() string {
	return c.ip
}

func (c *Container) PortForward(remotePort string) (sess ForwardingSession, port string, err error) {
	transport, upgrader, err := spdy.RoundTripperFor(c.cfg)
	if err != nil {
		err = fmt.Errorf("rountripper: %w", err)
		return
	}

	req := c.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(c.ns).
		Name(c.pod).
		SubResource("portforward")

	dialer := spdy.NewDialer(
		upgrader, &http.Client{Transport: transport},
		"POST", req.URL(),
	)

	sess, port, err = NewForwardingSession(dialer, remotePort)
	if err != nil {
		err = fmt.Errorf("new forwarding sess: %w", err)
		return
	}

	return
}

func (c *Container) Run(procSpec garden.ProcessSpec, procIO garden.ProcessIO) (status int, err error) {
	sess := log.WithFields(log.Fields{
		"action": "run",
		"ns":     c.ns,
		"pod":    c.pod,
		"cmd":    procSpec.Path,
	})

	sess.Info("start")
	defer sess.Info("finished")

	req := c.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(c.pod).
		Namespace(c.ns).
		SubResource("exec").
		Param("container", MainContainerName)

	req.VersionedParams(&apiv1.PodExecOptions{
		Container: MainContainerName,
		Command:   append([]string{procSpec.Path}, procSpec.Args...),
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
	}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(c.cfg, "POST", req.URL())
	if err != nil {
		err = fmt.Errorf("new spdy executor: %w", err)
		return
	}

	err = executor.Stream(remotecommand.StreamOptions{
		Stdin:  procIO.Stdin,
		Stdout: procIO.Stdout,
		Stderr: procIO.Stderr,
	})
	if err != nil {
		exitErr, ok := err.(exec.ExitError)
		if !ok {
			err = fmt.Errorf("stream: %w", err)
			return
		}

		err = nil
		status = exitErr.ExitStatus()
		return
	}

	return
}
