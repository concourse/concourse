package splash

import (
	"os"
	"pygmentize"

	"github.com/vito/booklit"
)

func init() {
	booklit.RegisterPlugin("concourse-splash", NewPlugin)
}

type Plugin struct {
	section *booklit.Section
}

func NewPlugin(section *booklit.Section) booklit.Plugin {
	return Plugin{
		section: section,
	}
}

func (p Plugin) SoftLine(content booklit.Content) booklit.Content {
	return booklit.Styled{
		Style:   "soft-line",
		Content: content,
	}
}

func (p Plugin) LightButton(content booklit.Content) booklit.Content {
	return booklit.Styled{
		Style:   "light-button",
		Content: content,
	}
}

func (p Plugin) BlueButton(content booklit.Content) booklit.Content {
	return booklit.Styled{
		Style:   "blue-button",
		Content: content,
	}
}

func (p Plugin) ValuePropLeft(title booklit.Content, image string, content booklit.Content) booklit.Content {
	return booklit.Styled{
		Style: "value-prop-left",

		Content: content,

		Partials: booklit.Partials{
			"Title": title,
			"Image": booklit.Image{
				Path:        "images/" + image + ".svg",
				Description: "value prop",
			},
		},
	}
}

func (p Plugin) ValuePropRight(title booklit.Content, image string, content booklit.Content) booklit.Content {
	return booklit.Styled{
		Style: "value-prop-right",

		Content: content,

		Partials: booklit.Partials{
			"Title": title,
			"Image": booklit.Image{
				Path:        "images/" + image + ".svg",
				Description: "value prop",
			},
		},
	}
}

func (p Plugin) ReleaseLink(file string) booklit.Content {
	version := os.Getenv("CONCOURSE_VERSION")
	if version == "" {
		version = "0.0.0"
	}

	url := "https://github.com/concourse/concourse/releases/download/v" + version + "/" + file
	return booklit.Link{
		Target:  url,
		Content: booklit.String(url),
	}
}

func (p Plugin) GettingStarted(title, content booklit.Content) booklit.Content {
	return booklit.Styled{
		Style: "getting-started",

		Content: content,

		Partials: booklit.Partials{
			"Title": title,
		},
	}
}

func (p Plugin) Newline() booklit.Content {
	return booklit.String("\n")
}

func (p Plugin) StepWithLine(duration, title, content booklit.Content) booklit.Content {
	return booklit.Styled{
		Style: "step-with-line",

		Content: content,

		Partials: booklit.Partials{
			"Title":    title,
			"Duration": duration,
			"Line": booklit.Image{
				Path: "images/line_simple.svg",
			},
		},
	}
}

func (p Plugin) Step(duration, title, content booklit.Content) booklit.Content {
	return booklit.Styled{
		Style: "step",

		Content: content,

		Partials: booklit.Partials{
			"Title":    title,
			"Duration": duration,
		},
	}
}

func (p Plugin) CodeLines(lines ...booklit.Content) (booklit.Content, error) {
	codeLines := booklit.Sequence{}
	for _, line := range lines {
		codeLines = append(codeLines, booklit.Styled{
			Style:   booklit.StyleVerbatim,
			Content: line,
		})
	}

	return booklit.Styled{
		Style: "code-lines",

		Content: codeLines,
	}, nil
}

func (p Plugin) CodeWindow(language string, content booklit.Content) (booklit.Content, error) {
	code, err := pygmentize.Block(language, content.String())
	if err != nil {
		return nil, err
	}

	return booklit.Styled{
		Style: "code-window",

		Content: code,
	}, nil
}

func (p Plugin) Coffee() booklit.Content {
	return booklit.Styled{
		Style: "coffee",

		Content: booklit.Paragraph{},

		Partials: booklit.Partials{
			"Image": booklit.Image{
				Path:        "images/coffee.svg",
				Description: "coffee",
			},
		},
	}
}

func (p Plugin) Truitt(title, content booklit.Content) booklit.Content {
	return booklit.Styled{
		Style: "truitt",

		Content: content,

		Partials: booklit.Partials{
			"Title": title,
		},
	}
}

func (p Plugin) Slack(content booklit.Content) booklit.Content {
	return booklit.Styled{
		Style: "slack-blurb",

		Content: content,
	}
}
