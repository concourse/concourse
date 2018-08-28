package db_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourceConfigVersions", func() {
	Describe("SaveMetadata", func() {
		var resourceConfigVersion db.ResourceConfigVersion
		var found bool
		var metadata db.ResourceConfigMetadataFields

		BeforeEach(func() {
			setupTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			brt := db.BaseResourceType{
				Name: "some-type",
			}
			_, err = brt.FindOrCreate(setupTx)
			Expect(err).ToNot(HaveOccurred())
			Expect(setupTx.Commit()).To(Succeed())

			resourceConfig, err := resourceConfigFactory.FindOrCreateResourceConfig(logger, "some-type", atc.Source{"some": "source"}, creds.VersionedResourceTypes{})
			Expect(err).ToNot(HaveOccurred())

			err = resourceConfig.SaveVersions([]atc.Version{
				{"version": "v1"},
				{"version": "v2"},
				{"version": "v3"},
			})
			Expect(err).ToNot(HaveOccurred())

			resourceConfigVersion, found, err = resourceConfig.FindVersion(atc.Version{"version": "v1"})
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			metadata = db.ResourceConfigMetadataFields{
				db.ResourceConfigMetadataField{
					Name:  "some-metadata",
					Value: "some-value",
				},
			}

			err = resourceConfigVersion.SaveMetadata(metadata)
			Expect(err).ToNot(HaveOccurred())

			reloaded, err := resourceConfigVersion.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(reloaded).To(BeTrue())
		})

		It("Saves the metadata", func() {
			Expect(resourceConfigVersion.Metadata()).To(Equal(metadata))
		})
	})
})
