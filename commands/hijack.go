// +build !windows

package commands

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/codegangsta/cli"
	"github.com/concourse/atc"
	"github.com/kr/pty"
	"github.com/mgutz/ansi"
	"github.com/pkg/term"
	"github.com/tedsuo/rata"
)

func Hijack(c *cli.Context) {
	target := returnTarget(c.GlobalString("target"))
	insecure := c.GlobalBool("insecure")
	privileged := true
	stepType := c.String("step-type")
	stepName := c.String("step-name")
	check := c.String("check")

	reqGenerator := rata.NewRequestGenerator(target, atc.Routes)

	argv := c.Args()

	var path string
	var args []string

	switch len(argv) {
	case 0:
		path = "bash"
	case 1:
		path = argv[0]
	default:
		path = argv[0]
		args = argv[1:]
	}

	var ttySpec *atc.HijackTTYSpec

	rows, cols, err := pty.Getsize(os.Stdin)
	if err == nil {
		ttySpec = &atc.HijackTTYSpec{
			WindowSize: atc.HijackWindowSize{
				Columns: cols,
				Rows:    rows,
			},
		}
	}

	spec := atc.HijackProcessSpec{
		Path: path,
		Args: args,
		Env:  []string{"TERM=" + os.Getenv("TERM")},

		Privileged: privileged,
		TTY:        ttySpec,
	}

	tlsConfig := &tls.Config{InsecureSkipVerify: insecure}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	client := &http.Client{Transport: transport}

	reqValues := url.Values{}

	if check == "" {
		build := getBuild(c, client, reqGenerator)
		reqValues["build-id"] = []string{strconv.Itoa(build.ID)}
		if stepType != "" {
			reqValues["type"] = []string{stepType}
		}
		reqValues["name"] = []string{stepName}
	} else {
		reqValues["type"] = []string{"check"}
		reqValues["name"] = []string{check}
	}

	payload, err := json.Marshal(spec)
	if err != nil {
		log.Fatalln("failed to marshal process spec:", err)
	}

	hijackReq, err := reqGenerator.CreateRequest(
		atc.Hijack,
		rata.Params{},
		bytes.NewBuffer(payload),
	)
	if err != nil {
		log.Fatalln("failed to create hijack request:", err)
	}

	if hijackReq.URL.User != nil {
		hijackReq.Header.Add("Authorization", basicAuth(hijackReq.URL.User))
		hijackReq.URL.User = nil
	}

	hijackReq.URL.RawQuery = reqValues.Encode()

	conn, err := dialEndpoint(hijackReq.URL, tlsConfig)
	if err != nil {
		log.Fatalln("failed to dial hijack endpoint:", err)
	}

	clientConn := httputil.NewClientConn(conn, nil)

	resp, err := clientConn.Do(hijackReq)
	if err != nil {
		log.Fatalln("failed to hijack:", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Println("bad response when hijacking:")
		resp.Body.Close()
		resp.Write(os.Stderr)
		os.Exit(1)
	}

	os.Exit(hijack(clientConn.Hijack()))
}

func hijack(conn net.Conn, br *bufio.Reader) int {
	var in io.Reader

	term, err := term.Open(os.Stdin.Name())
	if err == nil {
		err = term.SetRaw()
		if err != nil {
			log.Fatalln("failed to set raw:", term)
		}

		defer term.Restore()

		in = term
	} else {
		in = os.Stdin
	}

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(br)

	resized := make(chan os.Signal, 10)
	signal.Notify(resized, syscall.SIGWINCH)

	go func() {
		for {
			<-resized
			// TODO json race
			sendSize(encoder)
		}
	}()

	go io.Copy(&stdinWriter{encoder}, in)

	var exitStatus int
	for {
		var output atc.HijackOutput
		err := decoder.Decode(&output)
		if err != nil {
			break
		}

		if output.ExitStatus != nil {
			exitStatus = *output.ExitStatus
		} else if len(output.Error) > 0 {
			fmt.Fprintf(os.Stderr, "%s\n", ansi.Color(output.Error, "red+b"))
			exitStatus = 255
		} else if len(output.Stdout) > 0 {
			os.Stdout.Write(output.Stdout)
		} else if len(output.Stderr) > 0 {
			os.Stderr.Write(output.Stderr)
		}
	}

	return exitStatus
}

func sendSize(enc *json.Encoder) {
	rows, cols, err := pty.Getsize(os.Stdin)
	if err == nil {
		enc.Encode(atc.HijackInput{
			TTYSpec: &atc.HijackTTYSpec{
				WindowSize: atc.HijackWindowSize{
					Columns: cols,
					Rows:    rows,
				},
			},
		})
	}
}

type stdinWriter struct {
	enc *json.Encoder
}

func (w *stdinWriter) Write(d []byte) (int, error) {
	err := w.enc.Encode(atc.HijackInput{
		Stdin: d,
	})
	if err != nil {
		return 0, err
	}

	return len(d), nil
}

func basicAuth(user *url.Userinfo) string {
	username := user.Username()
	password, _ := user.Password()
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
}

var canonicalPortMap = map[string]string{
	"http":  "80",
	"https": "443",
}

func dialEndpoint(url *url.URL, tlsConfig *tls.Config) (net.Conn, error) {
	addr := canonicalAddr(url)

	if url.Scheme == "https" {
		return tls.Dial("tcp", addr, tlsConfig)
	} else {
		return net.Dial("tcp", addr)
	}
}

func canonicalAddr(url *url.URL) string {
	host, port, err := net.SplitHostPort(url.Host)
	if err != nil {
		if strings.Contains(err.Error(), "missing port in address") {
			host = url.Host
			port = canonicalPortMap[url.Scheme]
		} else {
			log.Fatalln("invalid host:", err)
		}
	}

	return net.JoinHostPort(host, port)
}
