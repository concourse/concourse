//go:build linux

package runtime

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"code.cloudfoundry.org/localip"
)

var (
	// Regular expressions used for DNS resolution parsing
	loopbackNameserverRegexp = regexp.MustCompile(`^\s*nameserver\s+127\.0\.0\.\d+\s*$`)
	loopbackIPRegexp         = regexp.MustCompile(`127\.\d{1,3}\.\d{1,3}\.\d{1,3}`)
)

// Parse resolve.conf file from the provided path.
// implementation is based on guardian's implementation
// here: https://github.com/cloudfoundry/guardian/blob/master/kawasaki/dns/resolv_compiler.go
func ParseHostResolveConf(path string) ([]string, error) {
	resolvConf, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read host's resolv.conf: %w", err)
	}

	resolvContents := string(resolvConf)

	if loopbackNameserverRegexp.MatchString(resolvContents) {
		ip, err := localip.LocalIP()
		if err != nil {
			return nil, err
		}
		return []string{"nameserver " + ip}, nil
	}

	var entries []string

	for _, resolvEntry := range strings.Split(strings.TrimSpace(resolvContents), "\n") {
		if resolvEntry == "" {
			continue
		}

		if !strings.HasPrefix(resolvEntry, "nameserver") {
			entries = append(entries, strings.TrimSpace(resolvEntry))
			continue
		}

		if !loopbackIPRegexp.MatchString(strings.TrimSpace(resolvEntry)) {
			nameserverFields := strings.Fields(resolvEntry)
			if len(nameserverFields) != 2 {
				continue
			}
			entries = append(entries, strings.Join(nameserverFields, " "))
		}
	}

	return entries, nil
}
