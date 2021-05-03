package network_test

import (
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/concourse/concourse/worker/network"
	"github.com/stretchr/testify/require"
)

func TestMTU(t *testing.T) {
	ifconfig, err := exec.LookPath("ifconfig")
	if err != nil {
		t.Skip("ifconfig is required")
	}
	iface, err := net.InterfaceByIndex(1)
	require.NoError(t, err)

	ifconfigOutput, err := exec.Command(ifconfig, iface.Name).CombinedOutput()
	require.NoError(t, err)

	groups := regexp.MustCompile(`(MTU|mtu).(\d+)`).FindStringSubmatch(string(ifconfigOutput))
	mtuStr := groups[2]
	expectMTU, err := strconv.Atoi(mtuStr)
	require.NoError(t, err)

	addrs, err := iface.Addrs()
	require.NoError(t, err)
	ipAddr := strings.Split(addrs[0].String(), "/")[0]

	mtu, err := network.MTU(ipAddr)
	require.NoError(t, err)

	require.Equal(t, expectMTU, mtu)
}
