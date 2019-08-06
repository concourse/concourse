package kubernetes_test

import (
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/creds/kubernetes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var _ = Describe("Kubernetes", func() {
	var fakeClientset *fake.Clientset
	var k creds.Secrets

	BeforeEach(func() {
		fakeClientset = fake.NewSimpleClientset()

		factory := kubernetes.NewKubernetesFactory(lagertest.NewTestLogger("test"), fakeClientset, "namespace")
		k = factory.NewSecrets()
	})

	Describe("NewSecretLookupPaths()", func() {
		It("should transform variable names to kubernetes secret names correctly", func() {
			pathObjects := k.NewSecretLookupPaths("team", "pipeline")
			var paths []string
			for _, p := range pathObjects {
				path, err := p.VariableToSecretPath("variable")
				Expect(err).To(BeNil())
				paths = append(paths, path)
			}

			Expect(len(paths)).To(BeEquivalentTo(2))
			Expect(paths).To(ContainElement("namespaceteam:pipeline.variable"))
			Expect(paths).To(ContainElement("namespaceteam:variable"))
		})
	})

	Describe("Get", func() {
		var secretNamespace string
		var secretName string

		var res interface{}
		var exp *time.Time
		var found bool
		var err error

		BeforeEach(func() {
			secretNamespace = "foo"
			secretName = "some-secret-name"
		})

		JustBeforeEach(func() {
			res, exp, found, err = k.Get(secretNamespace + ":" + secretName)
		})

		Context("when a secret has a value field", func() {
			BeforeEach(func() {
				fakeClientset.CoreV1().Secrets(secretNamespace).Create(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: secretName,
					},
					Data: map[string][]byte{
						"value": []byte("some-value"),
					},
				})
			})

			It("returns the value instead of the full secret", func() {
				Expect(res).To(Equal("some-value"))
				Expect(exp).To(BeNil())
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when the secret has multiple fields", func() {
			BeforeEach(func() {
				fakeClientset.CoreV1().Secrets("foo").Create(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: secretName,
					},
					Data: map[string][]byte{
						"key-a": []byte("value-a"),
						"key-b": []byte("value-b"),
					},
				})
			})

			It("returns the appropriate type for templating", func() {
				Expect(res).To(Equal(map[string]interface{}{
					"key-a": "value-a",
					"key-b": "value-b",
				}))
				Expect(exp).To(BeNil())
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when the secret is not found", func() {
			It("returns false", func() {
				Expect(res).To(BeNil())
				Expect(exp).To(BeNil())
				Expect(found).To(BeFalse())
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
