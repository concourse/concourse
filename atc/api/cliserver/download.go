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

	switch platform {
	case "windows":
		extension = ".exe"
	case "darwin", "linux":
		extension = ""
	default:
		http.Error(w, "invalid platform", http.StatusBadRequest)
		return
	}

	switch arch {
	case "amd64":
	case "i386":
		http.Error(w, "too few bits", http.StatusPaymentRequired)
		return
	default:
		http.Error(w, "invalid architecture", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=fly"+extension)

	http.ServeFile(w, r, filepath.Join(s.cliDownloadsDir, "fly_"+platform+"_"+arch+extension))
}
