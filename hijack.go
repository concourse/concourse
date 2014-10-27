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
	"strings"
	"syscall"

	garden_api "github.com/cloudfoundry-incubator/garden/api"
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

	var ttySpec *garden_api.TTYSpec

	rows, cols, err := pty.Getsize(os.Stdin)
	if err == nil {
		ttySpec = &garden_api.TTYSpec{
			WindowSize: &garden_api.WindowSize{
				Columns: cols,
				Rows:    rows,
			},
		}
	}

	spec := garden_api.ProcessSpec{
		Path: path,
		Args: args,
		Env:  []string{"TERM=" + os.Getenv("TERM")},

		Privileged: true,
		TTY:        ttySpec,
	}

	tlsConfig := &tls.Config{InsecureSkipVerify: insecure}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	client := &http.Client{Transport: transport}

	build := getBuild(c, client, reqGenerator)

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

	cconn, cbr := clientConn.Hijack()

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

	encoder := gob.NewEncoder(cconn)

	resized := make(chan os.Signal, 10)
	signal.Notify(resized, syscall.SIGWINCH)

	go func() {
		for {
			<-resized
			sendSize(encoder)
		}
	}()

	go io.Copy(&stdinWriter{encoder}, in)

	io.Copy(os.Stdout, cbr)
}

func getBuild(ctx *cli.Context, client *http.Client, reqGenerator *rata.RequestGenerator) resources.Build {
	jobName := ctx.String("job")
	buildName := ctx.String("build")

	if jobName != "" && buildName != "" {
		buildReq, err := reqGenerator.CreateRequest(
			routes.GetJobBuild,
			rata.Params{
				"job_name":   jobName,
				"build_name": buildName,
			},
			nil,
		)
		if err != nil {
			log.Fatalln("failed to create request", err)
		}

		buildResp, err := client.Do(buildReq)
		if err != nil {
			log.Fatalln("failed to get builds:", err)
		}

		if buildResp.StatusCode != http.StatusOK {
			log.Println("bad response when getting build:")
			buildResp.Body.Close()
			buildResp.Write(os.Stderr)
			os.Exit(1)
		}

		var build resources.Build
		err = json.NewDecoder(buildResp.Body).Decode(&build)
		if err != nil {
			log.Fatalln("failed to decode job:", err)
		}

		return build
	} else if jobName != "" {
		jobReq, err := reqGenerator.CreateRequest(
			routes.GetJob,
			rata.Params{"job_name": ctx.String("job")},
			nil,
		)
		if err != nil {
			log.Fatalln("failed to create request", err)
		}

		jobResp, err := client.Do(jobReq)
		if err != nil {
			log.Fatalln("failed to get builds:", err)
		}

		if jobResp.StatusCode != http.StatusOK {
			log.Println("bad response when getting job:")
			jobResp.Body.Close()
			jobResp.Write(os.Stderr)
			os.Exit(1)
		}

		var job resources.Job
		err = json.NewDecoder(jobResp.Body).Decode(&job)
		if err != nil {
			log.Fatalln("failed to decode job:", err)
		}

		if job.NextBuild != nil {
			return *job.NextBuild
		} else if job.FinishedBuild != nil {
			return *job.FinishedBuild
		} else {
			println("no job builds to hijack")
			os.Exit(1)
		}
	} else {
		buildsReq, err := reqGenerator.CreateRequest(
			routes.ListBuilds,
			nil,
			nil,
		)
		if err != nil {
			log.Fatalln("failed to create request", err)
		}

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

		return builds[0]
	}

	panic("unreachable")
}

func sendSize(enc *gob.Encoder) {
	rows, cols, err := pty.Getsize(os.Stdin)
	if err == nil {
		enc.Encode(thijack.ProcessPayload{
			TTYSpec: &garden_api.TTYSpec{
				WindowSize: &garden_api.WindowSize{
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
