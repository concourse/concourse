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
	defaultHostResolvConfPath     = "/etc/resolv.conf"
	systemdResolvedResolvConfPath = "/run/systemd/resolve/resolv.conf"
	readFileFn                    = os.ReadFile
	localIPFn                     = localip.LocalIP

	// Regular expressions used for DNS resolution parsing
	loopbackNameserverRegexp = regexp.MustCompile(`(?m)^\s*nameserver\s+(?:127\.\d{1,3}\.\d{1,3}\.\d{1,3}|::1)\s*$`)
	loopbackIPRegexp         = regexp.MustCompile(`(?:127\.\d{1,3}\.\d{1,3}\.\d{1,3}|::1)`)
)

// Parse resolve.conf file from the provided path.
// implementation is based on guardian's implementation
// here: https://github.com/cloudfoundry/guardian/blob/master/kawasaki/dns/resolv_compiler.go
func ParseHostResolveConf(path string) ([]string, error) {
	resolvContents, err := readResolvConf(path)
	if err != nil {
		return nil, err
	}

	entries := parseResolvEntries(resolvContents)
	if hasNameserverEntry(entries) {
		return entries, nil
	}

	if loopbackNameserverRegexp.MatchString(resolvContents) {
		if path == defaultHostResolvConfPath {
			entries, err := parseResolvConf(systemdResolvedResolvConfPath)
			if err == nil && hasNameserverEntry(entries) {
				return entries, nil
			}
		}

		ip, err := localIPFn()
		if err != nil {
			return nil, err
		}
		return []string{"nameserver " + ip}, nil
	}

	return entries, nil
}

func readResolvConf(path string) (string, error) {
	resolvConf, err := readFileFn(path)
	if err != nil {
		return "", fmt.Errorf("unable to read host's resolv.conf: %w", err)
	}

	return string(resolvConf), nil
}

func parseResolvConf(path string) ([]string, error) {
	resolvContents, err := readResolvConf(path)
	if err != nil {
		return nil, err
	}

	return parseResolvEntries(resolvContents), nil
}

func hasNameserverEntry(entries []string) bool {
	for _, entry := range entries {
		if strings.HasPrefix(entry, "nameserver ") {
			return true
		}
	}

	return false
}

func parseResolvEntries(resolvContents string) []string {
	var entries []string

	for resolvEntry := range strings.SplitSeq(strings.TrimSpace(resolvContents), "\n") {
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

	return entries
}
