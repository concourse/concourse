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
	var varFoo vars.VariableDefinition

	BeforeEach(func() {

		msr = &MockSecretReader{&[]MockSecret{
			{
				path: "/concourse/team",
				secret: &vaultapi.Secret{
					Data: map[string]interface{}{"foo": "bar"},
				},
			}},
		}

		p, _ := creds.BuildSecretTemplate("p", "/concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}")
		t, _ := creds.BuildSecretTemplate("t", "/concourse/{{.Team}}/{{.Secret}}")

		v = &vault.Vault{
			SecretReader:    msr,
			Prefix:          "/concourse",
			LookupTemplates: []*creds.SecretTemplate{p, t},
			SharedPath:      "shared",
		}

		variables = creds.NewVariables(v, "team", "pipeline", false)
		varFoo = vars.VariableDefinition{Ref: vars.VariableReference{Name: "foo"}}
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
			value, found, err := variables.Get(varFoo)
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
			value, found, err := variables.Get(varFoo)
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
			value, found, err := variables.Get(varFoo)
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
			value, found, err := variables.Get(varFoo)
			Expect(value).To(BeEquivalentTo("bar"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		Context("with custom lookup templates", func() {
			BeforeEach(func() {
				a, _ := creds.BuildSecretTemplate("a", "/concourse/place1/{{.Team}}/sub/{{.Pipeline}}/{{.Secret}}")
				b, _ := creds.BuildSecretTemplate("b", "/concourse/place2/{{.Team}}/{{.Secret}}")
				c, _ := creds.BuildSecretTemplate("c", "/concourse/place3/{{.Secret}}")

				sr := &MockSecretReader{&[]MockSecret{
					{
						path: "/concourse/place1/team/sub/pipeline/foo",
						secret: &vaultapi.Secret{
							Data: map[string]interface{}{"value": "bar"},
						},
					},
					{
						path: "/concourse/place2/team/baz",
						secret: &vaultapi.Secret{
							Data: map[string]interface{}{"value": "qux"},
						},
					},
					{
						path: "/concourse/place3/global",
						secret: &vaultapi.Secret{
							Data: map[string]interface{}{"value": "shared"},
						},
					}},
				}

				v = &vault.Vault{
					SecretReader:    sr,
					Prefix:          "/concourse",
					LookupTemplates: []*creds.SecretTemplate{a, b, c},
				}

				variables = creds.NewVariables(v, "team", "pipeline", false)
			})

			It("should find pipeline secrets in the configured place", func() {
				value, found, err := variables.Get(varFoo)
				Expect(value).To(BeEquivalentTo("bar"))
				Expect(found).To(BeTrue())
				Expect(err).To(BeNil())
			})

			It("should find team secrets in the configured place", func() {
				value, found, err := variables.Get(vars.VariableDefinition{Ref: vars.VariableReference{Name: "baz"}})
				Expect(value).To(BeEquivalentTo("qux"))
				Expect(found).To(BeTrue())
				Expect(err).To(BeNil())
			})

			It("should find static secrets in the configured place", func() {
				value, found, err := variables.Get(vars.VariableDefinition{Ref: vars.VariableReference{Name: "global"}})
				Expect(value).To(BeEquivalentTo("shared"))
				Expect(found).To(BeTrue())
				Expect(err).To(BeNil())
			})
		})

		Context("without shared", func() {
			BeforeEach(func() {
				p, _ := creds.BuildSecretTemplate("p", "/concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}")
				t, _ := creds.BuildSecretTemplate("t", "/concourse/{{.Team}}/{{.Secret}}")

				v = &vault.Vault{
					SecretReader:    msr,
					Prefix:          "/concourse",
					LookupTemplates: []*creds.SecretTemplate{p, t},
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
				_, found, err := variables.Get(varFoo)
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
					_, found, err := variables.Get(varFoo)
					Expect(err).To(BeNil())
					Expect(found).To(BeTrue())
				})
			})

			Context("is false", func() {
				BeforeEach(func() {
					variables = creds.NewVariables(v, "team", "pipeline", false)
				})

				It("should not get secret from root", func() {
					_, found, err := variables.Get(varFoo)
					Expect(err).To(BeNil())
					Expect(found).To(BeFalse())
				})
			})
		})
	})
})
