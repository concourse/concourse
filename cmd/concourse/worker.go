package main

type WorkerCommand struct {
	WorkDir string `long:"work-dir" required:"true" description:"Directory in which to place container data."`
	PeerIP  string `long:"peer-ip" default:"127.0.0.1" description:"IP used to reach this node from other Concourse nodes"`
}
