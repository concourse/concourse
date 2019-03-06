package docs

import (
	"fmt"
	"strings"
	"time"

	_ "github.com/concourse/docs/go/chromastyle"

	"github.com/vito/booklit"
	"github.com/vito/booklit/ast"
	"github.com/vito/booklit/chroma"
	"github.com/vito/booklit/stages"
)

func init() {
	booklit.RegisterPlugin("concourse-docs", NewPlugin)
}

type Plugin struct {
	section *booklit.Section
	chroma  chroma.Plugin

	definitionContext []string
}

func NewPlugin(section *booklit.Section) booklit.Plugin {
	return &Plugin{
		section: section,
		chroma:  chroma.NewPlugin(section).(chroma.Plugin),
	}
}

func (p Plugin) FontAwesome(class string) booklit.Content {
	return booklit.Styled{
		Style:   "font-awesome",
		Content: booklit.String(class),
	}
}

func (p Plugin) Codeblock(language string, code booklit.Content) (booklit.Content, error) {
	return p.chroma.Syntax(language, code, "concourseci")
}

func (p Plugin) InlineHeader(content booklit.Content) booklit.Content {
	return booklit.Styled{
		Style:   "inline-header",
		Content: content,
	}
}

func (p Plugin) SplashIntro(intro, blurb booklit.Content) booklit.Content {
	return booklit.Styled{
		Style: "splash-intro",
		Block: true,

		Content: blurb,

		Partials: booklit.Partials{
			"Intro": intro,
		},
	}
}

func (p Plugin) QuickStart(content booklit.Content) booklit.Content {
	return booklit.Styled{
		Style:   "quick-start",
		Block:   true,
		Content: content,
	}
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
	codeblock, err := p.Codeblock(language, code)
	if err != nil {
		return nil, err
	}

	return booklit.Styled{
		Style: "titled-codeblock",
		Block: true,

		Content: codeblock,

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

func (p Plugin) Frame(src booklit.Content) booklit.Content {
	return booklit.Styled{
		Style:   "frame",
		Content: booklit.Empty,
		Partials: booklit.Partials{
			"Src": src,
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
		Style: booklit.StyleVerbatim,
		Content: booklit.Styled{
			Style:   booklit.StyleBold,
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
		Target: fmt.Sprintf("http://bosh.io/jobs/%s?source=github.com/concourse/concourse-bosh-release#p=%s", job, target),
		Content: booklit.Styled{
			Style:   booklit.StyleVerbatim,
			Content: booklit.String(target),
		},
	}
}

func (p Plugin) Ghuser(user string) booklit.Content {
	return booklit.Link{
		Target: fmt.Sprintf("http://github.com/%s", user),
		Content: booklit.Styled{
			Style:   booklit.StyleBold,
			Content: booklit.String("@" + user),
		},
	}
}

func (p Plugin) Ghpr(repo string, number string) booklit.Content {
	return booklit.Link{
		Target: fmt.Sprintf("http://github.com/concourse/%s/pull/%s", repo, number),
		Content: booklit.Styled{
			Style:   booklit.StyleBold,
			Content: booklit.String(repo + " #" + number),
		},
	}
}

func (p Plugin) Ghissue(number string, optionalRepo ...string) booklit.Content {
	repo := "concourse"
	if len(optionalRepo) > 0 {
		repo = optionalRepo[0]
	}

	return booklit.Link{
		Target: fmt.Sprintf("http://github.com/concourse/%s/issues/%s", repo, number),
		Content: booklit.Styled{
			Style:   booklit.StyleBold,
			Content: booklit.String(repo + " #" + number),
		},
	}
}

func (p Plugin) Resource(name string) booklit.Content {
	return booklit.Link{
		Target: fmt.Sprintf("http://github.com/concourse/%s-resource", name),
		Content: booklit.Sequence{
			booklit.Styled{
				Style:   booklit.StyleVerbatim,
				Content: booklit.String(name),
			},
			booklit.String(" resource"),
		},
	}
}

func (p Plugin) TutorialImage(path string) booklit.Content {
	return booklit.Styled{
		Style: "classed-image",
		Content: booklit.Image{
			Path:        path,
			Description: "tutorial image",
		},
		Partials: booklit.Partials{"Class": booklit.String("tutorial-image")},
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

func (p Plugin) PipelineImage(path string) booklit.Content {
	return booklit.Styled{
		Style: "pipeline-image",
		Content: booklit.Image{
			Path:        path,
			Description: "pipeline",
		},
	}
}

func (p Plugin) ReleaseWithGardenLinux(date string, concourseVersion string, gardenLinuxVersion string, content booklit.Content) (booklit.Content, error) {
	p.section.SetPartial("GardenReleaseFilename", booklit.String("garden-linux"))
	p.section.SetPartial("GardenReleaseName", booklit.String("Garden Linux"))
	return p.release(date, concourseVersion, gardenLinuxVersion, content)
}

func (p Plugin) Release(date string, concourseVersion string, gardenRunCVersion string, content booklit.Content) (booklit.Content, error) {
	p.section.SetPartial("GardenReleaseFilename", booklit.String("garden-runc"))
	p.section.SetPartial("GardenReleaseName", booklit.String("Garden runC"))
	return p.release(date, concourseVersion, gardenRunCVersion, content)
}

func (p Plugin) Note(commaSeparatedTags string, content booklit.Content) booklit.Content {
	tags := strings.Split(commaSeparatedTags, ",")

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
			"Tags": booklit.Sequence(tagNotes),
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

func (p Plugin) ReleaseVersion(version string) {
	p.section.SetTitle(booklit.String("v" + version))
	p.section.SetPartial("Version", booklit.String(version))
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

func (p Plugin) release(
	date string,
	concourseVersion string,
	gardenVersion string,
	content booklit.Content,
) (booklit.Content, error) {
	t, err := time.Parse("2006-1-2", date)
	if err != nil {
		return nil, err
	}

	p.section.Style = "release"

	p.section.SetTitle(booklit.String("v" + concourseVersion))

	p.section.SetPartial("Version", booklit.String(concourseVersion))

	p.section.SetPartial("GardenVersion", booklit.String(gardenVersion))

	p.section.SetPartial("ReleaseDate", booklit.Styled{
		Style:   "release-date",
		Content: booklit.String(t.Format("January 2, 2006")),
	})

	return content, nil
}

type NoIndex struct {
	booklit.Content
}

func (NoIndex) String() string {
	return ""
}
