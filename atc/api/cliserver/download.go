package cliserver

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var whitelist = regexp.MustCompile(`^[a-z0-9]+$`)

func (s *Server) Download(w http.ResponseWriter, r *http.Request) {
	if s.cliDownloadsDir == "" {
		http.Error(w, "cli downloads directory not configured", http.StatusNotFound)
		return
	}

	platform := strings.ToLower(r.URL.Query().Get("platform"))
	arch := r.URL.Query().Get("arch")

	if !whitelist.MatchString(platform) || !whitelist.MatchString(arch) {
		// prevent attempts at accessing arbitrary paths
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var flyFile, archiveExtension string
	if platform == "windows" {
		flyFile = "fly.exe"
		archiveExtension = "zip"
	} else {
		flyFile = "fly"
		archiveExtension = "tgz"
	}

	archive := filepath.Join(s.cliDownloadsDir, "fly-"+platform+"-"+arch+"."+archiveExtension)

	switch archiveExtension {
	case "zip":
		reader, err := zip.OpenReader(archive)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to open zip archive: %s", err), http.StatusInternalServerError)
			return
		}

		defer reader.Close()

		for _, f := range reader.File {
			if f.Name != flyFile {
				continue
			}

			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", f.UncompressedSize64))
			w.Header().Set("Content-Disposition", "attachment; filename="+flyFile)
			w.Header().Set("Last-Modified", f.ModTime().UTC().Format(http.TimeFormat))

			stream, err := f.Open()
			if err != nil {
				http.Error(w, fmt.Sprintf("failed to open fly file in zip: %s", err), http.StatusInternalServerError)
				return
			}

			defer stream.Close()

			_, _ = io.Copy(w, stream)

			return
		}

	case "tgz":
		tgz, err := os.Open(archive)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to open tgz archive: %s", err), http.StatusInternalServerError)
			return
		}

		defer tgz.Close()

		gzReader, err := gzip.NewReader(tgz)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to decompress tgz archive: %s", err), http.StatusInternalServerError)
			return
		}

		defer gzReader.Close()

		tarReader := tar.NewReader(gzReader)

		for {
			f, err := tarReader.Next()
			if err != nil {
				if err == io.EOF {
					break
				}

				http.Error(w, fmt.Sprintf("failed to seek next header in tgz archive: %s", err), http.StatusInternalServerError)
				return
			}

			if f.Name != flyFile {
				continue
			}

			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", f.Size))
			w.Header().Set("Content-Disposition", "attachment; filename="+flyFile)
			w.Header().Set("Last-Modified", f.ModTime.UTC().Format(http.TimeFormat))

			_, _ = io.Copy(w, tarReader)

			return
		}
	}

	// normally we return out of the handler upon streaming the file from the
	// archive; if we got here it's because the archive, for whatever reason,
	// didn't have the fly binary
	http.Error(w, "fly executable not found in archive", http.StatusInternalServerError)
}
