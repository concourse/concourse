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
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/stretchr/testify/require"
)

type Document struct {
	file *ast.File
}

func Load(t *testing.T, path string) *Document {
	file, err := parser.ParseFile(path, 0)
	require.NoError(t, err)
	return &Document{file}
}

func (doc *Document) Bytes() []byte {
	return []byte(doc.file.String())
}

func (doc *Document) Read(t *testing.T, pathString string, dest interface{}) {
	path, err := yaml.PathString(pathString)
	require.NoError(t, err)

	err = path.Read(doc.file, dest)
	require.NoError(t, err)
}

func (doc *Document) Merge(t *testing.T, pathString string, value interface{}) {
	path, err := yaml.PathString(pathString)
	require.NoError(t, err)

	node, err := yaml.NewEncoder(nil).EncodeToNode(value)
	require.NoError(t, err)

	err = path.MergeFromNode(doc.file, node)
	require.NoError(t, err)
}

func (doc *Document) Replace(t *testing.T, pathString string, value interface{}) {
	path, err := yaml.PathString(pathString)
	require.NoError(t, err)

	node, err := yaml.NewEncoder(nil).EncodeToNode(value)
	require.NoError(t, err)

	err = path.ReplaceWithNode(doc.file, node)
	require.NoError(t, err)
}

func (doc *Document) Set(t *testing.T, pathString string, value interface{}) {
	segments := strings.Split(pathString, ".")
	field := segments[len(segments)-1]
	parent := strings.Join(segments[0:len(segments)-1], ".")

	path, err := yaml.PathString(parent)
	require.NoError(t, err)

	node, err := yaml.NewEncoder(nil).EncodeToNode(map[string]interface{}{
		field: value,
	})
	require.NoError(t, err)

	err = path.MergeFromNode(doc.file, node)
	require.NoError(t, err)
}

func (doc *Document) Delete(t *testing.T, pathString string) {
	segments := strings.Split(pathString, ".")
	field := segments[len(segments)-1]
	parent := strings.Join(segments[0:len(segments)-1], ".")

	path, err := yaml.PathString(parent)
	require.NoError(t, err)

	var newMap map[string]interface{}
	doc.Read(t, parent, &newMap)
	delete(newMap, field)

	newNode, err := yaml.NewEncoder(nil).EncodeToNode(newMap)
	require.NoError(t, err)

	err = path.ReplaceWithNode(doc.file, newNode)
	require.NoError(t, err)
}

func (doc *Document) Clone(t *testing.T, srcPath string, dstPath string) {
	var src interface{}
	doc.Read(t, srcPath, &src)
	doc.Set(t, dstPath, src)
}

func (doc *Document) Move(t *testing.T, srcPath string, dstPath string) {
	doc.Clone(t, srcPath, dstPath)
	doc.Delete(t, srcPath)
}
