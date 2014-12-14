package buildserver

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"strconv"

	"github.com/concourse/atc/engine"
	"github.com/concourse/turbine"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"
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

	if build.EngineMetadata == "" {
		hLog.Error("missing-engine-metadata", nil)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var metadata engine.TurbineMetadata
	err = json.Unmarshal([]byte(build.EngineMetadata), &metadata)
	if err != nil {
		hLog.Error("failed-to-unmarshal-metadata", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	generator := rata.NewRequestGenerator(metadata.Endpoint, turbine.Routes)

	hijack, err := generator.CreateRequest(
		turbine.HijackBuild,
		rata.Params{"guid": metadata.Guid},
		r.Body,
	)
	if err != nil {
		hLog.Error("failed-to-construct-hijack-request", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	hijackURL := hijack.URL

	conn, err := net.Dial("tcp", hijackURL.Host)
	if err != nil {
		hLog.Error("failed-to-dial-turbine", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	client := httputil.NewClientConn(conn, nil)

	resp, err := client.Do(hijack)
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
