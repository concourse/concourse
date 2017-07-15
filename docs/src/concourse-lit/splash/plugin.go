package splash

import (
	"pygmentize"

	"github.com/vito/booklit"
)

func init() {
	booklit.RegisterPlugin("concourse-splash", booklit.PluginFactoryFunc(NewPlugin))
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
	prop := booklit.Sequence{
		booklit.Block{
			Class:   "h4 value-prop-title",
			Content: title,
		},
		content,
	}

	vis := booklit.Element{
		Class: "value-pics",
		Content: booklit.Image{
			Path: "images/" + image + ".svg",
		},
	}

	return booklit.Block{
		Class: "container value-props",
		Content: booklit.Block{
			Class: "row",
			Content: booklit.Block{
				Class: "twelve columns value-prop-a center-columns",
				Content: booklit.Sequence{
					booklit.Block{Class: "six columns", Content: prop},
					booklit.Block{Class: "six columns", Content: vis},
				},
			},
		},
	}
}

func (p Plugin) ValuePropRight(title booklit.Content, image string, content booklit.Content) booklit.Content {
	prop := booklit.Sequence{
		booklit.Block{
			Class:   "h4 value-prop-title",
			Content: title,
		},
		content,
	}

	vis := booklit.Element{
		Class: "value-pics",
		Content: booklit.Image{
			Path: "images/" + image + ".svg",
		},
	}

	return booklit.Block{
		Class: "u-full-width value-prop-b",
		Content: booklit.Block{
			Class: "container",
			Content: booklit.Block{
				Class: "row",
				Content: booklit.Block{
					Class: "twelve columns value-prop-b-flex center-columns",
					Content: booklit.Sequence{
						booklit.Block{Class: "six columns", Content: vis},
						booklit.Block{Class: "six columns", Content: prop},
					},
				},
			},
		},
	}
}

func (p Plugin) GettingStarted(title, content booklit.Content) booklit.Content {
	return booklit.Block{
		Class: "u-full-width get-started",
		Content: booklit.Block{
			Class: "container",
			Content: booklit.Block{
				Class: "row",
				Content: booklit.Block{
					Class: "twelve columns",
					Content: booklit.Sequence{
						booklit.Block{
							Class:   "h3 started-title",
							Content: title,
						},
						content,
					},
				},
			},
		},
	}
}

func (p Plugin) Newline() booklit.Content {
	return booklit.String("\n")
}

func (p Plugin) StepWithLine(duration, title, content booklit.Content) booklit.Content {
	return booklit.Block{
		Class: "steps",
		Content: booklit.Sequence{
			booklit.Block{
				Class: "time-circle",
				Content: booklit.Sequence{
					booklit.Block{
						Class:   "time",
						Content: duration,
					},
					booklit.Element{
						Class: "line",
						Content: booklit.Image{
							Path: "images/line_simple.svg",
						},
					},
				},
			},
			booklit.Block{
				Class:   "h4 step-header",
				Content: title,
			},
			content,
		},
	}
}

func (p Plugin) Step(duration, title, content booklit.Content) booklit.Content {
	return booklit.Block{
		Class: "steps",
		Content: booklit.Sequence{
			booklit.Block{
				Class: "time-circle",
				Content: booklit.Sequence{
					booklit.Block{
						Class:   "time",
						Content: duration,
					},
				},
			},
			booklit.Block{
				Class:   "h4 step-header",
				Content: title,
			},
			content,
		},
	}
}

func (p Plugin) CodeLines(language string, lines ...booklit.Content) (booklit.Content, error) {
	buttons := booklit.Sequence{}
	for i := 0; i < 3; i++ {
		buttons = append(buttons, booklit.Block{
			Class:   "window-buttons",
			Content: booklit.Empty,
		})
	}

	codeLines := booklit.Sequence{}
	for _, line := range lines {
		code, err := pygmentize.Block(language, line.String())
		if err != nil {
			return nil, err
		}

		codeLines = append(codeLines, booklit.Block{
			Class:   "code-line",
			Content: code,
		})
	}

	return booklit.Block{
		Class: "code-step",
		Content: booklit.Sequence{
			buttons,
			booklit.Block{
				Class: "window-frame",
				Content: booklit.Block{
					Class:   "code-lines",
					Content: codeLines,
				},
			},
		},
	}, nil
}

func (p Plugin) CodeWindow(language string, content booklit.Content) (booklit.Content, error) {
	buttons := booklit.Sequence{}
	for i := 0; i < 3; i++ {
		buttons = append(buttons, booklit.Block{
			Class:   "window-buttons",
			Content: booklit.Empty,
		})
	}

	code, err := pygmentize.Block(language, content.String())
	if err != nil {
		return nil, err
	}

	return booklit.Block{
		Class: "code-step",
		Content: booklit.Sequence{
			buttons,
			booklit.Block{
				Class:   "window-frame",
				Content: code,
			},
		},
	}, nil
}

func (p Plugin) Coffee() booklit.Content {
	return booklit.Block{
		Class: "u-full-width",
		Content: booklit.Block{
			Class: "time-circle coffee",
			Content: booklit.Block{
				Class: "txt-c",
				Content: booklit.Element{
					Class: "coffee-icon",
					Content: booklit.Image{
						Path:        "images/coffee.svg",
						Description: "coffee",
					},
				},
			},
		},
	}
}

func (p Plugin) Truitt(title, content booklit.Content) booklit.Content {
	return booklit.Block{
		Class: "u-full-width come-fly",
		Content: booklit.Block{
			Class: "container",
			Content: booklit.Block{
				Class: "row",
				Content: booklit.Block{
					Class: "twelve columns",
					Content: booklit.Sequence{
						booklit.Element{
							Class:   "line-path",
							Content: booklit.Empty,
						},
						booklit.Block{
							Class: "fly-tagline",
							Content: booklit.Sequence{
								booklit.Block{
									Class:   "h4 fly-tagline-header",
									Content: title,
								},
								content,
							},
						},
					},
				},
			},
		},
	}
}

func (p Plugin) Slack(content booklit.Content) booklit.Content {
	return booklit.Block{
		Class: "u-full-width slack-section",
		Content: booklit.Block{
			Class: "container",
			Content: booklit.Block{
				Class: "row",
				Content: booklit.Block{
					Class: "twelve columns center-columns",
					Content: booklit.Block{
						Class: "h6 slack-intro-text",
						Content: booklit.Sequence{
							booklit.Image{
								Path: "images/slack.png",
							},
							content,
						},
					},
				},
			},
		},
	}
}
