package creds

import (
	"bytes"
	"errors"
	"io/ioutil"
	"text/template"
	"text/template/parse"
)

// SecretLookupPath transforms variable name into full secret path
type SecretLookupPath interface {
	VariableToSecretPath(string) (string, error)
}

// SecretLookupWithPrefix is an implementation which returns [prefix][separator][varName]
type SecretLookupWithPrefix struct {
	Prefix string
}

func NewSecretLookupWithPrefix(prefix string) SecretLookupPath {
	return &SecretLookupWithPrefix{
		Prefix: prefix,
	}
}

func (sl SecretLookupWithPrefix) VariableToSecretPath(varName string) (string, error) {
	return sl.Prefix + varName, nil
}

// SecretLookupWithTemplate uses the given template to construct a lookup path specific
// to a team and pipeline
type SecretLookupWithTemplate struct {
	PathTemplate *template.Template
	TeamName string
	PipelineName string
}

func BuildSecretTemplate(name, tmpl string) (*template.Template, error) {
	t, err := template.New(name).Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return nil, err
	}

	if parse.IsEmptyTree(t.Root) {
		return nil, errors.New("secret template should not be empty")
	}

	// Validate that the template only consumes the expected keys
	dummy := struct {Team, Pipeline, Secret string}{"team", "pipeline", "secret"}
	if err = t.Execute(ioutil.Discard, &dummy); err != nil {
		return nil, err
	}

	return t, nil
}

func NewSecretLookupWithTemplate(pathTemplate *template.Template, teamName string, pipelineName string) SecretLookupPath {
	return &SecretLookupWithTemplate{
		PathTemplate: pathTemplate,
		TeamName: teamName,
		PipelineName: pipelineName,
	}
}

func (sl SecretLookupWithTemplate) VariableToSecretPath(varName string) (string, error) {
	var buf bytes.Buffer
	data := struct {
		Team string
		Pipeline string
		Secret string
	}{
		sl.TeamName,
		sl.PipelineName,
		varName,
	}

	err := sl.PathTemplate.Execute(&buf, &data)
	return buf.String(), err
}
