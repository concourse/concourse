package flag

import (
	"fmt"
	"net"
)

type IP struct {
	net.IP
}

func (ip *IP) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var value string
	err := unmarshal(&value)
	if err != nil {
		return err
	}

	return ip.Set(value)
}

// Can be removed once flags are deprecated
func (ip *IP) Set(value string) error {
	parsedIP := net.ParseIP(value)
	if parsedIP == nil {
		return fmt.Errorf("parse IP: '%s'", ip)
	}

	ip.IP = parsedIP

	return nil
}

// Can be removed once flags are deprecated
func (ip *IP) String() string {
	return ip.String()
}

// Can be removed once flags are deprecated
func (ip *IP) Type() string {
	return "IP"
}
