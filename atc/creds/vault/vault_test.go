package vault_test

import (
	"encoding/json"
	"regexp"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/creds/vault"
	"github.com/concourse/concourse/vars"
	vaultapi "github.com/hashicorp/vault/api"
	. "github.com/onsi/ginkgo/v2"
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
	return &vaultapi.Secret{
		Data: map[string]interface{}{
			"data": map[string]interface{}{"value": value},
			"metadata": map[string]interface{}{
				"created_time":  "2021-01-06T22:32:10.969537Z",
				"deletion_time": "",
				"destroyed":     false,
				"version":       3,
			},
		},
	}
}

func createMockV1Secret(value string) *vaultapi.Secret {
	return &vaultapi.Secret{
		Data: map[string]interface{}{
			"value": value,
		},
	}
}

var _ = Describe("Vault", func() {

	var v *vault.Vault
	var variables vars.Variables
	var msr *MockSecretReader
	var varFoo vars.Reference
	var loggedInCh chan struct{}

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

		loggedInCh = make(chan struct{}, 1)

		v = &vault.Vault{
			SecretReader:    msr,
			Prefix:          "/concourse",
			LookupTemplates: []*creds.SecretTemplate{p, t},
			SharedPath:      "shared",
			LoggedIn:        loggedInCh,
			LoginTimeout:    1 * time.Second,
		}

		variables = creds.NewVariables(v, "team", "pipeline", false)
		varFoo = vars.Reference{Path: "foo"}
	})

	Describe("Get()", func() {
		Context("when log in timeout", func() {
			It("should return a timeout error", func() {
				_, _, err := variables.Get(varFoo)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(vault.VaultLoginTimeout{}))
			})
		})

		Context("when logged in successfully", func() {
			BeforeEach(func() {
				close(loggedInCh)
			})

			It("should set expiration", func() {
				v.SecretReader = &MockSecretReader{&[]MockSecret{
					{
						path: "/concourse/team/pipeline/foo",
						secret: &vaultapi.Secret{
							Data: map[string]interface{}{"value": "bar"},
						},
					}},
				}
				value, expiration, found, err := v.Get("/concourse/team/pipeline/foo")
				Expect(value).To(BeEquivalentTo("bar"))
				Expect(expiration).ToNot(BeNil())
				Expect(found).To(BeTrue())
				Expect(err).To(BeNil())
			})

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
	var varFoo vars.Reference

	BeforeEach(func() {
		server = ghttp.NewServer()

		var err error
		vaultApi, err = vault.NewAPIClient(lagertest.NewTestLogger("test"), "http://"+server.Addr(), vault.ClientConfig{}, vault.TLSConfig{}, vault.AuthConfig{}, "", 0)
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

		mountPathRegex, _ := regexp.Compile("/v1/sys/internal/ui/mounts/.*")
		server.RouteToHandler("GET", mountPathRegex, ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", MatchRegexp("/v1/sys/internal/ui/mounts/.*")),
			ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
		))

		variables = creds.NewVariables(v, "team", "pipeline", false)
		varFoo = vars.Reference{Path: "foo"}
	})

	AfterEach(func() {
		server.Close()
	})

	Describe("Get()", func() {
		It("should not return an expiration", func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/concourse/data/team/pipeline/foo"),
					ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, createMockV2Secret("bar")),
				),
			)
			value, expiration, found, err := v.Get("team/pipeline/foo")
			Expect(value).To(BeEquivalentTo("bar"))
			Expect(expiration).To(BeNil())
			Expect(found).To(BeTrue())
			Expect(err).To(BeNil())
		})

		It("should get secret from pipeline", func() {
			server.AppendHandlers(
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
					ghttp.VerifyRequest("GET", "/v1/concourse/data/team/pipeline/foo"),
					ghttp.RespondWith(404, ""),
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
					ghttp.VerifyRequest("GET", "/v1/concourse/data/team/pipeline/foo"),
					ghttp.RespondWith(404, ""),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/concourse/data/team/foo"),
					ghttp.RespondWith(404, ""),
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
						ghttp.VerifyRequest("GET", "/v1/concourse/data/place1/team/sub/pipeline/baz"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/data/place2/team/baz"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, createMockV2Secret("qux")),
					),
				)
				value, found, err := variables.Get(vars.Reference{Path: "baz"})
				Expect(value).To(BeEquivalentTo("qux"))
				Expect(found).To(BeTrue())
				Expect(err).To(BeNil())
			})

			It("should find static secrets in the configured place", func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/data/place1/team/sub/pipeline/global"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/data/place2/team/global"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/data/place3/global"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, createMockV2Secret("shared")),
					),
				)
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

				v.LookupTemplates = []*creds.SecretTemplate{p, t}
				variables = creds.NewVariables(v, "team", "pipeline", false)
			})

			It("should not get secret from root", func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/data/team/pipeline/foo"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/data/team/foo"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/data/shared/foo"),
						ghttp.RespondWith(404, ""),
					),
					// This should never be called.
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
						ghttp.VerifyRequest("GET", "/v1/concourse/data/team/pipeline/foo"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/data/team/foo"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/data/shared/foo"),
						ghttp.RespondWith(404, ""),
					),
					// This should only be called for root.
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
	var varFoo vars.Reference

	BeforeEach(func() {
		server = ghttp.NewServer()

		var err error
		vaultApi, err = vault.NewAPIClient(lagertest.NewTestLogger("test"), "http://"+server.Addr(), vault.ClientConfig{}, vault.TLSConfig{}, vault.AuthConfig{}, "", 0)
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

		mountPathRegex, _ := regexp.Compile("/v1/sys/internal/ui/mounts/.*")
		server.RouteToHandler("GET", mountPathRegex, ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", MatchRegexp("/v1/sys/internal/ui/mounts/.*")),
			ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
		))

		variables = creds.NewVariables(v, "team", "pipeline", false)
		varFoo = vars.Reference{Path: "foo"}
	})

	AfterEach(func() {
		server.Close()
	})

	Describe("Get()", func() {
		It("should get secret from pipeline", func() {
			server.AppendHandlers(
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
					ghttp.VerifyRequest("GET", "/v1/concourse/team/pipeline/foo"),
					ghttp.RespondWith(404, ""),
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
					ghttp.VerifyRequest("GET", "/v1/concourse/team/pipeline/foo"),
					ghttp.RespondWith(404, ""),
				),
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/v1/concourse/team/foo"),
					ghttp.RespondWith(404, ""),
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
						ghttp.VerifyRequest("GET", "/v1/concourse/place1/team/sub/pipeline/baz"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/place2/team/baz"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, createMockV1Secret("qux")),
					),
				)
				value, found, err := variables.Get(vars.Reference{Path: "baz"})
				Expect(value).To(BeEquivalentTo("qux"))
				Expect(found).To(BeTrue())
				Expect(err).To(BeNil())
			})

			It("should find static secrets in the configured place", func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/place1/team/sub/pipeline/global"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/place2/team/global"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/place3/global"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, createMockV1Secret("shared")),
					),
				)
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

				v.LookupTemplates = []*creds.SecretTemplate{p, t}
				variables = creds.NewVariables(v, "team", "pipeline", false)
			})

			It("should not get secret from root", func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/team/pipeline/foo"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/team/foo"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/shared/foo"),
						ghttp.RespondWith(404, ""),
					),
					// This should never be called.
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
						ghttp.VerifyRequest("GET", "/v1/concourse/team/pipeline/foo"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/team/foo"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/concourse/shared/foo"),
						ghttp.RespondWith(404, ""),
					),
					// This should only be called for root.
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

var _ = Describe("Vault KV1 and KV2", func() {

	var server *ghttp.Server
	var returnedMountInfo vaultapi.Secret
	var returnedMountInfoKv1 vaultapi.Secret
	var returnedMountInfoKv2 vaultapi.Secret
	var statusCodeOK int

	var secretsFactory creds.SecretsFactory
	var variables vars.Variables
	var varFooKv1 vars.Reference
	var varFooKv2 vars.Reference

	BeforeEach(func() {
		server = ghttp.NewServer()

		//var manager vault.VaultManager
		manager := vault.VaultManager{
			URL:             "http://" + server.Addr(),
			Auth:            vault.AuthConfig{ClientToken: "xxx"},
			TLS:             vault.TLSConfig{},
			Namespace:       "",
			QueryTimeout:    2764800 * time.Second,
			LoginTimeout:    2764800 * time.Second,
			PathPrefix:      "",
			PathPrefixes:    []string{"/kv2", "/kv1"},
			SharedPath:      "shared",
			LookupTemplates: []string{"/{{.Team}}/{{.Pipeline}}/{{.Secret}}", "/{{.Team}}/{{.Secret}}"},
		}

		err := manager.Validate()
		Expect(err).To(BeNil())

		// set up Vault API client
		err = manager.Init(lagertest.NewTestLogger("test"))
		Expect(err).To(BeNil())

		// apparently we need to call NewSecretsFactory() so it will set manager.SecretFactory
		secretsFactory, err = manager.NewSecretsFactory(lagertest.NewTestLogger("test"))
		Expect(err).To(BeNil())

		statusCodeOK = 200

		returnedMountInfo = vaultapi.Secret{}
		_ = json.Unmarshal([]byte(`
		{
		    "auth": null,
		    "data": {
		        "auth": {},
		        "secret": {
		            "kv2/": {
		                "accessor": "kv2_4cezb9b5",
		                "config": {
		                    "default_lease_ttl": 2764800,
		                    "force_no_cache": false,
		                    "max_lease_ttl": 2764800
		                },
		                "description": "kv_v2 secret storage",
		                "external_entropy_access": false,
		                "local": false,
		                "options": {
		                    "version": "2"
		                },
		                "seal_wrap": false,
		                "type": "kv",
		                "uuid": "6235eb1c-db7b-4b0b-b60f-d91cdc6a5bc6"
		            },
		            "kv1/": {
		                "accessor": "kv1_69pda40e",
		                "config": {
		                    "default_lease_ttl": 2764800,
		                    "force_no_cache": false,
		                    "max_lease_ttl": 2764800
		                },
		                "description": "kv v1 secret storage",
		                "external_entropy_access": false,
		                "local": false,
		                "options": null,
		                "seal_wrap": false,
		                "type": "generic",
		                "uuid": "0d22d970-ab97-4586-985e-61a5dd24cbf4"
		            }
		        }
		    },
		    "lease_duration": 2764800,
		    "lease_id": "",
		    "renewable": false,
		    "request_id": "224dbe47-fe8c-500c-db23-1e1dd677b87c",
		    "warnings": null,
		    "wrap_info": null
		}`), &returnedMountInfo.Data)

		returnedMountInfoKv1 = vaultapi.Secret{}
		_ = json.Unmarshal([]byte(`{"accessor":"kv_db2ac651","config":{"default_lease_ttl":2764800,"force_no_cache":false,"max_lease_ttl":2764800},"description":"A KV v1 Mount","external_entropy_access":false,"local":false,"options":{"version":"1"},"path":"kv1/","seal_wrap":false,"type":"kv","uuid":"40d031ff-8fed-2406-8965-39616be0bd42"}`), &returnedMountInfoKv1.Data)

		returnedMountInfoKv2 = vaultapi.Secret{}
		_ = json.Unmarshal([]byte(`{"accessor":"kv_a92a6156","config":{"default_lease_ttl":2764800,"force_no_cache":false,"max_lease_ttl":2764800},"description":"A KV v2 Mount","external_entropy_access":false,"local":false,"options":{"version":"2"},"path":"kv2/","seal_wrap":false,"type":"kv_v2","uuid":"1e54b331-04a4-4f31-455c-48955e713e67"}`), &returnedMountInfoKv2.Data)

		mountPathRegex, _ := regexp.Compile("/v1/sys/internal/ui/mounts/$")
		server.RouteToHandler("GET", mountPathRegex, ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", MatchRegexp("/v1/sys/internal/ui/mounts/$")),
			ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfo),
		))

		mountPathRegexKv1, _ := regexp.Compile("/v1/sys/internal/ui/mounts/kv1/.*")
		server.RouteToHandler("GET", mountPathRegexKv1, ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", MatchRegexp("/v1/sys/internal/ui/mounts/kv1/.*")),
			ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfoKv1),
		))

		mountPathRegexKv2, _ := regexp.Compile("/v1/sys/internal/ui/mounts/kv2/.*")
		server.RouteToHandler("GET", mountPathRegexKv2, ghttp.CombineHandlers(
			ghttp.VerifyRequest("GET", MatchRegexp("/v1/sys/internal/ui/mounts/kv2/.*")),
			ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, &returnedMountInfoKv2),
		))

		variables = creds.NewVariables(secretsFactory.NewSecrets(), "team", "pipeline", false)
		varFooKv1 = vars.Reference{Path: "foo_kv1"}
		varFooKv2 = vars.Reference{Path: "foo_kv2"}
	})

	AfterEach(func() {
		server.Close()
	})

	Describe("Get()", func() {
		When("the secret is in the kv2 pipeline folder", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/kv2/data/team/pipeline/foo_kv2"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, createMockV2Secret("pipeline var kv2")),
					),
				)
			})
			It("should find the secret in kv2 pipeline folder", func() {
				value, found, err := variables.Get(varFooKv2)
				Expect(err).To(BeNil())
				Expect(found).To(BeTrue())
				Expect(value).To(BeEquivalentTo("pipeline var kv2"))
			})
		})

		When("the secret is in the kv2 team folder", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/kv2/data/team/pipeline/foo_kv2"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/kv2/data/team/foo_kv2"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, createMockV2Secret("team var kv2")),
					),
				)
			})
			It("should look in the kv2 pipeline folder and find the secret in the team folder", func() {
				value, found, err := variables.Get(varFooKv2)
				Expect(err).To(BeNil())
				Expect(found).To(BeTrue())
				Expect(value).To(BeEquivalentTo("team var kv2"))
			})
		})

		When("the secret is in the kv2 shared folder", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/kv2/data/team/pipeline/foo_kv2"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/kv2/data/team/foo_kv2"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/kv1/team/pipeline/foo_kv2"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/kv1/team/foo_kv2"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/kv2/data/shared/foo_kv2"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, createMockV2Secret("shared var kv2")),
					),
				)
			})
			It("should look in the kv2 and kv1 folders and find the secret in the kv2 shared folder", func() {
				value, found, err := variables.Get(varFooKv2)
				Expect(err).To(BeNil())
				Expect(found).To(BeTrue())
				Expect(value).To(BeEquivalentTo("shared var kv2"))
			})
		})

		When("the secret is in the kv1 pipeline folder", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/kv2/data/team/pipeline/foo_kv1"), ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/kv2/data/team/foo_kv1"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/kv1/team/pipeline/foo_kv1"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, createMockV1Secret("pipeline var kv1")),
					),
				)
			})
			It("should look in the kv2 folders and find the secret in the kv1 pipeline folder", func() {
				value, found, err := variables.Get(varFooKv1)
				Expect(err).To(BeNil())
				Expect(found).To(BeTrue())
				Expect(value).To(BeEquivalentTo("pipeline var kv1"))
			})
		})

		When("the secret is in the kv1 team folder", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/kv2/data/team/pipeline/foo_kv1"), ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/kv2/data/team/foo_kv1"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/kv1/team/pipeline/foo_kv1"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/kv1/team/foo_kv1"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, createMockV1Secret("team var kv1")),
					),
				)
			})
			It("should look in the kv2 folders, kv1 pipeline folder, and find the secret in the kv1 team folder", func() {
				value, found, err := variables.Get(varFooKv1)
				Expect(err).To(BeNil())
				Expect(found).To(BeTrue())
				Expect(value).To(BeEquivalentTo("team var kv1"))
			})
		})

		When("the secret is in the kv1 shared folder", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/kv2/data/team/pipeline/foo_kv1"), ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/kv2/data/team/foo_kv1"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/kv1/team/pipeline/foo_kv1"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/kv1/team/foo_kv1"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/kv2/data/shared/foo_kv1"),
						ghttp.RespondWith(404, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/v1/kv1/shared/foo_kv1"),
						ghttp.RespondWithJSONEncodedPtr(&statusCodeOK, createMockV1Secret("shared var kv1")),
					),
				)
			})
			It("should look in the kv2 and kv2 folders, kv1 shared folder, and find the secret in the kv1 shared folder", func() {
				value, found, err := variables.Get(varFooKv1)
				Expect(err).To(BeNil())
				Expect(found).To(BeTrue())
				Expect(value).To(BeEquivalentTo("shared var kv1"))
			})
		})
	})
})
