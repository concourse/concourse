package vars_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/concourse/concourse/vars"
)

func TestReg(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "director/template")
}

type FakeVariables struct {
	GetFunc      func(Reference) (any, bool, error)
	GetVarDef    Reference
	GetErr       error
	GetCallCount int
}

func (v *FakeVariables) Get(ref Reference) (any, bool, error) {
	v.GetCallCount += 1
	v.GetVarDef = ref
	if v.GetFunc != nil {
		return v.GetFunc(ref)
	}
	return nil, false, v.GetErr
}

func (v *FakeVariables) List() ([]Reference, error) {
	return nil, nil
}
