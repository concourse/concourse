package db_test

import (
	"context"
	"fmt"

	"github.com/concourse/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BuildEventIDSequenceFactory", func() {
	var (
		allocator db.BuildEventIDAllocator
		ctx       context.Context
	)

	BeforeEach(func() {
		allocator = db.NewBuildEventIDAllocator(dbConn)
		ctx = context.Background()

		var err error
		Expect(err).ToNot(HaveOccurred())
	})

	It("creates sequences", func() {
		err := allocator.Initialize(ctx, 1)
		Expect(err).ToNot(HaveOccurred())

		seqName := "build_event_id_seq_1"
		var exists bool
		err = dbConn.QueryRow("SELECT EXISTS (SELECT 1 FROM pg_class WHERE relkind = 'S' AND relname = '" + seqName + "')").Scan(&exists)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue(), "didn't create '"+seqName+"' sequence")
	})

	It("sequences can allocate increasing blocks of ids", func() {
		allocator.Initialize(ctx, 1)

		count := 100
		ids, err := allocator.Allocate(ctx, 1, count)
		Expect(err).ToNot(HaveOccurred())

		prev, _ := ids.Next()
		for i := 0; i < count - 1; i++ {
			cur, ok := ids.Next()
			Expect(ok).To(BeTrue())
			Expect(cur).To(BeNumerically(">", prev))
			prev = cur
		}
	})

	It("sequences can be destroyed", func() {
		allocator.Initialize(ctx, 1)

		err := allocator.Finalize(ctx, 1)
		Expect(err).ToNot(HaveOccurred())

		seqName := fmt.Sprintf("build_event_id_seq_1")
		var exists bool
		err = dbConn.QueryRow("SELECT EXISTS (SELECT 1 FROM pg_class WHERE relkind = 'S' AND relname = '" + seqName + "')").Scan(&exists)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeFalse(), "didn't destroy '"+seqName+"' sequence")
	})
})
