package beacon

import (
	"fmt"
	"net"
)

type IPFlag net.IP

func (f *IPFlag) UnmarshalFlag(value string) error {
	parsedIP := net.ParseIP(value)
	if parsedIP == nil {
		return fmt.Errorf("invalid IP: '%s'", value)
	}

	*f = IPFlag(parsedIP)

	return nil
}

func (f IPFlag) IP() net.IP {
	return net.IP(f)
}
