package volume_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/worker/baggageclaim/volume"
)

var _ = Describe("Properties Superset", func() {
	It("return true when the two sets are equal", func() {
		properties := volume.Properties{
			"name": "value",
		}

		result := properties.HasProperties(properties)
		Expect(result).To(BeTrue())
	})

	It("returns true if all of the elements in the query are contained in the properties", func() {
		properties := volume.Properties{
			"name1": "value1",
			"name2": "value2",
		}

		query := volume.Properties{
			"name1": "value1",
		}

		result := properties.HasProperties(query)
		Expect(result).To(BeTrue())
	})

	It("returns false if the query has more elements than the properties", func() {
		properties := volume.Properties{
			"name1": "value1",
		}

		query := volume.Properties{
			"name1": "value1",
			"name2": "value2",
		}

		result := properties.HasProperties(query)
		Expect(result).To(BeFalse())
	})

	It("returns false if all of the names in the query are not contained in the properties", func() {
		properties := volume.Properties{
			"name1": "value1",
		}

		query := volume.Properties{
			"name2": "value1",
		}

		result := properties.HasProperties(query)
		Expect(result).To(BeFalse())
	})

	It("returns false if all of the names and values in the query are not contained in the properties", func() {
		properties := volume.Properties{
			"name1": "value1",
		}

		query := volume.Properties{
			"name1": "value2",
		}

		result := properties.HasProperties(query)
		Expect(result).To(BeFalse())
	})

	It("returns false if there is a query entry that does not exist in the properties", func() {
		properties := volume.Properties{
			"name1": "value1",
			"name2": "value2",
		}

		query := volume.Properties{
			"name1": "value1",
			"name3": "value3",
		}

		result := properties.HasProperties(query)
		Expect(result).To(BeFalse())
	})

	It("returns true if the query and properties are empty", func() {
		properties := volume.Properties{}
		query := volume.Properties{}

		result := properties.HasProperties(query)
		Expect(result).To(BeTrue())
	})

	It("returns true if the query is empty but properties are not", func() {
		properties := volume.Properties{
			"name1": "value1",
			"name2": "value2",
		}
		query := volume.Properties{}

		result := properties.HasProperties(query)
		Expect(result).To(BeTrue())
	})

	Describe("Update Property", func() {
		It("creates the property if it's not present", func() {
			properties := volume.Properties{}
			updatedProperties := properties.UpdateProperty("some", "property")

			Expect(updatedProperties).To(Equal(volume.Properties{"some": "property"}))
		})

		It("does not modify the original object", func() {
			properties := volume.Properties{}
			properties.UpdateProperty("some", "property")

			Expect(properties).To(Equal(volume.Properties{}))
		})

		It("updates the property if it exists already", func() {
			properties := volume.Properties{"some": "property"}
			updatedProperties := properties.UpdateProperty("some", "other-property")

			Expect(updatedProperties).To(Equal(volume.Properties{"some": "other-property"}))
		})
	})
})
