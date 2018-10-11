package cliserver

import (
	"net/http"
	"path/filepath"
	"strings"
)

func (s *Server) Download(w http.ResponseWriter, r *http.Request) {
	if s.cliDownloadsDir == "" {
		http.Error(w, "cli downloads directory not configured", http.StatusNotFound)
		return
	}

	platform := strings.ToLower(r.URL.Query().Get("platform"))
	arch := r.URL.Query().Get("arch")

	var extension string
	if platform == "windows" {
		extension = "zip"
	} else {
		extension = "tgz"
	}

	http.ServeFile(w, r, filepath.Join(s.cliDownloadsDir, "fly-"+platform+"-"+arch+"."+extension))
}
