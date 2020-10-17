package flaghelpers_test

import (
	"github.com/concourse/concourse/atc"
	. "github.com/concourse/concourse/fly/commands/internal/flaghelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceFlag", func() {
	var flag *ResourceFlag

	BeforeEach(func() {
		flag = &ResourceFlag{}
	})

	for _, tt := range []struct {
		desc         string
		flag         string
		pipelineRef  atc.PipelineRef
		resourceName string
		err          string
	}{
		{
			desc:         "basic",
			flag:         "some-pipeline/some-resource",
			pipelineRef:  atc.PipelineRef{Name: "some-pipeline"},
			resourceName: "some-resource",
		},
		{
			desc: "instance vars",
			flag: "some-pipeline/branch:master,foo.bar:baz/some-resource",
			pipelineRef: atc.PipelineRef{
				Name:         "some-pipeline",
				InstanceVars: atc.InstanceVars{"branch": "master", "foo": map[string]interface{}{"bar": "baz"}},
			},
			resourceName: "some-resource",
		},
		{
			desc: "instance var with a '/'",
			flag: "some-pipeline/branch:feature/do_things,foo:bar/some-resource",
			pipelineRef: atc.PipelineRef{
				Name:         "some-pipeline",
				InstanceVars: atc.InstanceVars{"branch": "feature/do_things", "foo": "bar"},
			},
			resourceName: "some-resource",
		},
		{
			desc: "only pipeline specified",
			flag: "some-pipeline",
			err:  "argument format should be <pipeline>/<resource>",
		},
		{
			desc: "resource name not specified",
			flag: "some-pipeline/",
			err:  "argument format should be <pipeline>/<resource>",
		},
		{
			desc: "malformed instance var",
			flag: "some-pipeline/branch=master/some-resource",
			err:  "argument format should be <pipeline>/<key:value>/<resource>",
		},
	} {
		tt := tt
		It(tt.desc, func() {
			err := flag.UnmarshalFlag(tt.flag)
			if tt.err == "" {
				Expect(err).ToNot(HaveOccurred())
				Expect(flag.PipelineRef).To(Equal(tt.pipelineRef))
				Expect(flag.ResourceName).To(Equal(tt.resourceName))
			} else {
				Expect(err).To(MatchError(tt.err))
			}
		})
	}
})
