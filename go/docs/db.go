package docs

import "github.com/vito/booklit"

func (p Plugin) DefineTable(table string, content booklit.Content) booklit.Content {
	tagName := table + "-table"

	return OmitFromSearchExcerpt{
		booklit.Styled{
			Style:   "definition",
			Content: content,
			Partials: booklit.Partials{
				"Targets": booklit.Target{
					TagName:  tagName,
					Location: p.section.InvokeLocation,
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
							TagName:  tagName,
							Location: p.section.InvokeLocation,
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
		TagName:  table + "-table",
		Location: p.section.InvokeLocation,
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
