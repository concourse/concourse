package db_test

import (
	. "github.com/onsi/ginkgo"

	. "github.com/concourse/atc/db"
	"github.com/concourse/atc/redisrunner"
)

var _ = Describe("RedisDB", func() {
	var redisRunner *redisrunner.Runner

	BeforeEach(func() {
		redisRunner = redisrunner.NewRunner()
		redisRunner.Start()

		db = NewRedis(redisRunner.Pool())
	})

	AfterEach(func() {
		redisRunner.Stop()
	})

	itIsADB()
})
