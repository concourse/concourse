package atccmd_test

import (
	"github.com/concourse/concourse/atc/atccmd"
	flags "github.com/jessevdk/go-flags"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("RunCommand", func() {
	Context("Postgres Config", func() {
		var (
			cmd    *atccmd.RunCommand
			parser *flags.Parser
		)

		BeforeEach(func() {
			cmd = &atccmd.RunCommand{}
			parser = flags.NewParser(cmd, flags.Default^flags.PrintErrors)
			parser.NamespaceDelimiter = "-"
			args := []string{
				"--postgres-sslmode=verify-full",
				"--postgres-host=1.2.3.4",
				"--postgres-user=some user",
				"--postgres-database=atc",
				"--postgres-password=password \\ with ' funny ! chars",
				"--postgres-read-timeout=15s",
				"--postgres-write-timeout=30s",
			}
			pgConfigGroup := parser.Group.Find("PostgreSQL Configuration")
			Expect(pgConfigGroup).ToNot(BeNil())
			args, err := parser.ParseArgs(args)
			Expect(err).NotTo(HaveOccurred())
		})

		It("check connection string", func() {
			connectionString := cmd.Postgres.ConnectionString()
			Expect(connectionString).To(Equal("connect_timeout='300' dbname='atc' host='1.2.3.4' password='password \\\\ with \\' funny ! chars' port=5432 sslmode='verify-full' user='some user'"))
		})

		It("check read/write timeout connector", func() {
			readTimeout := cmd.Postgres.ReadTimeout
			Expect(readTimeout).To(Equal(15 * time.Second))
			writeTimeout := cmd.Postgres.WriteTimeout
			Expect(writeTimeout).To(Equal(30 * time.Second))
		})
	})
})
