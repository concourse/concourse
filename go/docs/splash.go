package docs

import "github.com/vito/booklit"

func (p Plugin) SplashIntro(intro, downloads booklit.Content) booklit.Content {
	return booklit.Styled{
		Style: "splash-intro",
		Block: true,

		Content: intro,

		Partials: booklit.Partials{
			"Downloads": downloads,
		},
	}
}

func (p Plugin) PageRegion(style, title, content booklit.Content) booklit.Content {
	return booklit.Styled{
		Style: "page-region",
		Block: true,

		Content: content,

		Partials: booklit.Partials{
			"Style": style,
			"Title": title,
		},
	}
}

func (p Plugin) QuickStart(content booklit.Content) booklit.Content {
	return booklit.Styled{
		Style:   "quick-start",
		Block:   true,
		Content: content,
	}
}

func (p Plugin) SplashExample(title booklit.Content, content booklit.Content, example booklit.Content) booklit.Content {
	return booklit.Styled{
		Style: "splash-example",
		Block: true,

		Content: content,

		Partials: booklit.Partials{
			"Title":   title,
			"Example": example,
		},
	}
}
