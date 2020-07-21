package kubernetes_test

import (
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/creds/kubernetes"
	"github.com/concourse/concourse/vars"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type Example struct {
	Setup func()

	Template interface{}

	Result interface{}
	Err    error
}

func (example Example) Assert(vs vars.Variables) {
	if example.Setup != nil {
		example.Setup()
	}

	var res interface{}
	var err error

	switch t := example.Template.(type) {
	case string:
		res, err = creds.NewString(vs, t).Evaluate()
	case atc.Source:
		res, err = creds.NewSource(vs, t).Evaluate()
	case atc.Params:
		res, err = creds.NewParams(vs, t).Evaluate()
	}

	if example.Err != nil {
		Expect(err.Error()).To(ContainSubstring(example.Err.Error()))
	} else {
		Expect(res).To(Equal(example.Result))
	}
}

var _ = Describe("Kubernetes", func() {
	var fakeClientset *fake.Clientset
	var vs vars.Variables

	var secretName = "some-secret-name"

	BeforeEach(func() {
		fakeClientset = fake.NewSimpleClientset()

		factory := kubernetes.NewKubernetesFactory(
			lagertest.NewTestLogger("test"),
			fakeClientset,
			"prefix-",
		)

		vs = creds.NewVariables(factory.NewSecrets(), "some-team", "some-pipeline", false)
	})

	DescribeTable("var lookup", func(ex Example) {
		ex.Assert(vs)
	},

		Entry("bogus vars", Example{
			Template: "((bogus)) ((vars))",
			Err:      vars.UndefinedVarsError{Vars: []string{"bogus", "vars"}},
		}),

		Entry("team-scoped vars with a value field", Example{
			Setup: func() {
				fakeClientset.CoreV1().Secrets("prefix-some-team").Create(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: secretName,
					},
					Data: map[string][]byte{
						"value": []byte("some-value"),
					},
				})
			},

			Template: "((" + secretName + "))",
			Result:   "some-value",
		}),

		Entry("pipeline-scoped vars with a value field", Example{
			Setup: func() {
				fakeClientset.CoreV1().Secrets("prefix-some-team").Create(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "some-pipeline." + secretName,
					},
					Data: map[string][]byte{
						"value": []byte("some-value"),
					},
				})
			},

			Template: "((" + secretName + "))",
			Result:   "some-value",
		}),

		Entry("pipeline-scoped vars with arbitrary fields", Example{
			Setup: func() {
				fakeClientset.CoreV1().Secrets("prefix-some-team").Create(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "some-pipeline." + secretName,
					},
					Data: map[string][]byte{
						"some-field": []byte("some-field-value"),
					},
				})
			},

			Template: atc.Source{
				"some-source": "((" + secretName + "))",
			},
			Result: atc.Source{
				"some-source": map[string]interface{}{
					"some-field": "some-field-value",
				},
			},
		}),

		Entry("pipeline-scoped vars with arbitrary fields accessed via template", Example{
			Setup: func() {
				fakeClientset.CoreV1().Secrets("prefix-some-team").Create(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "some-pipeline." + secretName,
					},
					Data: map[string][]byte{
						"some-field": []byte("some-field-value"),
					},
				})
			},

			Template: "((" + secretName + ".some-field))",
			Result:   "some-field-value",
		}),
	)
})
