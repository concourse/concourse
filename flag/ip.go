package flag

import (
	"fmt"
	"net"
)

type IP struct {
	net.IP
}

func (f *IP) UnmarshalFlag(value string) error {
	parsedIP := net.ParseIP(value)
	if parsedIP == nil {
		return fmt.Errorf("invalid IP: '%s'", value)
	}

	f.IP = parsedIP

	return nil
}
