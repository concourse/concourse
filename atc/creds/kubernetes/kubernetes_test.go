package kubernetes_test

import (
	"github.com/concourse/concourse/v5/atc/creds"
	"github.com/concourse/concourse/v5/atc/creds/kubernetes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Kubernetes", func() {

	var k creds.Secrets

	JustBeforeEach(func() {
		factory := kubernetes.NewKubernetesFactory(nil, nil, "namespace")
		k = factory.NewSecrets()
	})

	Describe("NewSecretLookupPaths()", func() {
		It("should transform variable names to kubernetes secret names correctly", func() {
			pathObjects := k.NewSecretLookupPaths("team", "pipeline")
			var paths []string
			for _, p := range pathObjects {
				path, err := p.VariableToSecretPath("variable")
				Expect(err).To(BeNil())
				paths = append(paths, path)
			}

			Expect(len(paths)).To(BeEquivalentTo(2))
			Expect(paths).To(ContainElement("namespaceteam:pipeline.variable"))
			Expect(paths).To(ContainElement("namespaceteam:variable"))
		})

	})
})
