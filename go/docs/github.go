package docs

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/blang/semver"
	"github.com/google/go-github/github"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/vito/booklit"
	"golang.org/x/oauth2"
)

func (p Plugin) Ghuser(user string) booklit.Content {
	return booklit.Styled{
		Style:   "github-user-link",
		Content: booklit.String(user),
	}
}

func (p Plugin) Ghrelease(tag string, optionalRepo ...string) booklit.Content {
	repo := "concourse"
	if len(optionalRepo) > 0 {
		repo = optionalRepo[0]
	}

	return booklit.Styled{
		Style:   "github-release-link",
		Content: booklit.String(tag),
		Partials: booklit.Partials{
			"Owner": booklit.String("concourse"),
			"Repo":  booklit.String(repo),
		},
	}
}

func (p Plugin) Ghpr(number string, optionalRepo ...string) booklit.Content {
	repo := "concourse"
	if len(optionalRepo) > 0 {
		repo = optionalRepo[0]
	}

	return booklit.Styled{
		Style:   "github-pr-link",
		Content: booklit.String(number),
		Partials: booklit.Partials{
			"Owner": booklit.String("concourse"),
			"Repo":  booklit.String(repo),
		},
	}
}

func (p Plugin) Ghissue(number string, optionalRepo ...string) booklit.Content {
	repo := "concourse"
	if len(optionalRepo) > 0 {
		repo = optionalRepo[0]
	}

	return booklit.Styled{
		Style:   "github-issue-link",
		Content: booklit.String(number),
		Partials: booklit.Partials{
			"Owner": booklit.String("concourse"),
			"Repo":  booklit.String(repo),
		},
	}
}

func (p Plugin) Ghlabel(name booklit.Content, colorHex string) (booklit.Content, error) {
	color, err := colorful.Hex(colorHex)
	if err != nil {
		return nil, err
	}

	lightContrastRating := contrast(color, white)
	darkContrastRating := contrast(color, color.BlendRgb(black, 0.8))

	class := "dark-label"
	if lightContrastRating > darkContrastRating {
		class = "light-label"
	}

	return booklit.Styled{
		Style:   "label",
		Content: name,
		Partials: booklit.Partials{
			"Color": booklit.String(colorHex),
			"Class": booklit.String(class),
		},
	}, nil
}

func (p *Plugin) DownloadLinks() (booklit.Content, error) {
	ctx := context.Background()

	client := p.githubClient(ctx)

	var latestRelease *github.RepositoryRelease
	var latestVersion semver.Version
	releases, _, err := client.Repositories.ListReleases(ctx, "concourse", "concourse", nil)
	if err != nil {
		return nil, err
	}
	if len(releases) == 0 {
		return nil, errors.New("no releases found!")
	}
	for _, release := range releases {
		if release.GetPrerelease() {
			continue
		}

		if release.TagName == nil {
			return nil, fmt.Errorf("no tag name for release %v", release)
		}
		version, err := semver.ParseTolerant(*release.TagName)
		if err != nil {
			return nil, err
		}
		if latestRelease == nil || version.GT(latestVersion) {
			latestRelease = release
			latestVersion = version
		}
	}

	var linuxURL, darwinURL, windowsURL string
	var flyLinuxURL, flyDarwinURL, flyWindowsURL string
	for _, asset := range latestRelease.Assets {
		name := asset.GetName()

		if strings.Contains(name, "concourse") && strings.Contains(name, "linux") && strings.HasSuffix(name, ".tgz") {
			linuxURL = asset.GetBrowserDownloadURL()
		}

		if strings.Contains(name, "concourse") && strings.Contains(name, "darwin") && strings.HasSuffix(name, ".tgz") {
			darwinURL = asset.GetBrowserDownloadURL()
		}

		if strings.Contains(name, "concourse") && strings.Contains(name, "windows") && strings.HasSuffix(name, ".zip") {
			windowsURL = asset.GetBrowserDownloadURL()
		}

		if strings.Contains(name, "fly") && strings.Contains(name, "linux") && strings.HasSuffix(name, ".tgz") {
			flyLinuxURL = asset.GetBrowserDownloadURL()
		}

		if strings.Contains(name, "fly") && strings.Contains(name, "darwin") && strings.HasSuffix(name, ".tgz") {
			flyDarwinURL = asset.GetBrowserDownloadURL()
		}

		if strings.Contains(name, "fly") && strings.Contains(name, "windows") && strings.HasSuffix(name, ".zip") {
			flyWindowsURL = asset.GetBrowserDownloadURL()
		}
	}

	return booklit.Styled{
		Style: "download-links",
		Block: true,
		Content: booklit.Link{
			Target:  latestRelease.GetHTMLURL(),
			Content: booklit.String(latestRelease.GetName()),
		},
		Partials: booklit.Partials{
			"Version":         booklit.String(latestRelease.GetName()),
			"ReleaseNotesURL": booklit.String(latestRelease.GetHTMLURL()),
			"LinuxURL":        booklit.String(linuxURL),
			"DarwinURL":       booklit.String(darwinURL),
			"WindowsURL":      booklit.String(windowsURL),
			"FlyLinuxURL":     booklit.String(flyLinuxURL),
			"FlyDarwinURL":    booklit.String(flyDarwinURL),
			"FlyWindowsURL":   booklit.String(flyWindowsURL),
		},
	}, nil
}

func (p *Plugin) githubClient(ctx context.Context) *github.Client {
	var tc *http.Client
	var githubToken = os.Getenv("GITHUB_TOKEN")

	if githubToken != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: githubToken},
		)

		tc = oauth2.NewClient(ctx, ts)
	}

	return github.NewClient(tc)
}
