package mtu

import (
	"fmt"
	"net"
	"strings"
)

func MTU(ipAddr string) (int, error) {
	networkIface, err := findNetworkInterface(ipAddr)
	if err != nil {
		return 0, err
	}
	return networkIface.MTU, nil
}

func findNetworkInterface(ipAddr string) (net.Interface, error) {
	networkIfaces, err := net.Interfaces()
	if err != nil {
		return net.Interface{}, err
	}
	for _, networkIface := range networkIfaces {
		addrs, err := networkIface.Addrs()
		if err != nil {
			return net.Interface{}, err
		}
		if contains(addrs, ipAddr) {
			return networkIface, nil
		}
	}
	return net.Interface{}, fmt.Errorf("no interface found for address %s", ipAddr)
}

func contains(addresses []net.Addr, ip string) bool {
	for _, address := range addresses {
		if strings.HasPrefix(address.String(), ip+"/") {
			return true
		}
	}
	return false
}
