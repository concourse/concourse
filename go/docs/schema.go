package docs

import (
	"crypto/sha1"
	"fmt"
	"strings"

	"github.com/vito/booklit"
	"github.com/vito/booklit/ast"
	"github.com/vito/booklit/stages"
)

func (p *Plugin) Schema(name string, contentNode ast.Node) (booklit.Content, error) {
	old := p.schemaContext
	p.schemaContext = []string{name}
	defer func() {
		p.schemaContext = old
	}()

	tagName := "schema." + name

	p.section.SetTagAnchored(
		tagName,
		booklit.Sequence{
			booklit.Styled{
				Style: booklit.StyleBold,
				Content: booklit.Styled{
					Style:   booklit.StyleVerbatim,
					Content: booklit.String(name),
				},
			},
			booklit.String(" schema"),
		},
		p.section.InvokeLocation,
		booklit.Empty,
		tagName,
	)

	stage := &stages.Evaluate{
		Section: p.section,
	}

	err := contentNode.Visit(stage)
	if err != nil {
		return nil, err
	}

	content := stage.Result

	return booklit.Styled{
		Style:   "schema",
		Block:   true,
		Content: content,
		Partials: booklit.Partials{
			"Name":   booklit.String(name),
			"Anchor": booklit.String(tagName),
		},
	}, nil
}

func (p *Plugin) SchemaGroup(title booklit.Content, tagName string, contentNode ast.Node) (booklit.Content, error) {
	p.pushSchema(tagName)
	defer p.popSchema()

	p.schemaGroupTitle = title
	defer func() {
		p.schemaGroupTitle = nil
	}()

	stage := &stages.Evaluate{
		Section: p.section,
	}

	err := contentNode.Visit(stage)
	if err != nil {
		return nil, err
	}

	content := stage.Result

	return booklit.Styled{
		Style:   "schema-group",
		Block:   true,
		Content: content,
		Partials: booklit.Partials{
			"Title":   title,
			"TagName": booklit.String(tagName),
			"Target": booklit.Target{
				TagName:  tagName,
				Location: p.section.InvokeLocation,
				Title:    title,
				Content:  content,
			},
		},
	}, nil
}

func (p *Plugin) RequiredAttribute(attribute string, type_ string, contentNode ast.Node) (booklit.Content, error) {
	return p.schemaAttribute(
		attribute,
		type_,
		contentNode,
		booklit.Partials{
			"Required": booklit.String("true"),
		},
	)
}

func (p *Plugin) OptionalAttribute(attribute string, type_ string, contentNode ast.Node) (booklit.Content, error) {
	return p.schemaAttribute(
		attribute,
		type_,
		contentNode,
		booklit.Partials{
			"Optional": booklit.String("true"),
		},
	)
}

func (p *Plugin) ExampleToggle(title, content booklit.Content) booklit.Content {
	uniq := strings.Join(p.schemaContext, ".") + title.String()
	hash := sha1.Sum([]byte(uniq))

	return booklit.Styled{
		Style:   "example-toggle",
		Block:   true,
		Content: content,
		Partials: booklit.Partials{
			"Title": title,
			"For":   booklit.String(fmt.Sprintf("%x", hash)),
		},
	}
}

func (p *Plugin) OneOf(options ...booklit.Content) booklit.Content {
	return booklit.Styled{
		Style:   "schema-one-of",
		Block:   true,
		Content: booklit.Sequence(options),
	}
}

func (p *Plugin) SchemaAttribute(attribute string, type_ string, contentNode ast.Node) (booklit.Content, error) {
	return p.schemaAttribute(
		attribute,
		type_,
		contentNode,
		booklit.Partials{},
	)
}

func (p *Plugin) schemaAttribute(attribute string, type_ string, contentNode ast.Node, partials booklit.Partials) (booklit.Content, error) {
	p.pushSchema(attribute)
	defer p.popSchema()

	tagName := "schema." + strings.Join(p.schemaContext, ".")

	stage := &stages.Evaluate{
		Section: p.section,
	}

	err := contentNode.Visit(stage)
	if err != nil {
		return nil, err
	}

	content := stage.Result

	var display booklit.Content
	if p.schemaGroupTitle != nil {
		display = booklit.Sequence{
			p.schemaGroupTitle,
			booklit.String(" "),
			booklit.Styled{
				Style: booklit.StyleBold,
				Content: booklit.Styled{
					Style:   booklit.StyleVerbatim,
					Content: booklit.String(attribute),
				},
			},
		}
	} else {
		attr := booklit.Sequence{}
		for _, con := range p.schemaContext {
			field := con
			if len(attr) > 0 {
				field = "." + field
			}

			attr = append(attr, booklit.String(field))
		}

		display = booklit.Styled{
			Style:   "schema-attribute-name",
			Content: attr,
		}
	}

	targets := booklit.Sequence{
		booklit.Target{
			TagName:  tagName,
			Location: p.section.InvokeLocation,
			Title:    display,
			Content:  content,
		},
	}

	partials["TagName"] = booklit.String(tagName)
	partials["Targets"] = targets
	partials["Attribute"] = booklit.String(attribute)
	partials["Type"] = p.autoReferenceType(type_)

	return OmitFromSearchExcerpt{
		booklit.Styled{
			Style:    "schema-attribute",
			Block:    true,
			Content:  content,
			Partials: partials,
		},
	}, nil
}

func (p *Plugin) pushSchema(attribute string) {
	p.schemaContext = append(p.schemaContext, attribute)
}

func (p *Plugin) popSchema() {
	p.schemaContext = p.schemaContext[0 : len(p.schemaContext)-1]
}
