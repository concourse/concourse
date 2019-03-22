package list

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/vito/booklit"
)

type ResourceType struct {
	Name       string     `json:"name"`
	Site       string     `json:"site"`
	Maintainer Maintainer `json:"maintainer"`
}

type Maintainer struct {
	Name    string `json:"name"`
	Contact string `json:"contact"`
}

func (rt ResourceType) GetName() booklit.Content {
	return booklit.Link{
		Target:  rt.Site,
		Content: booklit.String(rt.Name),
	}
}

func (rt ResourceType) GetMaintainer() booklit.Content {
	content := booklit.Sequence{}

	content = append(content, booklit.String("by "))

	content = append(content, booklit.Styled{
		Style: booklit.StyleBold,
		Content: booklit.Link{
			Target:  rt.Maintainer.Contact,
			Content: booklit.String("@" + rt.Maintainer.Name),
		},
	})
	return content
}

func init() {
	booklit.RegisterPlugin("resource-type-list", NewPlugin)
}

func NewPlugin(sec *booklit.Section) booklit.Plugin {
	return Plugin{
		section: sec,
	}
}

type Plugin struct {
	section *booklit.Section
}

func (plugin Plugin) BuildResourceTypeList(
	path string,
	columns ...string,
) (booklit.Content, error) {
	if path == "" {
		return nil, fmt.Errorf("path cannot be blank")
	}

	if len(columns) == 0 {
		return nil, fmt.Errorf("must have at least one columns")
	}

	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	content := [][]booklit.Content{}

	headers := []booklit.Content{}

	headers = append(headers, booklit.Styled{
		Style:   booklit.StyleBold,
		Content: booklit.String("Resource Type Name"),
	})
	headers = append(headers, booklit.Styled{
		Style:   booklit.StyleBold,
		Content: booklit.String("Maintained By"),
	})
	content = append(content, headers)

	for _, f := range files {
		jsonFile, err := os.Open(path + "/" + f.Name())
		if err != nil {
			return nil, err
		}
		byteValue, err := ioutil.ReadAll(jsonFile)
		if err != nil {
			return nil, err
		}

		var resource_type ResourceType
		err = json.Unmarshal(byteValue, &resource_type)
		if err == nil {
			line := []booklit.Content{}

			line = append(line, resource_type.GetName())
			line = append(line, resource_type.GetMaintainer())

			content = append(content, line)
		}
		if err != nil {
			var resource_types []ResourceType
			err := json.Unmarshal(byteValue, &resource_types)
			if err != nil {
				return nil, err
			}

			for _, resource_type := range resource_types {
				line := []booklit.Content{}

				line = append(line, resource_type.GetName())
				line = append(line, resource_type.GetMaintainer())

				content = append(content, line)

			}

		}
	}

	return booklit.Table{
		Rows: content,
	}, nil
}
