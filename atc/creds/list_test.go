package creds_test

import (
	"github.com/concourse/concourse/v8/atc/creds"
	"github.com/concourse/concourse/v8/vars"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Evaluate", func() {
	It("interpolates the full list", func() {
		list, err := creds.NewList(
			vars.StaticVariables{"list": []string{"foo", "bar"}},
			"((list))",
		).Evaluate()
		Expect(err).ToNot(HaveOccurred())
		Expect(list).To(Equal([]any{"foo", "bar"}))
	})

	It("interpolates within a list", func() {
		list, err := creds.NewList(
			vars.StaticVariables{"element": "blah"},
			[]any{"abc", "((element))"},
		).Evaluate()
		Expect(err).ToNot(HaveOccurred())
		Expect(list).To(Equal([]any{"abc", "blah"}))
	})
})
