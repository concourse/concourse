package resourcestable

import "github.com/vito/booklit"

func init() {
	booklit.RegisterPlugin("concourse-resources-table", booklit.PluginFactoryFunc(NewPlugin))
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
	return booklit.Block{
		Class:   "resources-table",
		Content: body,
	}
}

func (p Plugin) Column(name booklit.Content, resources ...booklit.Content) booklit.Content {
	header := booklit.Block{
		Class:   "header",
		Content: name,
	}

	return booklit.List{
		Items: append([]booklit.Content{header}, resources...),
	}
}

func (p Plugin) Normal(content booklit.Content) booklit.Content {
	return booklit.Block{
		Class: "normal",
		Content: booklit.Block{
			Class:   "cell",
			Content: content,
		},
	}
}

func (p Plugin) Good(content booklit.Content) booklit.Content {
	return booklit.Block{
		Class: "good",
		Content: booklit.Block{
			Class:   "cell",
			Content: content,
		},
	}
}

func (p Plugin) Bad(content booklit.Content) booklit.Content {
	return booklit.Block{
		Class: "bad",
		Content: booklit.Block{
			Class:   "cell",
			Content: content,
		},
	}
}

func (p Plugin) Gap(content booklit.Content) booklit.Content {
	return booklit.Block{
		Class: "gap",
		Content: booklit.Block{
			Class:   "cell",
			Content: content,
		},
	}
}

func (p Plugin) Disabled(content booklit.Content) booklit.Content {
	return booklit.Block{
		Class: "disabled",
		Content: booklit.Block{
			Class:   "cell",
			Content: content,
		},
	}
}
