package network

import (
	"fmt"
	"net"
)

func MTU(ipAddr string) (int, error) {
	ip := net.ParseIP(ipAddr)
	networkIface, err := findNetworkInterface(ip)
	if err != nil {
		return 0, err
	}
	return networkIface.MTU, nil
}

func findNetworkInterface(ip net.IP) (net.Interface, error) {
	networkIfaces, err := net.Interfaces()
	if err != nil {
		return net.Interface{}, err
	}
	for _, networkIface := range networkIfaces {
		addrs, err := networkIface.Addrs()
		if err != nil {
			return net.Interface{}, err
		}
		if contains(addrs, ip) {
			return networkIface, nil
		}
	}
	return net.Interface{}, fmt.Errorf("no interface found for address %s", ip)
}

func contains(addresses []net.Addr, ip net.IP) bool {
	for _, address := range addresses {
		_, cidr, err := net.ParseCIDR(address.String())
		if err != nil {
			continue
		}
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}
