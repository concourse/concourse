package main

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/cloudfoundry-incubator/garden/warden"
	"github.com/codegangsta/cli"
	"github.com/concourse/atc/api/resources"
	"github.com/concourse/atc/api/routes"
	thijack "github.com/concourse/turbine/api/hijack"
	"github.com/kr/pty"
	"github.com/pkg/term"
	"github.com/tedsuo/rata"
)

func hijack(c *cli.Context) {
	atc := c.GlobalString("atcURL")
	insecure := c.GlobalBool("insecure")

	reqGenerator := rata.NewRequestGenerator(atc, routes.Routes)

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

	var ttySpec *warden.TTYSpec

	rows, cols, err := pty.Getsize(os.Stdin)
	if err == nil {
		ttySpec = &warden.TTYSpec{
			WindowSize: &warden.WindowSize{
				Columns: cols,
				Rows:    rows,
			},
		}
	}

	spec := warden.ProcessSpec{
		Path: path,
		Args: args,
		Env:  []string{"TERM=" + os.Getenv("TERM")},

		Privileged: true,
		TTY:        ttySpec,
	}

	buildsReq, err := reqGenerator.CreateRequest(
		routes.ListBuilds,
		nil,
		nil,
	)
	if err != nil {
		log.Fatalln("failed to create request", err)
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
	}

	client := &http.Client{Transport: transport}

	buildsResp, err := client.Do(buildsReq)
	if err != nil {
		log.Fatalln("failed to get builds:", err)
	}

	if buildsResp.StatusCode != http.StatusOK {
		log.Println("bad response when getting builds:")
		buildsResp.Body.Close()
		buildsResp.Write(os.Stderr)
		os.Exit(1)
	}

	var builds []resources.Build
	err = json.NewDecoder(buildsResp.Body).Decode(&builds)
	if err != nil {
		log.Fatalln("failed to decode builds:", err)
	}

	if len(builds) == 0 {
		println("no builds to hijack")
		os.Exit(1)
	}

	build := builds[0]

	payload, err := json.Marshal(spec)
	if err != nil {
		log.Fatalln("failed to marshal build spec:", err)
	}

	hijackReq, err := reqGenerator.CreateRequest(
		routes.HijackBuild,
		rata.Params{"build_id": strconv.Itoa(build.ID)},
		bytes.NewBuffer(payload),
	)
	if err != nil {
		log.Fatalln("failed to create hijack request:", err)
	}

	if hijackReq.URL.User != nil {
		hijackReq.Header.Add("Authorization", basicAuth(hijackReq.URL.User))
		hijackReq.URL.User = nil
	}

	conn, err := net.Dial("tcp", hijackReq.URL.Host)
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

	cconn, cbr := clientConn.Hijack()

	term, err := term.Open(os.Stdin.Name())
	if err != nil {
		log.Fatalln("failed to open terminal:", err)
	}

	err = term.SetRaw()
	if err != nil {
		log.Fatalln("failed to set raw:", term)
	}

	defer term.Restore()

	encoder := gob.NewEncoder(cconn)

	resized := make(chan os.Signal, 10)
	signal.Notify(resized, syscall.SIGWINCH)

	go func() {
		for {
			<-resized
			sendSize(encoder)
		}
	}()

	go io.Copy(&stdinWriter{encoder}, term)

	io.Copy(os.Stdout, cbr)
}

func sendSize(enc *gob.Encoder) {
	rows, cols, err := pty.Getsize(os.Stdin)
	if err == nil {
		enc.Encode(thijack.ProcessPayload{
			TTYSpec: &warden.TTYSpec{
				WindowSize: &warden.WindowSize{
					Columns: cols,
					Rows:    rows,
				},
			},
		})
	}
}

type stdinWriter struct {
	enc *gob.Encoder
}

func (w *stdinWriter) Write(d []byte) (int, error) {
	err := w.enc.Encode(thijack.ProcessPayload{
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
