package docs

import (
	"fmt"
	"pygmentize"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/vito/booklit"
)

var flyBinariesVersion = semver.MustParse("2.2.0")

func init() {
	booklit.RegisterPlugin("concourse-docs", booklit.PluginFactoryFunc(NewPlugin))
}

type Plugin struct {
	section *booklit.Section
}

func NewPlugin(section *booklit.Section) booklit.Plugin {
	return Plugin{
		section: section,
	}
}

func (p Plugin) FontAwesome(class string) booklit.Content {
	return booklit.Element{
		Class:   "fa " + class,
		Content: booklit.Empty,
	}
}

func (p Plugin) Codeblock(language string, code booklit.Content) (booklit.Content, error) {
	code, err := pygmentize.Block(language, code.String())
	if err != nil {
		return nil, err
	}

	return booklit.Block{
		Class:   "code",
		Content: code,
	}, nil
}

func (p Plugin) TitledCodeblock(title booklit.Content, language string, code booklit.Content) (booklit.Content, error) {
	codeblock, err := p.Codeblock(language, code)
	if err != nil {
		return nil, err
	}

	return booklit.Block{
		Class: "titled-codeblock",
		Content: booklit.Sequence{
			booklit.Block{
				Class: "codeblock-title",
				Content: booklit.Styled{
					Style:   booklit.StyleVerbatim,
					Content: title,
				},
			},
			codeblock,
		},
	}, nil
}

func (p Plugin) Warn(content booklit.Content) booklit.Content {
	return booklit.Block{
		Class:   "warning",
		Content: content,
	}
}

func (p Plugin) DefineAttribute(attribute string, content booklit.Content, tags ...string) (booklit.Content, error) {
	attrSplit := strings.SplitN(attribute, ":", 2)

	attrName := attrSplit[0]
	if len(tags) == 0 {
		tags = []string{attrName}
	}

	display := booklit.Styled{
		Style: booklit.StyleVerbatim,
		Content: booklit.Styled{
			Style:   booklit.StyleBold,
			Content: booklit.String(attrName),
		},
	}

	targets := booklit.Sequence{}
	for _, t := range tags {
		targets = append(targets, booklit.Target{
			TagName: t,
			Display: display,
		})
	}

	return booklit.Block{
		Class: "definition",
		Content: booklit.Sequence{
			booklit.Block{
				Class: "thumb",
				Content: booklit.Sequence{
					targets,
					booklit.Styled{
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
			content,
		},
	}, nil
}

func (p Plugin) DefineMetric(metric string, content booklit.Content) booklit.Content {
	return booklit.Block{
		Class: "definition",
		Content: booklit.Sequence{
			booklit.Block{
				Class: "thumb",
				Content: booklit.Sequence{
					booklit.Target{
						TagName: metric,
						Display: booklit.String(metric),
					},
					booklit.Styled{
						Style: booklit.StyleVerbatim,
						Content: booklit.Block{
							Content: booklit.String(metric),
						},
					},
				},
			},
			content,
		},
	}
}

func (p Plugin) Boshprop(job string, target string) booklit.Content {
	return booklit.Link{
		Target: fmt.Sprintf("http://bosh.io/jobs/%s?source=github.com/concourse/concourse#p=%s", job, target),
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
			Content: booklit.String(user),
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
			Content: booklit.String("#" + number),
		},
	}
}

func (p Plugin) Resource(resource string, optionalName ...string) booklit.Content {
	name := ""
	if len(optionalName) > 0 {
		name = optionalName[0]
	} else {
		for _, word := range strings.Split(resource, "-") {
			if name != "" {
				name += " "
			}

			name += strings.Title(word)
		}
	}

	return booklit.Link{
		Target:  fmt.Sprintf("http://github.com/concourse/%s-resource", resource),
		Content: booklit.String(fmt.Sprintf("%s resource", name)),
	}
}

func (p Plugin) TutorialImage(path string) booklit.Content {
	return booklit.Block{
		Class: "tutorial-image",
		Content: booklit.Image{
			Path:        path,
			Description: "tutorial image",
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

	return booklit.Block{
		Class: "literate-segment",
		Content: booklit.Block{
			Class: "literate-entry",
			Content: booklit.Sequence{
				booklit.Block{
					Class:   "prose",
					Content: booklit.Sequence(paras),
				},
				code,
			},
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

	p.section.SetTitle(booklit.String("v" + concourseVersion))

	p.section.SetPartial("Version", booklit.String(concourseVersion))
	p.section.SetPartial("VersionLabel", booklit.Element{
		Class:   "version-number",
		Content: booklit.String("v" + concourseVersion),
	})

	p.section.SetPartial("GardenVersion", booklit.String(gardenVersion))
	p.section.SetPartial("GardenVersionLabel", booklit.Element{
		Class:   "version-number",
		Content: booklit.String("v" + gardenVersion),
	})

	p.section.SetPartial("ReleaseDate", booklit.Element{
		Class:   "release-date",
		Content: booklit.String(t.Format("January 1, 2006")),
	})

	cv, err := semver.Parse(concourseVersion)
	if err != nil {
		return nil, err
	}

	if cv.GTE(flyBinariesVersion) {
		p.section.SetPartial("HasFlyBinaries", booklit.Empty)
	}

	return content, nil
}

func (p Plugin) Note(commaSeparatedTags string, content booklit.Content) booklit.Content {
	tags := strings.Split(commaSeparatedTags, ",")

	tagNotes := []booklit.Content{}
	for _, t := range tags {
		tagNotes = append(tagNotes, booklit.Block{
			Class:   "note-tag " + t,
			Content: booklit.String(t),
		})
	}

	return booklit.Block{
		Class: "release-note",
		Content: booklit.Sequence{
			booklit.Block{
				Class: "tags",
				Content: booklit.List{
					Items: tagNotes,
				},
			},
			content,
		},
	}
}
