package main

type execRequest struct {
	Command string
}

type tcpipForwardRequest struct {
	BindIP   string
	BindPort uint32
}

type tcpipForwardResponse struct {
	BoundPort uint32
}

type forwardTCPIPChannelRequest struct {
	ForwardIP   string
	ForwardPort uint32
	OriginIP    string
	OriginPort  uint32
}
