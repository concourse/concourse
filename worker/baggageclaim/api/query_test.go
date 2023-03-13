package api_test

import (
	"net/url"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/worker/baggageclaim/api"
	"github.com/concourse/concourse/worker/baggageclaim/volume"
)

var _ = Describe("Query Parameters", func() {
	It("returns the properties when each query parameter key only has one value", func() {
		values := url.Values{}
		values.Add("name1", "value1")
		values.Add("name2", "value2")
		values.Add("name3", "value3")

		properties, err := api.ConvertQueryToProperties(values)
		Expect(err).NotTo(HaveOccurred())

		Expect(properties).To(Equal(volume.Properties{
			"name1": "value1",
			"name2": "value2",
			"name3": "value3",
		}))

	})

	It("returns an error when a query parameter has multiple values", func() {
		values := url.Values{}
		values.Add("name1", "value1")
		values.Add("name1", "value2")

		_, err := api.ConvertQueryToProperties(values)
		Expect(err).To(HaveOccurred())
	})

	It("returns empty properties when there are no query parameters", func() {
		values := url.Values{}

		properties, err := api.ConvertQueryToProperties(values)
		Expect(err).NotTo(HaveOccurred())

		Expect(properties).To(BeEmpty())
	})
})
