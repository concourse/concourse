package jobserver

import (
	"bytes"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"text/template"

	"github.com/concourse/concourse/atc/db"
)

var (
	badgePassing = Badge{Width: 88, FillColor: `#44cc11`, Status: `passing`, Title: `build`}
	badgeFailing = Badge{Width: 80, FillColor: `#e05d44`, Status: `failing`, Title: `build`}
	badgeUnknown = Badge{Width: 98, FillColor: `#9f9f9f`, Status: `unknown`, Title: `build`}
	badgeAborted = Badge{Width: 90, FillColor: `#8f4b2d`, Status: `aborted`, Title: `build`}
	badgeErrored = Badge{Width: 88, FillColor: `#fe7d37`, Status: `errored`, Title: `build`}
)

type Badge struct {
	Width       int
	FillColor   string
	Status      string
	Title       string
	customTitle bool
}

func (b *Badge) TitleWidth() int {
	if !b.customTitle {
		return 37
	}

	titleLen := len(b.Title)
	charWidth := titleLen * 6

	var padding int
	if titleLen <= 8 {
		padding = 12
	} else if titleLen <= 15 {
		padding = 6
	} else {
		padding = 0
	}

	return charWidth + padding
}

func (b *Badge) StatusWidth() int {
	return b.Width - b.TitleWidth()
}

// TitleTextX returns X coordinate for centering title text
func (b *Badge) TitleTextX() string {
	if !b.customTitle {
		return "18.5"
	}
	return fmt.Sprintf("%.1f", float64(b.TitleWidth())/2)
}

// StatusTextX returns X coordinate for centering status text
func (b *Badge) StatusTextX() string {
	if !b.customTitle {
		// Preserve original formula for default badges
		return fmt.Sprintf("%.1f", float64(b.Width)/2+17.5)
	}
	return fmt.Sprintf("%.1f", float64(b.TitleWidth())+float64(b.StatusWidth())/2)
}

func (b *Badge) String() string {
	tmpl, err := template.New("Badge").Parse(badgeTemplate)
	if err != nil {
		panic(err)
	}

	buffer := &bytes.Buffer{}

	_ = tmpl.Execute(buffer, &b)

	return buffer.String()
}

func (b *Badge) EnrichFromQuery(params url.Values) {
	if title := params.Get("title"); title != "" {
		originalStatusWidth := b.Width - 37

		b.Title = html.EscapeString(title)
		b.customTitle = true

		// New total width: custom title width + original status width
		b.Width = b.TitleWidth() + originalStatusWidth
	}
}

func BadgeForBuild(build db.Build) Badge {
	switch {
	case build == nil:
		return badgeUnknown
	case build.Status() == db.BuildStatusSucceeded:
		return badgePassing
	case build.Status() == db.BuildStatusFailed:
		return badgeFailing
	case build.Status() == db.BuildStatusAborted:
		return badgeAborted
	case build.Status() == db.BuildStatusErrored:
		return badgeErrored
	default:
		return badgeUnknown
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
      <path fill="#555" d="M0 0h{{ .TitleWidth }}v20H0z" />
      <path fill="{{ .FillColor }}" d="M{{ .TitleWidth }} 0h{{ .StatusWidth }}v20H{{ .TitleWidth }}z" />
      <path fill="url(#b)" d="M0 0h{{ .Width }}v20H0z" />
   </g>
   <g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="11">
      <text x="{{ .TitleTextX }}" y="15" fill="#010101" fill-opacity=".3">{{ .Title }}</text>
      <text x="{{ .TitleTextX }}" y="14">{{ .Title }}</text>
      <text x="{{ .StatusTextX }}" y="15" fill="#010101" fill-opacity=".3">{{ .Status }}</text>
      <text x="{{ .StatusTextX }}" y="14">{{ .Status }}</text>
   </g>
</svg>`

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

		badge := BadgeForBuild(build)
		badge.EnrichFromQuery(r.URL.Query())
		fmt.Fprint(w, &badge)
	})
}
