package plugin

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/vito/booklit"
)

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

func (plugin Plugin) BuildResourceTypeList(relPath string) (booklit.Content, error) {
	path := filepath.Join(filepath.Dir(plugin.section.Path), relPath)

	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	content := [][]booklit.Content{}

	headers := []booklit.Content{
		booklit.String("Resource Type"),
		booklit.String("Maintained By"),
	}

	content = append(content, headers)

	for _, f := range files {
		jsonFile, err := os.Open(filepath.Join(path, f.Name()))
		if err != nil {
			return nil, err
		}

		byteValue, err := ioutil.ReadAll(jsonFile)
		if err != nil {
			return nil, err
		}

		var resourceType ResourceType
		err = json.Unmarshal(byteValue, &resourceType)
		if err == nil {
			line := []booklit.Content{
				resourceType.NameLink(),
				resourceType.MaintainerLink(),
			}

			content = append(content, line)
		}
		if err != nil {
			var resourceTypes []ResourceType
			err := json.Unmarshal(byteValue, &resourceTypes)
			if err != nil {
				return nil, err
			}

			for _, resourceType := range resourceTypes {
				line := []booklit.Content{
					resourceType.NameLink(),
					resourceType.MaintainerLink(),
				}

				content = append(content, line)
			}
		}
	}

	return booklit.Table{
		Rows: content,
	}, nil
}
