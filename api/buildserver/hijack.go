package buildserver

import (
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"

	"github.com/pivotal-golang/lager"
)

func (s *Server) HijackBuild(w http.ResponseWriter, r *http.Request) {
	buildID, err := strconv.Atoi(r.FormValue(":build_id"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	hLog := s.logger.Session("hijack", lager.Data{
		"build": buildID,
	})

	build, err := s.db.GetBuild(buildID)
	if err != nil {
		hLog.Error("failed-to-get-build", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if build.HijackURL == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	hijackURL, err := url.Parse(build.HijackURL)
	if err != nil {
		hLog.Error("failed-to-parse-url", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	conn, err := net.Dial("tcp", hijackURL.Host)
	if err != nil {
		hLog.Error("failed-to-dial-turbine", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	req, err := http.NewRequest(r.Method, build.HijackURL, r.Body)
	if err != nil {
		hLog.Error("failed-to-create-request", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	client := httputil.NewClientConn(conn, nil)

	resp, err := client.Do(req)
	if err != nil {
		hLog.Error("failed-to-hijack", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if resp.StatusCode != http.StatusOK {
		hLog.Info("bad-hijack-response", lager.Data{
			"status": resp.Status,
		})

		resp.Write(w)
		return
	}

	w.WriteHeader(http.StatusOK)

	sconn, sbr, err := w.(http.Hijacker).Hijack()
	if err != nil {
		hLog.Error("failed-to-hijack", err)
		return
	}

	cconn, cbr := client.Hijack()

	defer cconn.Close()
	defer sconn.Close()

	hLog.Info("hijacked")

	go io.Copy(cconn, sbr)

	io.Copy(sconn, cbr)
}
