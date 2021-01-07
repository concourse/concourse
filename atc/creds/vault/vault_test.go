package vault_test

import (
	"encoding/json"
	"fmt"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/creds/vault"
	"github.com/concourse/concourse/vars"
	vaultapi "github.com/hashicorp/vault/api"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
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

func createMockV2Secret(value string) *vaultapi.Secret {
	secret := vaultapi.Secret{}
	json.Unmarshal([]byte(fmt.Sprintf(`{"data":{"value":"%s"},"metadata":{"created_time":"2021-01-06T22:32:10.969537Z","deletion_time":"","destroyed":false,"version":3}}`, value)), &secret.Data)

	return &secret
}

func createMockV1Secret(value string) *vaultapi.Secret {
	secret := vaultapi.Secret{}
	json.Unmarshal([]byte(fmt.Sprintf(`{"value":"%s"}`, value)), &secret.Data)

	return &secret
}

var _ = Describe("Vault", func() {

	var v *vault.Vault
	var variables vars.Variables
	var msr *MockSecretReader
	var varFoo vars.Reference

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
		varFoo = vars.Reference{Path: "foo"}
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
				value, found, err := variables.Get(vars.Reference{Path: "baz"})
				Expect(value).To(BeEquivalentTo("qux"))
				Expect(found).To(BeTrue())
				Expect(err).To(BeNil())
			})

			It("should find static secrets in the configured place", func() {
				value, found, err := variables.Get(vars.Reference{Path: "global"})
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

// The below tests use ghttp handlers to mock a real vault API to the api_client.
// AppendHandlers has the following behavior which make the tests a bit messy.

//The first incoming request is handled by the first handler in the list, the
//second by the second, etc...

// As such, if a url is requested out of order or not for the proper path, the
// test fails with a corresponding error.
// Its annoying, but it works (if theres a better solution feel free to refactor this)

var _ = Describe("Vault KV2", func() {

	var server *ghttp.Server
	var returnedMountInfo vaultapi.Secret
	var statusCodeOK int

	var v *vault.Vault
	var variables vars.Variables
	var vaultApi *vault.APIClient
	var varFoo vars.VariableDefinition

	BeforeEach(func() {
		server = ghttp.NewServer()

		var err error
		vaultApi, err = vault.NewAPIClient(lagertest.NewTestLogger("test"), "http://"+server.Addr(), vault.TLSConfig{}, vault.AuthConfig{}, "")
		Expect(err).To(BeNil())

		statusCodeOK = 200

		returnedMountInfo = vaultapi.Secret{}
		json.Unmarshal([]byte(`{"accessor":"kv_db2ac651","config":{"default_lease_ttl":0,"force_no_cache":false,"max_lease_ttl":0},"description":"A KV v2 Mount","external_entropy_access":false,"local":false,"options":{"version":"2"},"path":"concourse/","seal_wrap":false,"type":"kv","uuid":"40d031ff-8fed-2406-8965-39616be0bd42"}`), &returnedMountInfo.Data)

		p, _ := creds.BuildSecretTemplate("p", "/concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}")
		t, _ := creds.BuildSecretTemplate("t", "/concourse/{{.Team}}/{{.Secret}}")

		v = &vault.Vault{
			SecretReader:    vaultApi,
			Prefix:          "/concourse",
			LookupTemplates: []*creds.SecretTemplate{p, t},
			SharedPath:      "shared",
		}

		variables = creds.NewVariables(v, "team", "pipeline", false)
		varFoo = vars.VariableDefinition{Ref: vars.VariableReference{Path: "foo"}}
	})

	AfterEach(func() {
		server.Close()
	})

	Describe("Get()", func() {
		It("should get secret from pipeline", func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/team/pipeline/foo"),
					ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/concourse/data/team/pipeline/foo"),
					ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, createMockV2Secret("bar")),
				),
			)
			value, found, err := variables.Get(varFoo)
			Expect(value).To(BeEquivalentTo("bar"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should get secret from team", func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/team/pipeline/foo"),
					ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/concourse/data/team/pipeline/foo"),
					ghttp.RespondWith(404, ""),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/team/foo"),
					ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/concourse/data/team/foo"),
					ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, createMockV2Secret("bar")),
				),
			)
			value, found, err := variables.Get(varFoo)
			Expect(value).To(BeEquivalentTo("bar"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should get secret from shared", func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/team/pipeline/foo"),
					ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/concourse/data/team/pipeline/foo"),
					ghttp.RespondWith(404, ""),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/team/foo"),
					ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/concourse/data/team/foo"),
					ghttp.RespondWith(404, ""),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/shared/foo"),
					ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/concourse/data/shared/foo"),
					ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, createMockV2Secret("bar")),
				),
			)
			value, found, err := variables.Get(varFoo)
			Expect(value).To(BeEquivalentTo("bar"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should get secret from pipeline even its in shared", func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/team/pipeline/foo"),
					ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/concourse/data/team/pipeline/foo"),
					ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, createMockV2Secret("bar")),
				),
			)
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

				v.LookupTemplates = []*creds.SecretTemplate{a, b, c}
				variables = creds.NewVariables(v, "team", "pipeline", false)
			})

			It("should find pipeline secrets in the configured place", func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/place1/team/sub/pipeline/foo"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/data/place1/team/sub/pipeline/foo"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, createMockV2Secret("bar")),
					),
				)
				value, found, err := variables.Get(varFoo)
				Expect(value).To(BeEquivalentTo("bar"))
				Expect(found).To(BeTrue())
				Expect(err).To(BeNil())
			})

			It("should find team secrets in the configured place", func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/place1/team/sub/pipeline/baz"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/data/place1/team/sub/pipeline/baz"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/place2/team/baz"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/data/place2/team/baz"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, createMockV2Secret("qux")),
					),
				)
				value, found, err := variables.Get(vars.VariableDefinition{Ref: vars.VariableReference{Path: "baz"}})
				Expect(value).To(BeEquivalentTo("qux"))
				Expect(found).To(BeTrue())
				Expect(err).To(BeNil())
			})

			It("should find static secrets in the configured place", func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/place1/team/sub/pipeline/global"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/data/place1/team/sub/pipeline/global"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/place2/team/global"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/data/place2/team/global"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/place3/global"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/data/place3/global"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, createMockV2Secret("shared")),
					),
				)
				value, found, err := variables.Get(vars.VariableDefinition{Ref: vars.VariableReference{Path: "global"}})
				Expect(value).To(BeEquivalentTo("shared"))
				Expect(found).To(BeTrue())
				Expect(err).To(BeNil())
			})
		})

		Context("without shared", func() {
			BeforeEach(func() {
				p, _ := creds.BuildSecretTemplate("p", "/concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}")
				t, _ := creds.BuildSecretTemplate("t", "/concourse/{{.Team}}/{{.Secret}}")

				v.LookupTemplates = []*creds.SecretTemplate{p, t}
				variables = creds.NewVariables(v, "team", "pipeline", false)
			})

			It("should not get secret from root", func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/team/pipeline/foo"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/data/team/pipeline/foo"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/team/foo"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/data/team/foo"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/shared/foo"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/data/shared/foo"),
						ghttp.RespondWith(404, ""),
					),
					// These 2 should never be called.
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/foo"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/data/foo"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, createMockV2Secret("foo")),
					),
				)
				_, found, err := variables.Get(varFoo)
				Expect(found).To(BeFalse())
				Expect(err).To(BeNil())
			})
		})

		Context("allowRootPath", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/team/pipeline/foo"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/data/team/pipeline/foo"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/team/foo"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/data/team/foo"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/shared/foo"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/data/shared/foo"),
						ghttp.RespondWith(404, ""),
					),
					// These 2 should only be called for root.
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/foo"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/data/foo"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, createMockV2Secret("foo")),
					),
				)
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

var _ = Describe("Vault KV1", func() {

	var server *ghttp.Server
	var returnedMountInfo vaultapi.Secret
	var statusCodeOK int

	var v *vault.Vault
	var variables vars.Variables
	var vaultApi *vault.APIClient
	var varFoo vars.VariableDefinition

	BeforeEach(func() {
		server = ghttp.NewServer()

		var err error
		vaultApi, err = vault.NewAPIClient(lagertest.NewTestLogger("test"), "http://"+server.Addr(), vault.TLSConfig{}, vault.AuthConfig{}, "")
		Expect(err).To(BeNil())

		statusCodeOK = 200

		returnedMountInfo = vaultapi.Secret{}
		json.Unmarshal([]byte(`{"accessor":"kv_a92a6156","config":{"default_lease_ttl":0,"force_no_cache":false,"max_lease_ttl":0},"description":"","external_entropy_access":false,"local":false,"options":{"version":"1"},"path":"concourse/","seal_wrap":false,"type":"kv","uuid":"1e54b331-04a4-4f31-455c-48955e713e67"}`), &returnedMountInfo.Data)

		p, _ := creds.BuildSecretTemplate("p", "/concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}")
		t, _ := creds.BuildSecretTemplate("t", "/concourse/{{.Team}}/{{.Secret}}")

		v = &vault.Vault{
			SecretReader:    vaultApi,
			Prefix:          "/concourse",
			LookupTemplates: []*creds.SecretTemplate{p, t},
			SharedPath:      "shared",
		}

		variables = creds.NewVariables(v, "team", "pipeline", false)
		varFoo = vars.VariableDefinition{Ref: vars.VariableReference{Path: "foo"}}
	})

	AfterEach(func() {
		server.Close()
	})

	Describe("Get()", func() {
		It("should get secret from pipeline", func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/team/pipeline/foo"),
					ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/concourse/team/pipeline/foo"),
					ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, createMockV1Secret("bar")),
				),
			)
			value, found, err := variables.Get(varFoo)
			Expect(value).To(BeEquivalentTo("bar"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should get secret from team", func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/team/pipeline/foo"),
					ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/concourse/team/pipeline/foo"),
					ghttp.RespondWith(404, ""),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/team/foo"),
					ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/concourse/team/foo"),
					ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, createMockV1Secret("bar")),
				),
			)
			value, found, err := variables.Get(varFoo)
			Expect(value).To(BeEquivalentTo("bar"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should get secret from shared", func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/team/pipeline/foo"),
					ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/concourse/team/pipeline/foo"),
					ghttp.RespondWith(404, ""),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/team/foo"),
					ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/concourse/team/foo"),
					ghttp.RespondWith(404, ""),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/shared/foo"),
					ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/concourse/shared/foo"),
					ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, createMockV1Secret("bar")),
				),
			)
			value, found, err := variables.Get(varFoo)
			Expect(value).To(BeEquivalentTo("bar"))
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should get secret from pipeline even its in shared", func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/team/pipeline/foo"),
					ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/concourse/team/pipeline/foo"),
					ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, createMockV1Secret("bar")),
				),
			)
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

				v.LookupTemplates = []*creds.SecretTemplate{a, b, c}
				variables = creds.NewVariables(v, "team", "pipeline", false)
			})

			It("should find pipeline secrets in the configured place", func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/place1/team/sub/pipeline/foo"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/place1/team/sub/pipeline/foo"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, createMockV1Secret("bar")),
					),
				)
				value, found, err := variables.Get(varFoo)
				Expect(value).To(BeEquivalentTo("bar"))
				Expect(found).To(BeTrue())
				Expect(err).To(BeNil())
			})

			It("should find team secrets in the configured place", func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/place1/team/sub/pipeline/baz"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/place1/team/sub/pipeline/baz"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/place2/team/baz"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/place2/team/baz"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, createMockV1Secret("qux")),
					),
				)
				value, found, err := variables.Get(vars.VariableDefinition{Ref: vars.VariableReference{Path: "baz"}})
				Expect(value).To(BeEquivalentTo("qux"))
				Expect(found).To(BeTrue())
				Expect(err).To(BeNil())
			})

			It("should find static secrets in the configured place", func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/place1/team/sub/pipeline/global"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/place1/team/sub/pipeline/global"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/place2/team/global"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/place2/team/global"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/place3/global"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/place3/global"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, createMockV1Secret("shared")),
					),
				)
				value, found, err := variables.Get(vars.VariableDefinition{Ref: vars.VariableReference{Path: "global"}})
				Expect(value).To(BeEquivalentTo("shared"))
				Expect(found).To(BeTrue())
				Expect(err).To(BeNil())
			})
		})

		Context("without shared", func() {
			BeforeEach(func() {
				p, _ := creds.BuildSecretTemplate("p", "/concourse/{{.Team}}/{{.Pipeline}}/{{.Secret}}")
				t, _ := creds.BuildSecretTemplate("t", "/concourse/{{.Team}}/{{.Secret}}")

				v.LookupTemplates = []*creds.SecretTemplate{p, t}
				variables = creds.NewVariables(v, "team", "pipeline", false)
			})

			It("should not get secret from root", func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/team/pipeline/foo"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/team/pipeline/foo"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/team/foo"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/team/foo"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/shared/foo"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/shared/foo"),
						ghttp.RespondWith(404, ""),
					),
					// These 2 should never be called.
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/foo"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/foo"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, createMockV1Secret("foo")),
					),
				)
				_, found, err := variables.Get(varFoo)
				Expect(found).To(BeFalse())
				Expect(err).To(BeNil())
			})
		})

		Context("allowRootPath", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/team/pipeline/foo"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/team/pipeline/foo"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/team/foo"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/team/foo"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/shared/foo"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/shared/foo"),
						ghttp.RespondWith(404, ""),
					),
					// These 2 should only be called for root.
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/sys/internal/ui/mounts/concourse/foo"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/foo"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, createMockV1Secret("foo")),
					),
				)
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
