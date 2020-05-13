package runtime_test

import (
	"bytes"
	"testing"

	"github.com/concourse/concourse/worker/runtime"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type UserNamespaceSuite struct {
	suite.Suite
	*require.Assertions
}

func (s *UserNamespaceSuite) TestMaxValid() {
	for _, tc := range []struct {
		desc      string
		input     string
		shouldErr bool
		val       uint32
	}{
		{
			desc:      "empty input",
			shouldErr: true,
		},
		{
			desc:      "invalid input",
			input:     "0",
			shouldErr: true,
		},
		{
			desc:  "size of 1",
			input: "0 1 1",
			val:   0,
		},
		{
			desc:  "size > 1",
			input: "0 1 10",
			val:   9,
		},
		{
			desc:  "size > maxUint32",
			input: "0 1 10",
			val:   9,
		},
		{
			desc:  "multiline size = 1",
			input: "0 1 1\n1 2 1",
			val:   1,
		},
		{
			desc:  "multiline size = 1 first",
			input: "1 2 1\n0 1 1",
			val:   1,
		},
	} {
		s.T().Run(tc.desc, func(t *testing.T) {
			res, err := runtime.MaxValid(bytes.NewBufferString(tc.input))
			if tc.shouldErr {
				s.Error(err)
				return
			}

			s.NoError(err)
			s.Equal(int(tc.val), int(res))
		})
	}
}
