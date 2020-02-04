package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var (
	flagWaitForSignal = flag.String("wait-for-signal", "", "wait for a sigal (sigterm|sighup)")
	flagHttpGet       = flag.String("http-get", "", "website to perform an HTTP GET request against")
	flagWriteTenTimes = flag.String("write-many-times", "", "writes a string to stdout many times")

	signals = map[string]os.Signal{
		"sighup":  syscall.SIGHUP,
		"sigterm": syscall.SIGTERM,
	}
)

const defaultMessage = "hello world"

// waitForSignal blocks until the signal we care about (`sig`) is develired.
//
// ps.: all other signals are ignored.
//
func waitForSignal(sig string) {
	fmt.Println("setting up")

	s, found := signals[strings.ToLower(sig)]
	if !found {
		log.Fatal("signal %s not found - available: %v",
			sig, signals,
		)
	}

	ch := make(chan os.Signal)
	signal.Ignore()
	signal.Notify(ch, s)
	<-ch

	fmt.Println("got signaled!")
}

func httpGet(url string) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(url)
	if err != nil {
		log.Fatal("failed performing http get", err)
	}

	fmt.Println(resp.Status)
}

func writeTenTimes(content string) {
	for i := 0; i < 20; i++ {
		fmt.Println(content)
		time.Sleep(300 * time.Millisecond)
	}
}

func main() {
	flag.Parse()

	switch {
	case *flagWaitForSignal != "":
		waitForSignal(*flagWaitForSignal)
	case *flagHttpGet != "":
		httpGet(*flagHttpGet)
	case *flagWriteTenTimes != "":
		writeTenTimes(*flagWriteTenTimes)
	default:
		fmt.Println(defaultMessage)
	}
}
