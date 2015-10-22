// +build !windows

package commands

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/concourse/fly/pty"
)

func askToConfirm(prompt string) bool {
	fmt.Printf("%s (y/n): ", prompt)

	var in io.Reader

	t, err := pty.OpenRawTerm()
	if err == nil {
		in = t
	} else {
		in = os.Stdin
	}

	ans := make([]byte, 1)
	n, err := in.Read(ans)

	if t != nil {
		err = t.Restore()
		if err != nil {
			println("failed to restore terminal: " + err.Error())
		}

		t.Write(ans)
		t.Write([]byte("\n"))
	}

	if err != nil {
		log.Fatalln("failed to read response:", err)
	}

	if n == 0 {
		log.Fatalln("no response")
	}

	return ans[0] == 'y'
}
