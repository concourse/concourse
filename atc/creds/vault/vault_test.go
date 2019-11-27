package vault_test

import (
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/creds/vault"
	"github.com/concourse/concourse/vars"
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
	var variables vars.Variables
	var msr *MockSecretReader

	BeforeEach(func() {

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

		variables = creds.NewVariables(v, "team", "pipeline", false)
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
			value, found, err := variables.Get(vars.VariableDefinition{Name: "foo"})
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
			value, found, err := variables.Get(vars.VariableDefinition{Name: "foo"})
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
			value, found, err := variables.Get(vars.VariableDefinition{Name: "foo"})
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
			value, found, err := variables.Get(vars.VariableDefinition{Name: "foo"})
			Expect(value).To(BeEquivalentTo("bar"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		Context("without shared", func() {
			BeforeEach(func() {
				v = &vault.Vault{
					SecretReader: msr,
					Prefix:       "/concourse",
				}

				variables = creds.NewVariables(v, "team", "pipeline", false)
			})

			It("should not get secret from root", func() {
				v.SecretReader = &MockSecretReader{&[]MockSecret{
					{
						path: "/concourse/foo",
						secret: &vaultapi.Secret{
							Data: map[string]interface{}{"value": "foo"},
						},
					}},
				}
				_, found, err := variables.Get(vars.VariableDefinition{Name: "foo"})
				Expect(found).To(BeFalse())
				Expect(err).To(BeNil())
			})
		})

		Context("allowRootPath", func() {
			BeforeEach(func() {
				v.SecretReader = &MockSecretReader{&[]MockSecret{
					{
						path: "/concourse/foo",
						secret: &vaultapi.Secret{
							Data: map[string]interface{}{"value": "foo"},
						},
					}},
				}
			})

			Context("is true", func() {
				BeforeEach(func() {
					variables = creds.NewVariables(v, "team", "pipeline", true)
				})

				It("should get secret from root", func() {
					_, found, err := variables.Get(vars.VariableDefinition{Name: "foo"})
					Expect(err).To(BeNil())
					Expect(found).To(BeTrue())
				})
			})

			Context("is false", func() {
				BeforeEach(func() {
					variables = creds.NewVariables(v, "team", "pipeline", false)
				})

				It("should not get secret from root", func() {
					_, found, err := variables.Get(vars.VariableDefinition{Name: "foo"})
					Expect(err).To(BeNil())
					Expect(found).To(BeFalse())
				})
			})
		})

		Context("skipTeamPath", func() {
			BeforeEach(func() {

				msr = &MockSecretReader{&[]MockSecret{
					{
						path: "/concourse/team/foo",
						secret: &vaultapi.Secret{
							Data: map[string]interface{}{"value": "team-bar"},
						},
					},
					{
						path: "/concourse/team/pipeline/foo",
						secret: &vaultapi.Secret{
							Data: map[string]interface{}{"value": "pipeline-bar"},
						},
					},
					{
						path: "/concourse/shared/foo",
						secret: &vaultapi.Secret{
							Data: map[string]interface{}{"value": "shared-bar"},
						},
					},
				}}

				v = &vault.Vault{
					SecretReader: msr,
					Prefix:       "/concourse",
					SharedPath:   "shared",
					SkipTeamPath: true,
				}

				variables = creds.NewVariables(v, "team", "pipeline", false)
			})

			It("should get secret from shared", func() {
				value, found, err := variables.Get(vars.VariableDefinition{Name: "foo"})
				Expect(value).To(BeEquivalentTo("shared-bar"))
				Expect(found).To(BeTrue())
				Expect(err).To(BeNil())
			})
		})
	})
})
