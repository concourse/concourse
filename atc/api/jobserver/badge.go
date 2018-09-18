package jobserver

import (
	"bytes"
	"fmt"
	"net/http"
	"text/template"

	"github.com/concourse/atc/db"
)

var (
	badgePassing = Badge{width: 88, fillColor: `#44cc11`, status: `passing`}
	badgeFailing = Badge{width: 80, fillColor: `#e05d44`, status: `failing`}
	badgeUnknown = Badge{width: 98, fillColor: `#9f9f9f`, status: `unknown`}
	badgeAborted = Badge{width: 90, fillColor: `#8f4b2d`, status: `aborted`}
	badgeErrored = Badge{width: 88, fillColor: `#fe7d37`, status: `errored`}
)

type Badge struct {
	width     int
	fillColor string
	status    string
}

func (b *Badge) statusWidth() int {
	return b.width - 37
}

func (b *Badge) statusTextWidth() string {
	return fmt.Sprintf("%.1f", float64(b.width)/2+17.5)
}

func (b *Badge) String() string {
	tmpl, err := template.New("Badge").Parse(badgeTemplate)
	if err != nil {
		panic(err)
	}

	buffer := &bytes.Buffer{}

	_ = tmpl.Execute(buffer, badgeTemplateConfig{
		Width:           b.width,
		FillColor:       b.fillColor,
		Status:          b.status,
		StatusWidth:     b.statusWidth(),
		StatusTextWidth: b.statusTextWidth(),
	})

	return buffer.String()
}

func BadgeForBuild(build db.Build) *Badge {
	switch {
	case build == nil:
		return &badgeUnknown
	case build.Status() == db.BuildStatusSucceeded:
		return &badgePassing
	case build.Status() == db.BuildStatusFailed:
		return &badgeFailing
	case build.Status() == db.BuildStatusAborted:
		return &badgeAborted
	case build.Status() == db.BuildStatusErrored:
		return &badgeErrored
	default:
		return &badgeUnknown
	}
}

const badgeTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="{{ .Width }}" height="20">
   <linearGradient id="b" x2="0" y2="100%">
      <stop offset="0" stop-color="#bbb" stop-opacity=".1" />
      <stop offset="1" stop-opacity=".1" />
   </linearGradient>
   <mask id="a">
      <rect width="{{ .Width }}" height="20" rx="3" fill="#fff" />
   </mask>
   <g mask="url(#a)">
      <path fill="#555" d="M0 0h37v20H0z" />
      <path fill="{{ .FillColor }}" d="M37 0h{{ .StatusWidth }}v20H37z" />
      <path fill="url(#b)" d="M0 0h{{ .Width }}v20H0z" />
   </g>
   <g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="11">
      <text x="18.5" y="15" fill="#010101" fill-opacity=".3">build</text>
      <text x="18.5" y="14">build</text>
      <text x="{{ .StatusTextWidth }}" y="15" fill="#010101" fill-opacity=".3">{{ .Status }}</text>
      <text x="{{ .StatusTextWidth }}" y="14">{{ .Status }}</text>
   </g>
</svg>`

type badgeTemplateConfig struct {
	Width           int
	StatusWidth     int
	StatusTextWidth string
	Status          string
	FillColor       string
}

func (s *Server) JobBadge(pipeline db.Pipeline) http.Handler {
	logger := s.logger.Session("job-badge")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jobName := r.FormValue(":job_name")

		job, found, err := pipeline.Job(jobName)
		if err != nil {
			logger.Error("error-finding-job", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		build, _, err := job.FinishedAndNextBuild()
		if err != nil {
			logger.Error("could-not-get-job-finished-and-next-build", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-type", "image/svg+xml")

		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Expires", "0")

		w.WriteHeader(http.StatusOK)

		fmt.Fprint(w, BadgeForBuild(build))
	})
}
