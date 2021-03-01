package flag

import (
	"fmt"
	"net"
)

type IP struct {
	net.IP
}

func (i IP) MarshalYAML() (interface{}, error) {
	return i.String(), nil
}

func (i *IP) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var value string
	err := unmarshal(&value)
	if err != nil {
		return err
	}

	if value != "" {
		return i.Set(value)
	}

	return nil
}

// Can be removed once flags are deprecated
func (i *IP) Set(value string) error {
	parsedIP := net.ParseIP(value)
	if parsedIP == nil {
		return fmt.Errorf("parse IP: '%s'", i)
	}

	i.IP = parsedIP

	return nil
}

// Can be removed once flags are deprecated
func (i *IP) String() string {
	return i.String()
}

// Can be removed once flags are deprecated
func (i *IP) Type() string {
	return "IP"
}
