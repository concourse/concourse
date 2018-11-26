package docs

import "github.com/vito/booklit"

func init() {
	booklit.RegisterPlugin("concourse-community", NewPlugin)
}

type Plugin struct {
	section *booklit.Section
}

func NewPlugin(section *booklit.Section) booklit.Plugin {
	return Plugin{
		section: section,
	}
}

func (p Plugin) CommunityImage(path string) booklit.Content {
	return booklit.Styled{
		Style: "community-image",

		Content: booklit.Paragraph{},

		Partials: booklit.Partials{
			"Image": booklit.Image{
				Path: path,
			},
		},
	}
}

func (p Plugin) Talk(title, author booklit.Content, thumb string, videoURL string, slidesURL string, description booklit.Content) booklit.Content {
	var slidesLink booklit.Content
	if slidesURL != "" {
		slidesLink = booklit.Link{
			Target:  slidesURL,
			Content: booklit.String("slides"),
		}
	}

	return booklit.Styled{
		Style: "talk",

		Content: description,

		Partials: booklit.Partials{
			"Link": booklit.Link{
				Target:  videoURL,
				Content: title,
			},
			"Thumb": booklit.Link{
				Target: videoURL,
				Content: booklit.Image{
					Path:        thumb,
					Description: "talk thumbnail",
				},
			},
			"Author":     author,
			"SlidesLink": slidesLink,
		},
	}
}
