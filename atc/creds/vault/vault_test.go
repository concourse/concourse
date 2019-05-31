package vault_test

import (
	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/concourse/v5/atc/creds"
	"github.com/concourse/concourse/v5/atc/creds/vault"
	vaultapi "github.com/hashicorp/vault/api"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type MockSecret struct {
	path   string
	secret *vaultapi.Secret
}

type MockSecretReader struct {
	secrets *[]MockSecret
}

func (msr *MockSecretReader) Read(lookupPath string) (*vaultapi.Secret, error) {
	Expect(lookupPath).ToNot(BeNil())

	for _, secret := range *msr.secrets {
		if lookupPath == secret.path {
			return secret.secret, nil
		}
	}

	return nil, nil
}

var _ = Describe("Vault", func() {

	var v *vault.Vault
	var variables creds.Variables
	var msr *MockSecretReader

	JustBeforeEach(func() {

		msr = &MockSecretReader{&[]MockSecret{
			{
				path: "/concourse/team",
				secret: &vaultapi.Secret{
					Data: map[string]interface{}{"foo": "bar"},
				},
			}},
		}

		v = &vault.Vault{
			SecretReader: msr,
			Prefix:       "/concourse",
			SharedPath:   "shared",
		}

		variables = creds.NewVariables(v, "team", "pipeline")
	})

	Describe("Get()", func() {
		It("should get secret from pipeline", func() {
			v.SecretReader = &MockSecretReader{&[]MockSecret{
				{
					path: "/concourse/team/pipeline/foo",
					secret: &vaultapi.Secret{
						Data: map[string]interface{}{"value": "bar"},
					},
				}},
			}
			value, found, err := variables.Get(template.VariableDefinition{Name: "foo"})
			Expect(value).To(BeEquivalentTo("bar"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should get secret from team", func() {
			v.SecretReader = &MockSecretReader{&[]MockSecret{
				{
					path: "/concourse/team/foo",
					secret: &vaultapi.Secret{
						Data: map[string]interface{}{"value": "bar"},
					},
				}},
			}
			value, found, err := variables.Get(template.VariableDefinition{Name: "foo"})
			Expect(value).To(BeEquivalentTo("bar"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should get secret from shared", func() {
			v.SecretReader = &MockSecretReader{&[]MockSecret{
				{
					path: "/concourse/shared/foo",
					secret: &vaultapi.Secret{
						Data: map[string]interface{}{"value": "bar"},
					},
				}},
			}
			value, found, err := variables.Get(template.VariableDefinition{Name: "foo"})
			Expect(value).To(BeEquivalentTo("bar"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should get secret from pipeline even its in shared", func() {
			v.SecretReader = &MockSecretReader{&[]MockSecret{
				{
					path: "/concourse/shared/foo",
					secret: &vaultapi.Secret{
						Data: map[string]interface{}{"value": "foo"},
					},
				},
				{
					path: "/concourse/team/foo",
					secret: &vaultapi.Secret{
						Data: map[string]interface{}{"value": "bar"},
					},
				}},
			}
			value, found, err := variables.Get(template.VariableDefinition{Name: "foo"})
			Expect(value).To(BeEquivalentTo("bar"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})
	})
})
