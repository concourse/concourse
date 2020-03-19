package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/google/go-github/github"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"golang.org/x/oauth2"

	"github.com/vito/booklit"
	"github.com/vito/booklit/ast"
	"github.com/vito/booklit/stages"
)

func init() {
	booklit.RegisterPlugin("concourse-docs", NewPlugin)
}

type Plugin struct {
	section *booklit.Section

	definitionContext []string
	noteIdx           int

	schemaContext    []string
	schemaGroupTitle booklit.Content
}

func NewPlugin(section *booklit.Section) booklit.Plugin {
	return &Plugin{
		section: section,
	}
}

func (p Plugin) Dim(content booklit.Content) booklit.Content {
	return booklit.Styled{
		Style:   "dim",
		Content: content,
	}
}

func (p Plugin) FontAwesome(class string) booklit.Content {
	return booklit.Styled{
		Style:   "font-awesome",
		Content: booklit.String(class),
	}
}

func (p Plugin) Button(content booklit.Content) booklit.Content {
	return booklit.Styled{
		Style:   "button",
		Content: content,
	}
}

func (p Plugin) Codeblock(language string, code booklit.Content) booklit.Content {
	return booklit.Styled{
		Style:   "code-block",
		Content: code,
		Partials: booklit.Partials{
			"Language": booklit.String(language),
		},
	}
}

func (p Plugin) InlineHeader(content booklit.Content) booklit.Content {
	return booklit.Styled{
		Style:   "inline-header",
		Content: content,
		Block:   true,
	}
}

func (p Plugin) SplashIntro(intro, downloads booklit.Content) {
	p.section.SetPartial(
		"Splash",
		booklit.Styled{
			Style: "splash-intro",
			Block: true,

			Content: intro,

			Partials: booklit.Partials{
				"Downloads": downloads,
			},
		},
	)
}

func (p Plugin) QuickStart(content booklit.Content) booklit.Content {
	return booklit.Styled{
		Style:   "quick-start",
		Block:   true,
		Content: content,
	}
}

func (p *Plugin) Schema(name string, contentNode ast.Node) (booklit.Content, error) {
	old := p.schemaContext
	p.schemaContext = []string{name}
	defer func() {
		p.schemaContext = old
	}()

	tagName := "schema." + name

	p.section.SetTagAnchored(
		tagName,
		booklit.Sequence{
			booklit.Styled{
				Style: booklit.StyleBold,
				Content: booklit.Styled{
					Style:   booklit.StyleVerbatim,
					Content: booklit.String(name),
				},
			},
			booklit.String(" schema"),
		},
		booklit.Empty,
		tagName,
	)

	stage := &stages.Evaluate{
		Section: p.section,
	}

	err := contentNode.Visit(stage)
	if err != nil {
		return nil, err
	}

	content := stage.Result

	return booklit.Styled{
		Style:   "schema",
		Block:   true,
		Content: content,
		Partials: booklit.Partials{
			"Name":   booklit.String(name),
			"Anchor": booklit.String(tagName),
		},
	}, nil
}

func (p *Plugin) SchemaGroup(title booklit.Content, tagName string, contentNode ast.Node) (booklit.Content, error) {
	p.pushSchema(tagName)
	defer p.popSchema()

	p.schemaGroupTitle = title
	defer func() {
		p.schemaGroupTitle = nil
	}()

	stage := &stages.Evaluate{
		Section: p.section,
	}

	err := contentNode.Visit(stage)
	if err != nil {
		return nil, err
	}

	content := stage.Result

	return booklit.Styled{
		Style:   "schema-group",
		Block:   true,
		Content: content,
		Partials: booklit.Partials{
			"Title":   title,
			"TagName": booklit.String(tagName),
			"Target": booklit.Target{
				TagName: tagName,
				Title:   title,
				Content: content,
			},
		},
	}, nil
}

func (p *Plugin) RequiredAttribute(attribute string, type_ string, contentNode ast.Node) (booklit.Content, error) {
	return p.schemaAttribute(
		attribute,
		type_,
		contentNode,
		booklit.Partials{
			"Required": booklit.String("true"),
		},
	)
}

func (p *Plugin) OptionalAttribute(attribute string, type_ string, contentNode ast.Node) (booklit.Content, error) {
	return p.schemaAttribute(
		attribute,
		type_,
		contentNode,
		booklit.Partials{
			"Optional": booklit.String("true"),
		},
	)
}

func (p *Plugin) ExampleToggle(title, content booklit.Content) booklit.Content {
	uniq := strings.Join(p.schemaContext, ".") + title.String()
	hash := sha1.Sum([]byte(uniq))

	return booklit.Styled{
		Style:   "example-toggle",
		Block:   true,
		Content: content,
		Partials: booklit.Partials{
			"Title": title,
			"For":   booklit.String(fmt.Sprintf("%x", hash)),
		},
	}
}

func (p *Plugin) OneOf(options ...booklit.Content) booklit.Content {
	return booklit.Styled{
		Style:   "schema-one-of",
		Block:   true,
		Content: booklit.Sequence(options),
	}
}

func (p *Plugin) SchemaAttribute(attribute string, type_ string, contentNode ast.Node) (booklit.Content, error) {
	return p.schemaAttribute(
		attribute,
		type_,
		contentNode,
		booklit.Partials{},
	)
}

func (p *Plugin) schemaAttribute(attribute string, type_ string, contentNode ast.Node, partials booklit.Partials) (booklit.Content, error) {
	p.pushSchema(attribute)
	defer p.popSchema()

	tagName := "schema." + strings.Join(p.schemaContext, ".")

	stage := &stages.Evaluate{
		Section: p.section,
	}

	err := contentNode.Visit(stage)
	if err != nil {
		return nil, err
	}

	content := stage.Result

	var display booklit.Content
	if p.schemaGroupTitle != nil {
		display = booklit.Sequence{
			p.schemaGroupTitle,
			booklit.String(" "),
			booklit.Styled{
				Style: booklit.StyleBold,
				Content: booklit.Styled{
					Style:   booklit.StyleVerbatim,
					Content: booklit.String(attribute),
				},
			},
		}
	} else {
		attr := booklit.Sequence{}
		for _, con := range p.schemaContext {
			attr = append(attr, booklit.String(con))
		}

		display = booklit.Styled{
			Style:   "schema-attribute-name",
			Content: attr,
		}
	}

	targets := booklit.Sequence{
		booklit.Target{
			TagName: tagName,
			Title:   display,
			Content: content,
		},
	}

	partials["TagName"] = booklit.String(tagName)
	partials["Targets"] = targets
	partials["Attribute"] = booklit.String(attribute)
	partials["Type"] = autoReferenceType(type_)

	return NoIndex{
		booklit.Styled{
			Style:    "schema-attribute",
			Block:    true,
			Content:  content,
			Partials: partials,
		},
	}, nil
}

func (p *Plugin) pushSchema(attribute string) {
	p.schemaContext = append(p.schemaContext, attribute)
}

func (p *Plugin) popSchema() {
	p.schemaContext = p.schemaContext[0 : len(p.schemaContext)-1]
}

func (p *Plugin) DownloadLinks() (booklit.Content, error) {
	ctx := context.Background()

	var tc *http.Client
	var githubToken = os.Getenv("GITHUB_TOKEN")

	if githubToken != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: githubToken},
		)

		tc = oauth2.NewClient(ctx, ts)
	}

	client := github.NewClient(tc)

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

func (p Plugin) SplashExample(title booklit.Content, content booklit.Content, example booklit.Content) booklit.Content {
	return booklit.Styled{
		Style: "splash-example",
		Block: true,

		Content: content,

		Partials: booklit.Partials{
			"Title":   title,
			"Example": example,
		},
	}
}

func (p Plugin) ExamplePipeline() booklit.Content {
	return booklit.Styled{
		Style:   "example-pipeline",
		Block:   true,
		Content: booklit.Empty,
	}
}

func (p Plugin) TitledCodeblock(title booklit.Content, language string, code booklit.Content) (booklit.Content, error) {
	return booklit.Styled{
		Style: "titled-codeblock",
		Block: true,

		Content: p.Codeblock(language, code),

		Partials: booklit.Partials{
			"Title": booklit.Styled{
				Style:   booklit.StyleVerbatim,
				Content: title,
			},
		},
	}, nil
}

func (p Plugin) Warn(content booklit.Content) booklit.Content {
	return booklit.Styled{
		Style:   "warning",
		Content: content,
	}
}

func (p Plugin) BetterTable(content booklit.Content) (booklit.Content, error) {
	switch v := content.(type) {
	case booklit.Paragraph:
		table := booklit.Table{}

		for _, row := range v {
			list, ok := row.(booklit.List)
			if !ok {
				return nil, fmt.Errorf("table row is not a list: %s", row)
			}

			table.Rows = append(table.Rows, list.Items)
		}

		return table, nil
	default:
		return nil, fmt.Errorf("invalid table content: %T", content)
	}
}

func (p Plugin) Frame(src booklit.Content, optionalHeight ...string) booklit.Content {
	height := "300px"
	if len(optionalHeight) > 0 {
		height = optionalHeight[0]
	}

	return booklit.Styled{
		Style:   "frame",
		Content: booklit.Empty,
		Partials: booklit.Partials{
			"URL":    src,
			"Height": booklit.String(height),
		},
	}
}

func (p *Plugin) DefineAttribute(attribute string, contentNode ast.Node, tags ...string) (booklit.Content, error) {
	attrSplit := strings.SplitN(attribute, ":", 2)

	attrName := attrSplit[0]
	if len(tags) == 0 {
		tags = []string{attrName}
	}

	oldCtx := p.definitionContext
	p.definitionContext = append(p.definitionContext, attrName)

	stage := &stages.Evaluate{
		Section: p.section,
	}

	err := contentNode.Visit(stage)
	if err != nil {
		return nil, err
	}

	content := stage.Result

	display := booklit.Styled{
		Style: booklit.StyleBold,
		Content: booklit.Styled{
			Style:   booklit.StyleVerbatim,
			Content: booklit.String(strings.Join(p.definitionContext, ".")),
		},
	}

	p.definitionContext = oldCtx

	targets := booklit.Sequence{}
	for _, t := range tags {
		targets = append(targets, booklit.Target{
			TagName: t,
			Title:   display,
			Content: content,
		})
	}

	return NoIndex{
		booklit.Styled{
			Style:   "definition",
			Content: content,
			Partials: booklit.Partials{
				"Targets": targets,
				"Thumb": booklit.Styled{
					Style: booklit.StyleVerbatim,
					Content: booklit.Preformatted{
						booklit.Sequence{
							&booklit.Reference{
								TagName: tags[0],
								Content: booklit.Styled{
									Style:   booklit.StyleBold,
									Content: booklit.String(attrName),
								},
							},
							booklit.String(":" + attrSplit[1]),
						},
					},
				},
			},
		},
	}, nil
}

func (p Plugin) DefineMetric(metric string, content booklit.Content) booklit.Content {
	return NoIndex{
		booklit.Styled{
			Style:   "definition",
			Content: content,
			Partials: booklit.Partials{
				"Targets": booklit.Target{
					TagName: metric,
					Title:   booklit.String(metric),
					Content: content,
				},
				"Thumb": booklit.Styled{
					Style:   booklit.StyleVerbatim,
					Content: booklit.Preformatted{booklit.String(metric)},
				},
			},
		},
	}
}

func (p Plugin) DefineTable(table string, content booklit.Content) booklit.Content {
	tagName := table + "-table"

	return NoIndex{
		booklit.Styled{
			Style:   "definition",
			Content: content,
			Partials: booklit.Partials{
				"Targets": booklit.Target{
					TagName: tagName,
					Title: booklit.Styled{
						Style: booklit.StyleVerbatim,
						Content: booklit.Styled{
							Style:   booklit.StyleBold,
							Content: booklit.String(table),
						},
					},
					Content: content,
				},
				"Thumb": booklit.Styled{
					Style: booklit.StyleVerbatim,
					Content: booklit.Preformatted{
						&booklit.Reference{
							TagName: tagName,
							Content: booklit.Styled{
								Style:   booklit.StyleBold,
								Content: booklit.String(table),
							},
						},
					},
				},
			},
		},
	}
}

func (p Plugin) ReferenceColumn(table string, column string) booklit.Content {
	return &booklit.Reference{
		TagName: table + "-table",
		Content: booklit.Styled{
			Style: booklit.StyleVerbatim,
			Content: booklit.Sequence{
				booklit.Styled{
					Style:   booklit.StyleBold,
					Content: booklit.String(table),
				},
				booklit.String(" (" + column + ")"),
			},
		},
	}
}

func (p Plugin) Boshprop(job string, target string) booklit.Content {
	return booklit.Link{
		Target: fmt.Sprintf("https://bosh.io/jobs/%s?source=github.com/concourse/concourse-bosh-release#p=%s", job, target),
		Content: booklit.Styled{
			Style:   booklit.StyleVerbatim,
			Content: booklit.String(target),
		},
	}
}

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

func (p Plugin) Resource(name string) booklit.Content {
	return booklit.Link{
		Target: fmt.Sprintf("https://github.com/concourse/%s-resource", name),
		Content: booklit.Sequence{
			booklit.Styled{
				Style:   booklit.StyleVerbatim,
				Content: booklit.String(name),
			},
			booklit.String(" resource"),
		},
	}
}

func (p Plugin) Diagram(path string, width string) booklit.Content {
	return booklit.Styled{
		Style:   "diagram",
		Block:   true,
		Content: booklit.String(path),
		Partials: booklit.Partials{
			"Width": booklit.String(width),
		},
	}
}

func (p Plugin) LiterateSegment(parasAndFinalCode ...booklit.Content) (booklit.Content, error) {
	if len(parasAndFinalCode) == 0 {
		return nil, fmt.Errorf("no paragraphs or code given")
	}

	paras := parasAndFinalCode[0 : len(parasAndFinalCode)-1]
	code := parasAndFinalCode[len(parasAndFinalCode)-1]

	if len(paras) == 0 {
		paras = []booklit.Content{code}
		code = booklit.Empty
	}

	return booklit.Styled{
		Style:   "literate-segment",
		Content: booklit.Sequence(paras),
		Partials: booklit.Partials{
			"Code": code,
		},
	}, nil
}

func (p Plugin) SideBySide(left, right booklit.Content) booklit.Content {
	return booklit.Styled{
		Style:   "side-by-side",
		Content: booklit.Sequence{left, right},
		Block:   true,
	}
}

func (p Plugin) IncludeTemplate(name string) booklit.Content {
	return booklit.Styled{
		Style:   booklit.Style(name),
		Content: booklit.Empty,
		Block:   true,
	}
}

func (p *Plugin) Note(commaSeparatedTags string, content booklit.Content) booklit.Content {
	tags := strings.Split(commaSeparatedTags, ",")

	p.noteIdx++

	idx := p.noteIdx
	targetTag := fmt.Sprintf("%s-note-%d", p.section.PrimaryTag.Name, idx)

	tagNotes := []booklit.Content{}
	for _, t := range tags {
		tagNotes = append(tagNotes, booklit.Styled{
			Style:   "release-note-tag",
			Content: booklit.String(t),
		})
	}

	return booklit.Styled{
		Style:   "release-note",
		Content: content,
		Partials: booklit.Partials{
			"Tags":   booklit.Sequence(tagNotes),
			"Target": booklit.String(targetTag),
		},
	}
}

func (p Plugin) RightSide(title, content booklit.Content) {
	wrappedContent := booklit.Styled{
		Style:   "sidebar-right",
		Content: content,
		Partials: booklit.Partials{
			"Title": title,
		},
	}

	p.section.SetPartial("RightSide", wrappedContent)
}

func (p Plugin) Example(title, content booklit.Content) booklit.Content {
	return booklit.Styled{
		Style:   "example",
		Block:   true,
		Content: content,
		Partials: booklit.Partials{
			"Title": title,
		},
	}
}

func (p Plugin) TrademarkGuidelines(content ...booklit.Content) booklit.Content {
	return booklit.Styled{
		Style: "trademark-guidelines",
		Block: true,
		Content: booklit.List{
			Items: content,
		},
	}
}

var archivedBinariesVersion = semver.MustParse("5.0.0")
var flyBinariesVersion = semver.MustParse("2.2.0")

func (p Plugin) ReleaseVersion(version string) error {
	p.section.Style = "release"

	p.section.SetTitle(booklit.String("v" + version))

	p.section.SetPartial("Version", booklit.String(version))

	v, err := semver.Parse(version)
	if err != nil {
		return err
	}

	downloadURL := "https://github.com/concourse/concourse/releases/download/v" + version

	if v.GTE(archivedBinariesVersion) {
		p.section.SetPartial("ConcourseLinuxURL", booklit.String(downloadURL+"/concourse-"+version+"-linux-amd64.tgz"))
		p.section.SetPartial("ConcourseWindowsURL", booklit.String(downloadURL+"/concourse-"+version+"-windows-amd64.zip"))
		p.section.SetPartial("ConcourseDarwinURL", booklit.String(downloadURL+"/concourse-"+version+"-darwin-amd64.tgz"))

		p.section.SetPartial("HasFlyBinaries", booklit.Empty)
		p.section.SetPartial("FlyLinuxURL", booklit.String(downloadURL+"/fly-"+version+"-linux-amd64.tgz"))
		p.section.SetPartial("FlyWindowsURL", booklit.String(downloadURL+"/fly-"+version+"-windows-amd64.zip"))
		p.section.SetPartial("FlyDarwinURL", booklit.String(downloadURL+"/fly-"+version+"-darwin-amd64.tgz"))
	} else {
		p.section.SetPartial("ConcourseLinuxURL", booklit.String(downloadURL+"/concourse_linux_amd64"))
		p.section.SetPartial("ConcourseWindowsURL", booklit.String(downloadURL+"/concourse_windows_amd64.exe"))
		p.section.SetPartial("ConcourseDarwinURL", booklit.String(downloadURL+"/concourse_darwin_amd64"))

		if v.GTE(flyBinariesVersion) {
			p.section.SetPartial("HasFlyBinaries", booklit.Empty)
			p.section.SetPartial("FlyLinuxURL", booklit.String(downloadURL+"/fly_linux_amd64"))
			p.section.SetPartial("FlyWindowsURL", booklit.String(downloadURL+"/fly_windows_amd64.exe"))
			p.section.SetPartial("FlyDarwinURL", booklit.String(downloadURL+"/fly_darwin_amd64"))
		}
	}

	return nil
}

func (p Plugin) ReleaseGardenLinuxVersion(version string) {
	p.section.SetPartial("GardenLinuxVersion", booklit.String(version))
}

func (p Plugin) ReleaseGardenRuncVersion(version string) {
	p.section.SetPartial("GardenRuncVersion", booklit.String(version))
}

func (p Plugin) ReleaseDate(date string) error {
	t, err := time.Parse("2006-1-2", date)
	if err != nil {
		return err
	}

	p.section.SetPartial("ReleaseDate", booklit.Styled{
		Style:   "release-date",
		Content: booklit.String(t.Format("January 2, 2006")),
	})

	return nil
}

func (p Plugin) PromethusDocs(sample string) (booklit.Content, error) {
	parser := new(expfmt.TextParser)

	mfs, err := parser.TextToMetricFamilies(bytes.NewBufferString(sample))
	if err != nil {
		return nil, fmt.Errorf("failed to parse prometheus sample: %w", err)
	}

	type metric struct {
		name   string
		family *dto.MetricFamily
	}

	metrics := booklit.Sequence{}

	sorted := []metric{}
	for metricName, family := range mfs {
		sorted = append(sorted, metric{
			name:   metricName,
			family: family,
		})
	}

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].name < sorted[j].name
	})

	for _, metric := range sorted {
		metricName := metric.name
		family := metric.family

		distinctLabels := map[string]bool{}

		labels := booklit.Sequence{}
		for _, metric := range family.GetMetric() {
			for _, label := range metric.GetLabel() {
				labelName := label.GetName()

				if distinctLabels[labelName] {
					continue
				}

				distinctLabels[labelName] = true

				labels = append(labels, booklit.String(labelName))
			}
		}

		metricType := strings.ToLower(family.GetType().String())

		metrics = append(metrics, booklit.Styled{
			Style:   "prometheus-metric",
			Content: booklit.String(family.GetHelp()),
			Partials: booklit.Partials{
				"Name":   booklit.String(metricName),
				"Type":   booklit.String(metricType),
				"Labels": labels,
			},
		})
	}

	return metrics, nil
}

type NoIndex struct {
	booklit.Content
}

func (NoIndex) String() string {
	return ""
}

func autoReferenceType(type_ string) booklit.Content {
	if strings.HasPrefix(type_, "[") && strings.HasSuffix(type_, "]") {
		subType := strings.TrimPrefix(strings.TrimSuffix(type_, "]"), "[")
		return booklit.Sequence{
			booklit.String("["),
			autoReferenceType(subType),
			booklit.String("]"),
		}
	}

	if strings.HasPrefix(type_, "{") && strings.HasSuffix(type_, "}") {
		subType := strings.TrimPrefix(strings.TrimSuffix(type_, "}"), "{")
		return booklit.Sequence{
			booklit.String("{"),
			autoReferenceType(subType),
			booklit.String("}"),
		}
	}

	for _, punc := range []string{" | ", ": ", ", "} {
		if strings.Contains(type_, punc) {
			ors := strings.Split(type_, punc)

			seq := booklit.Sequence{}
			for i, t := range ors {
				seq = append(seq, autoReferenceType(t))

				if i+1 < len(ors) {
					seq = append(seq, booklit.String(punc))
				}
			}

			return seq
		}
	}

	if strings.HasPrefix(type_, "`") && strings.HasSuffix(type_, "`") {
		scalar := strings.TrimPrefix(strings.TrimSuffix(type_, "`"), "`")
		return booklit.Styled{
			Style:   "schema-scalar",
			Content: booklit.String(scalar),
		}
	}

	return &booklit.Reference{
		TagName: "schema." + type_,
		Content: booklit.Styled{
			Style:   booklit.StyleBold,
			Content: booklit.String(type_),
		},
	}
}
