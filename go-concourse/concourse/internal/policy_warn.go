package internal

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

func showPolicyCheckWarningIfHas(response *http.Response) {
	warn := response.Header.Get("X-Concourse-Policy-Check-Warning")
	if warn == "" {
		return
	}

	parts := strings.Split(warn, " * ")
	warnText := fmt.Sprintf("\x1b[1;33m%s\x1b[0m\n", parts[0])
	for i := 1; i < len(parts); i++ {
		warnText += fmt.Sprintf("\x1b[1;33m * %s\x1b[0m\n", parts[i])
	}
	warnText += fmt.Sprintln("\x1b[33mWARNING: unblocking from the policy check failure for soft enforcement\x1b[0m")
	fmt.Fprintln(os.Stderr, warnText)
}
