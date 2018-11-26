package resourcestable

import "github.com/vito/booklit"

func init() {
	booklit.RegisterPlugin("concourse-resources-table", NewPlugin)
}

type Plugin struct {
	section *booklit.Section
}

func NewPlugin(section *booklit.Section) booklit.Plugin {
	return Plugin{
		section: section,
	}
}

func (p Plugin) ResourcesTable(body booklit.Content) booklit.Content {
	return booklit.Styled{
		Style:   "resources-table",
		Content: body,
	}
}

func (p Plugin) Column(name booklit.Content, resources ...booklit.Content) booklit.Content {
	header := booklit.Styled{
		Style:   "resources-table-header",
		Content: name,
	}

	return booklit.List{
		Items: append([]booklit.Content{header}, resources...),
	}
}

func (p Plugin) Normal(content booklit.Content) booklit.Content {
	return booklit.Styled{
		Style:    "resources-table-cell",
		Content:  content,
		Block:    true,
		Partials: booklit.Partials{"State": booklit.String("normal")},
	}
}

func (p Plugin) Good(content booklit.Content) booklit.Content {
	return booklit.Styled{
		Style:    "resources-table-cell",
		Content:  content,
		Block:    true,
		Partials: booklit.Partials{"State": booklit.String("good")},
	}
}

func (p Plugin) Bad(content booklit.Content) booklit.Content {
	return booklit.Styled{
		Style:    "resources-table-cell",
		Content:  content,
		Block:    true,
		Partials: booklit.Partials{"State": booklit.String("bad")},
	}
}

func (p Plugin) Gap(content booklit.Content) booklit.Content {
	return booklit.Styled{
		Style:    "resources-table-cell",
		Content:  content,
		Block:    true,
		Partials: booklit.Partials{"State": booklit.String("gap")},
	}
}

func (p Plugin) Disabled(content booklit.Content) booklit.Content {
	return booklit.Styled{
		Style:    "resources-table-cell",
		Content:  content,
		Block:    true,
		Partials: booklit.Partials{"State": booklit.String("disabled")},
	}
}
