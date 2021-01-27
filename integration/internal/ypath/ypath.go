// Package ypath assists with modifying a YAML document in memory by mutating
// paths within the document.
//
// It is useful for situations where Docker Compose overrides aren't expressive
// enough - for example, a service may be literally copied rather than
// duplicating it in a separate override YAML file, which introduces risk as
// they may subtly drift.
package ypath

import (
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

type Document struct {
	file *ast.File
}

func Load(path string) (*Document, error) {
	file, err := parser.ParseFile(path, 0)
	if err != nil {
		return nil, err
	}

	return &Document{file}, nil
}

func (doc *Document) Bytes() []byte {
	return []byte(doc.file.String())
}

func (doc *Document) Read(pathString string, dest interface{}) error {
	path, err := yaml.PathString(pathString)
	if err != nil {
		return err
	}

	return path.Read(doc.file, dest)
}

func (doc *Document) Merge(pathString string, value interface{}) error {
	path, err := yaml.PathString(pathString)
	if err != nil {
		return err
	}

	node, err := yaml.NewEncoder(nil).EncodeToNode(value)
	if err != nil {
		return err
	}

	return path.MergeFromNode(doc.file, node)
}

func (doc *Document) Replace(pathString string, value interface{}) error {
	path, err := yaml.PathString(pathString)
	if err != nil {
		return err
	}

	node, err := yaml.NewEncoder(nil).EncodeToNode(value)
	if err != nil {
		return err
	}

	return path.ReplaceWithNode(doc.file, node)
}

func (doc *Document) Set(pathString string, value interface{}) error {
	segments := strings.Split(pathString, ".")
	field := segments[len(segments)-1]
	parent := strings.Join(segments[0:len(segments)-1], ".")

	path, err := yaml.PathString(parent)
	if err != nil {
		return err
	}

	node, err := yaml.NewEncoder(nil).EncodeToNode(map[string]interface{}{
		field: value,
	})
	if err != nil {
		return err
	}

	return path.MergeFromNode(doc.file, node)
}
