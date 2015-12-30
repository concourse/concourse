package main

type WorkerCommand struct {
	WorkDir string `long:"work-dir" required:"true" description:"Directory in which to place container data."`

	PeerIP string `long:"peer-ip" description:"IP used to reach this worker from the ATC nodes. If omitted, the worker will be forwarded through the SSH connection to the TSA."`

	TSA struct {
		Host             string `long:"host" default:"127.0.0.1" description:"TSA host to forward the worker through."`
		Port             int    `long:"port" default:"2222" description:"TSA port to connect to."`
		PublicKey        string `long:"public-key" description:"Public key to expect from the TSA."`
		WorkerPrivateKey string `long:"worker-private-key" description:"Key to use when authenticating to the TSA."`
	} `group:"TSA Configuration" namespace:"tsa"`
}
