package ops_test

func (s *OpsSuite) TestUpgrade() {
	dc, err := s.dockerCompose("overrides/latest.yml")
	s.NoError(err)

	s.Run("deploy latest", func() {
		s.NoError(dc.Run("up", "-d"))
	})

	fly := s.initFly(dc)

	s.Run("set up test pipeline", func() {
		err = fly.Run("set-pipeline", "-p", "test", "-c", "pipelines/smoke-pipeline.yml", "-n")
		s.NoError(err)

		err = fly.Run("unpause-pipeline", "-p", "test")
		s.NoError(err)

		err = fly.Run("trigger-job", "-j", "test/say-hello", "-w")
		s.NoError(err)
	})

	dc, err = s.dockerCompose()
	s.NoError(err)

	s.Run("upgrade to dev", func() {
		s.NoError(dc.Run("up", "-d"))
	})

	fly = s.initFly(dc)

	s.Run("pipeline and build still exists", func() {
		err := fly.Run("get-pipeline", "-p", "test")
		s.NoError(err)

		out, err := fly.Output("watch", "-j", "test/say-hello", "-b", "1")
		s.NoError(err)
		s.Contains(out, "Hello, world!")
	})

	s.Run("can still run pipeline builds", func() {
		err := fly.Run("trigger-job", "-j", "test/say-hello", "-w")
		s.NoError(err)
	})

	s.Run("can still run checks", func() {
		err = fly.Run("check-resource", "-r", "test/mockery")
		s.NoError(err)
	})

	s.Run("can still reach the internet", func() {
		out, err := fly.Output("trigger-job", "-j", "test/use-the-internet", "-w")
		s.NoError(err)
		s.Contains(out, "Example Domain")
	})

	s.Run("can still run one-off builds", func() {
		out, err := fly.Output("execute", "-c", "tasks/hello.yml")
		s.NoError(err)
		s.Contains(out, "hello")
	})
}
