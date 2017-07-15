package docs

import "github.com/vito/booklit"

func init() {
	booklit.RegisterPlugin("concourse-community", booklit.PluginFactoryFunc(NewPlugin))
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
	return booklit.Block{
		Class: "community-image",
		Content: booklit.Image{
			Path: path,
		},
	}
}

func (p Plugin) Talk(title, author booklit.Content, thumb string, videoURL string, slidesURL string, description booklit.Content) booklit.Content {
	var slidesLink booklit.Content = booklit.Empty
	if slidesURL != "" {
		slidesLink = booklit.Element{
			Class: "talk-slides-link",
			Content: booklit.Link{
				Target:  slidesURL,
				Content: booklit.String("slides"),
			},
		}
	}

	return booklit.Block{
		Class: "talk",
		Content: booklit.Sequence{
			booklit.Block{
				Class: "talk-info",
				Content: booklit.Sequence{
					booklit.Block{
						Class: "h2 talk-header",
						Content: booklit.Link{
							Target:  videoURL,
							Content: title,
						},
					},
					booklit.Block{
						Class: "talk-author",
						Content: booklit.Paragraph{
							booklit.String("by "),
							author,
						},
					},
					booklit.Block{
						Class:   "talk-description",
						Content: description,
					},
				},
			},
			booklit.Block{
				Class: "talk-links",
				Content: booklit.Block{
					Class: "talk-thumbnail",
					Content: booklit.Sequence{
						booklit.Link{
							Target: videoURL,
							Content: booklit.Image{
								Path:        thumb,
								Description: "talk thumbnail",
							},
						},
						slidesLink,
					},
				},
			},
		},
	}
}
