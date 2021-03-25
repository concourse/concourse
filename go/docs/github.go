package docs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/blang/semver"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/shurcooL/githubv4"
	"github.com/sirupsen/logrus"
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

	client, ok := p.githubClient(ctx)
	if !ok {
		logrus.Warn("no $GITHUB_TOKEN set; using filler download links")
		return fillerDownloads, nil
	}

	type release struct {
		Name         string
		TagName      *string
		IsDraft      bool
		IsPrerelease bool
		Url          string
		Assets       struct {
			Nodes []struct {
				Name        string
				DownloadUrl string
			}
		} `graphql:"releaseAssets(first: 20)"`
	}

	var releasesQuery struct {
		Repository struct {
			Releases struct {
				Nodes []release
			} `graphql:"releases(first: 10, orderBy: {field: CREATED_AT, direction: DESC})"`
		} `graphql:"repository(owner: \"concourse\", name: \"concourse\")"`
	}

	err := client.Query(ctx, &releasesQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch releases: %w", err)
	}

	releases := releasesQuery.Repository.Releases.Nodes

	var latestRelease release
	var latestVersion semver.Version
	for _, release := range releases {
		if release.IsPrerelease || release.IsDraft {
			continue
		}

		if release.TagName == nil {
			return nil, fmt.Errorf("no tag name for release %v", release)
		}

		version, err := semver.ParseTolerant(*release.TagName)
		if err != nil {
			return nil, err
		}

		if version.GT(latestVersion) {
			latestRelease = release
			latestVersion = version
		}
	}

	if latestRelease.Url == "" {
		return nil, errors.New("no releases found!")
	}

	var linuxURL, darwinURL, windowsURL string
	var flyLinuxURL, flyDarwinURL, flyWindowsURL string
	for _, asset := range latestRelease.Assets.Nodes {
		name := asset.Name

		if strings.Contains(name, "concourse") && strings.Contains(name, "linux") && strings.HasSuffix(name, ".tgz") {
			linuxURL = asset.DownloadUrl
		}

		if strings.Contains(name, "concourse") && strings.Contains(name, "darwin") && strings.HasSuffix(name, ".tgz") {
			darwinURL = asset.DownloadUrl
		}

		if strings.Contains(name, "concourse") && strings.Contains(name, "windows") && strings.HasSuffix(name, ".zip") {
			windowsURL = asset.DownloadUrl
		}

		if strings.Contains(name, "fly") && strings.Contains(name, "linux") && strings.HasSuffix(name, ".tgz") {
			flyLinuxURL = asset.DownloadUrl
		}

		if strings.Contains(name, "fly") && strings.Contains(name, "darwin") && strings.HasSuffix(name, ".tgz") {
			flyDarwinURL = asset.DownloadUrl
		}

		if strings.Contains(name, "fly") && strings.Contains(name, "windows") && strings.HasSuffix(name, ".zip") {
			flyWindowsURL = asset.DownloadUrl
		}
	}

	return booklit.Styled{
		Style: "download-links",
		Block: true,
		Content: booklit.Link{
			Target:  latestRelease.Url,
			Content: booklit.String(latestRelease.Name),
		},
		Partials: booklit.Partials{
			"Version":         booklit.String(latestRelease.Name),
			"ReleaseNotesURL": booklit.String(latestRelease.Url),
			"LinuxURL":        booklit.String(linuxURL),
			"DarwinURL":       booklit.String(darwinURL),
			"WindowsURL":      booklit.String(windowsURL),
			"FlyLinuxURL":     booklit.String(flyLinuxURL),
			"FlyDarwinURL":    booklit.String(flyDarwinURL),
			"FlyWindowsURL":   booklit.String(flyWindowsURL),
		},
	}, nil
}

func (p *Plugin) githubClient(ctx context.Context) (*githubv4.Client, bool) {
	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken == "" {
		return nil, false
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubToken},
	)

	return githubv4.NewClient(oauth2.NewClient(ctx, ts)), true
}

var fillerDownloads = booklit.Styled{
	Style: "download-links",
	Block: true,
	Content: booklit.Link{
		Target:  "https://github.com/concourse/concourse/releases/latest",
		Content: booklit.String("vX.X.X"),
	},
	Partials: booklit.Partials{
		"Version":         booklit.String("vX.X.X"),
		"ReleaseNotesURL": booklit.String("https://github.com/concourse/concourse/releases/latest"),
		"LinuxURL":        booklit.String("https://example.com/#linux"),
		"DarwinURL":       booklit.String("https://example.com/#darwin"),
		"WindowsURL":      booklit.String("https://example.com/#windows"),
		"FlyLinuxURL":     booklit.String("https://example.com/#fly-linux"),
		"FlyDarwinURL":    booklit.String("https://example.com/#fly-darwin"),
		"FlyWindowsURL":   booklit.String("https://example.com/#fly-windows"),
	},
}
