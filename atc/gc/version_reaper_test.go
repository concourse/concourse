package gc_test

import (
	"code.cloudfoundry.org/lager/lagerctx"
	"context"
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/gc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("VersionReaper", func() {
	var (
		maxVersionsToRetain int
		resourceConfigScopeId int
	)

	BeforeEach(func(){

		resource, found, err := defaultPipeline.Resource("some-resource")
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())

		resourceScope, err := resource.SetResourceConfig(atc.Source{"some": "source"}, atc.VersionedResourceTypes{})
		Expect(err).NotTo(HaveOccurred())

		resourceConfigScopeId = resourceScope.ID()
	})

	JustBeforeEach(func(){
		ctx := lagerctx.NewContext(context.Background(), logger)
		err := gc.NewVersionReaper(dbConn, maxVersionsToRetain).Run(ctx)
		Expect(err).ToNot(HaveOccurred())
	})

	Context("when version count not exceed the cap", func(){
		BeforeEach(func(){
			maxVersionsToRetain = 101
			for i := 0; i < 66; i ++ {
				insertVersion(i, resourceConfigScopeId)
			}
		})
		It("should not delete any version", func(){
			Expect(versionCount(resourceConfigScopeId, true)).To(Equal(1))
			Expect(versionCount(resourceConfigScopeId, false)).To(Equal(65))
		})
	})

	Context("when version count exceeds the cap", func(){
		BeforeEach(func(){
			maxVersionsToRetain = 101
			for i := 0; i < 200; i ++ {
				insertVersion(i, resourceConfigScopeId)
			}
		})
		It("should not delete any version", func(){
			Expect(versionCount(resourceConfigScopeId, true)).To(Equal(1))
			Expect(versionCount(resourceConfigScopeId, false)).To(Equal(maxVersionsToRetain))
		})
	})
})

func insertVersion(index int, resourceConfigScopeId int) {
	var versionMD5 string
	version := fmt.Sprintf(`{"some":"version-%d"}`, index)
	err = psql.Insert("resource_config_versions").
		Columns("version", "version_md5", "metadata", "check_order", "resource_config_scope_id").
		Values(version, sq.Expr(fmt.Sprintf("md5('%s')", version)), `null`, index, resourceConfigScopeId).
		Suffix("RETURNING version_md5").
		RunWith(dbConn).QueryRow().Scan(&versionMD5)
	Expect(err).NotTo(HaveOccurred())
}

func versionCount(resourceConfigScopeId int, zeroCheckOrder bool) int {
	var count int
	builder := psql.Select("count(*)").
		From("resource_config_versions").
		Where(sq.Eq{"resource_config_scope_id": resourceConfigScopeId})
	if zeroCheckOrder {
		builder = builder.Where(sq.Eq{"check_order": 0})
	} else {
		builder = builder.Where(sq.Gt{"check_order": 0})
	}
	err := builder.RunWith(dbConn).QueryRow().Scan(&count)
	Expect(err).ToNot(HaveOccurred())
	return count
}