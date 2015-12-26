package atccmd

import (
	"fmt"
	"net"
)

type IPFlag string

func (f *IPFlag) UnmarshalFlag(value string) error {
	parsedIP := net.ParseIP(value)
	if parsedIP == nil {
		return fmt.Errorf("invalid IP: '%s'", value)
	}

	*f = IPFlag(parsedIP.String())

	return nil
}
