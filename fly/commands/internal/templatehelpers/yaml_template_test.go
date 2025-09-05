package templatehelpers_test

import (
	"os"
	"path/filepath"

	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/commands/internal/templatehelpers"
	"github.com/concourse/concourse/vars"

	"github.com/concourse/concourse/atc"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("YAML Template With Params", func() {

	Describe("resolve", func() {
		var tmpdir string

		BeforeEach(func() {
			var err error

			tmpdir, err = os.MkdirTemp("", "yaml-template-test")
			Expect(err).NotTo(HaveOccurred())

			err = os.WriteFile(
				filepath.Join(tmpdir, "sample.yml"),
				[]byte(`section:
- param1: ((param1))
  param2: ((param2))
  param3:
    nested: ((param3))
`),
				0644,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			os.RemoveAll(tmpdir)
		})

		It("resolves all variables successfully", func() {
			variables := []flaghelpers.VariablePairFlag{
				{Ref: vars.Reference{Path: "param1"}, Value: "value1"},
				{Ref: vars.Reference{Path: "param2"}, Value: "value2"},
				{Ref: vars.Reference{Path: "param3"}, Value: "value3"},
			}
			sampleYaml := templatehelpers.NewYamlTemplateWithParams(atc.PathFlag(filepath.Join(tmpdir, "sample.yml")), nil, variables, nil, nil)
			result, err := sampleYaml.Evaluate(false, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(result)).To(Equal(`section:
- param1: value1
  param2: value2
  param3:
    nested: value3
`))
		})

		It("leave param uninterpolated if it's not provided", func() {
			variables := []flaghelpers.VariablePairFlag{
				{Ref: vars.Reference{Path: "param1"}, Value: "value1"},
				{Ref: vars.Reference{Path: "param2"}, Value: "value2"},
			}
			sampleYaml := templatehelpers.NewYamlTemplateWithParams(atc.PathFlag(filepath.Join(tmpdir, "sample.yml")), nil, variables, nil, nil)
			result, err := sampleYaml.Evaluate(false, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(result)).To(Equal(`section:
- param1: value1
  param2: value2
  param3:
    nested: ((param3))
`))
		})
	})
})
