package atccmd

import (
	"fmt"
	"net"

	"gopkg.in/yaml.v3"
)

type IP struct {
	net.IP
}

func (i *IP) UnmarshalYAML(node yaml.Node) error {
	var ip string
	err := node.Decode(&ip)
	if err != nil {
		return err
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return fmt.Errorf("invalid IP: '%s'", ip)
	}

	i.IP = parsedIP

	return nil
}
