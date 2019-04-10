package main

import "github.com/vito/booklit"

type ResourceType struct {
	Name       string     `json:"name"`
	Site       string     `json:"site"`
	Maintainer Maintainer `json:"maintainer"`
}

type Maintainer struct {
	Name    string `json:"name"`
	Contact string `json:"contact"`
}

func (rt ResourceType) NameLink() booklit.Content {
	return booklit.Link{
		Target:  rt.Site,
		Content: booklit.String(rt.Name),
	}
}

func (rt ResourceType) MaintainerLink() booklit.Content {
	return booklit.Sequence{
		booklit.String("by "),
		booklit.Link{
			Target: rt.Maintainer.Contact,
			Content: booklit.Styled{
				Style:   booklit.StyleBold,
				Content: booklit.String("@" + rt.Maintainer.Name),
			},
		},
	}
}
