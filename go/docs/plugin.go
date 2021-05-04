package docs

import (
	"fmt"
	"strings"

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
			TagName:  t,
			Location: p.section.InvokeLocation,
			Title:    display,
			Content:  content,
		})
	}

	return OmitFromSearchExcerpt{
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
								TagName:  tags[0],
								Location: p.section.InvokeLocation,
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

func (p Plugin) Boshprop(job string, target string) booklit.Content {
	return booklit.Link{
		Target: fmt.Sprintf("https://bosh.io/jobs/%s?source=github.com/concourse/concourse-bosh-release#p=%s", job, target),
		Content: booklit.Styled{
			Style:   booklit.StyleVerbatim,
			Content: booklit.String(target),
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

func (p Plugin) Team(name string) booklit.Content {
	return booklit.Link{
		Target: fmt.Sprintf("https://github.com/concourse/governance/blob/master/teams/%s.yml", name),
		Content: booklit.Sequence{
			booklit.Styled{
				Style:   booklit.StyleBold,
				Content: booklit.String(name),
			},
			booklit.String(" team"),
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

type OmitFromSearchExcerpt struct {
	booklit.Content
}

// Strings returns an empty string, preventing the content from appearing in
// search index excerpts.
func (OmitFromSearchExcerpt) String() string {
	return ""
}
