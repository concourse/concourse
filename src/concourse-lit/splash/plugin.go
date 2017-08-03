package splash

import (
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
	return booklit.Element{
		Class:   "soft-line",
		Content: content,
	}
}

func (p Plugin) LightButton(content booklit.Content) booklit.Content {
	return booklit.Element{
		Class:   "button button-primary button-light",
		Content: content,
	}
}

func (p Plugin) BlueButton(content booklit.Content) booklit.Content {
	return booklit.Element{
		Class:   "button button-primary button-blue",
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

func (p Plugin) CodeLines(language string, lines ...booklit.Content) (booklit.Content, error) {
	codeLines := booklit.Sequence{}
	for _, line := range lines {
		code, err := pygmentize.Block(language, line.String())
		if err != nil {
			return nil, err
		}

		codeLines = append(codeLines, code)
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
