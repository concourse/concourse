package vars_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/concourse/concourse/vars"
)

var _ = Describe("Template", func() {
	It("can interpolate values into a struct with byte slice", func() {
		template := NewTemplate([]byte("((key))"))
		vars := StaticVariables{"key": "foo"}

		result, err := template.Evaluate(vars, EvaluateOpts{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte("foo\n")))
	})

	It("can interpolate values with leading slash into a struct with byte slice", func() {
		template := NewTemplate([]byte("((/key/foo))"))
		vars := StaticVariables{"/key/foo": "foo"}

		result, err := template.Evaluate(vars, EvaluateOpts{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte("foo\n")))
	})

	It("can interpolate multiple values into a byte slice", func() {
		template := NewTemplate([]byte("((key)): ((value))"))
		vars := StaticVariables{
			"key":   "foo",
			"value": "bar",
		}

		result, err := template.Evaluate(vars, EvaluateOpts{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte("foo: bar\n")))
	})

	It("can interpolate boolean values into a byte slice", func() {
		template := NewTemplate([]byte("otherstuff: ((boule))"))
		vars := StaticVariables{"boule": true}

		result, err := template.Evaluate(vars, EvaluateOpts{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte("otherstuff: true\n")))
	})

	It("can interpolate a different data types into a byte slice", func() {
		hashValue := map[string]interface{}{"key2": []string{"value1", "value2"}}
		template := NewTemplate([]byte("name1: ((name1))\nname2: ((name2))\nname3: ((name3))\nname4: ((name4))\nname5: ((name5))\nname6: ((name6))\n1234: value\n"))
		vars := StaticVariables{
			"name1": 1,
			"name2": "nil",
			"name3": true,
			"name4": "",
			"name5": nil,
			"name6": map[string]interface{}{"key": hashValue},
		}

		result, err := template.Evaluate(vars, EvaluateOpts{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte(`1234: value
name1: 1
name2: nil
name3: true
name4: ""
name5: null
name6:
  key:
    key2:
    - value1
    - value2
`)))
	})

	It("return errors if there are missing variable keys and ExpectAllKeys is true", func() {
		template := NewTemplate([]byte(`
((key4))_array:
- ((key_in_array))
((key)): ((key2))
((key3)): 2
dup-key: ((key3))
`))
		vars := StaticVariables{"key3": "foo"}

		_, err := template.Evaluate(vars, EvaluateOpts{ExpectAllKeys: true})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("undefined vars: key, key2, key4, key_in_array"))
	})

	It("does not return error if there are missing variable keys and ExpectAllKeys is false", func() {
		template := NewTemplate([]byte("((key)): ((key2))\n((key3)): 2"))
		vars := StaticVariables{"key3": "foo"}

		result, err := template.Evaluate(vars, EvaluateOpts{})
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal([]byte("((key)): ((key2))\nfoo: 2\n")))
	})

	It("return errors if there are unused variable keys and ExpectAllVarsUsed is true", func() {
		template := NewTemplate([]byte("((key2))"))
		vars := StaticVariables{"key1": "1", "key2": "2", "key3": "3"}

		_, err := template.Evaluate(vars, EvaluateOpts{ExpectAllVarsUsed: true})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("unused vars: key1, key3"))
	})

	It("does not return error if there are unused variable keys and ExpectAllVarsUsed is false", func() {
		template := NewTemplate([]byte("((key)): ((key2))"))
		vars := StaticVariables{"key3": "foo"}

		result, err := template.Evaluate(vars, EvaluateOpts{})
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal([]byte("((key)): ((key2))\n")))
	})

	It("return errors if there are not found and unused variables and ExpectAllKeys and ExpectAllVarsUsed are true", func() {
		template := NewTemplate([]byte("((key2))"))
		vars := StaticVariables{"key1": "1", "key3": "3"}

		_, err := template.Evaluate(vars, EvaluateOpts{ExpectAllKeys: true, ExpectAllVarsUsed: true})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("undefined vars: key2"))
		Expect(err.Error()).To(ContainSubstring("unused vars: key1, key3"))
	})

	Context("When template is a number", func() {
		It("returns it", func() {
			template := NewTemplate([]byte(`1234`))
			vars := StaticVariables{"key": "not key"}

			result, err := template.Evaluate(vars, EvaluateOpts{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal([]byte("1234\n")))
		})
	})

	Context("When variable has nil as value for key", func() {
		It("uses null", func() {
			template := NewTemplate([]byte("((key)): value"))
			vars := StaticVariables{"key": nil}

			result, err := template.Evaluate(vars, EvaluateOpts{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal([]byte("null: value\n")))
		})
	})

	It("can interpolate unicode values into a byte slice", func() {
		template := NewTemplate([]byte("((Ω))"))
		vars := StaticVariables{"Ω": "☃"}

		result, err := template.Evaluate(vars, EvaluateOpts{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte("☃\n")))
	})

	It("can interpolate keys with dashes and underscores into a byte slice", func() {
		template := NewTemplate([]byte("((with-a-dash)): ((with_an_underscore))"))
		vars := StaticVariables{
			"with-a-dash":        "dash",
			"with_an_underscore": "underscore",
		}

		result, err := template.Evaluate(vars, EvaluateOpts{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte("dash: underscore\n")))
	})

	It("can interpolate keys with dot and colon into a byte slice", func() {
		template := NewTemplate([]byte("bar: ((foo:\"with.dot:colon\".buzz))"))
		vars := NamedVariables{
			"foo": StaticVariables{
				"with.dot:colon": map[string]interface{}{
					"buzz": "fuzz",
				},
			},
		}

		result, err := template.Evaluate(vars, EvaluateOpts{})
		Expect(err).NotTo(HaveOccurred())
		Expect(string(result)).To(Equal(string([]byte("bar: fuzz\n"))))
	})

	It("can interpolate a secret key in the middle of a string", func() {
		template := NewTemplate([]byte("url: https://((ip))"))
		vars := StaticVariables{
			"ip": "10.0.0.0",
		}

		result, err := template.Evaluate(vars, EvaluateOpts{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte("url: https://10.0.0.0\n")))
	})

	It("can interpolate multiple secret keys in the middle of a string", func() {
		template := NewTemplate([]byte("uri: nats://nats:((password))@((ip)):4222"))
		vars := StaticVariables{
			"password": "secret",
			"ip":       "10.0.0.0",
		}

		result, err := template.Evaluate(vars, EvaluateOpts{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte("uri: nats://nats:secret@10.0.0.0:4222\n")))
	})

	It("can interpolate multiple keys of type string and int in the middle of a string", func() {
		template := NewTemplate([]byte("address: ((ip)):((port))"))
		vars := StaticVariables{
			"port": 4222,
			"ip":   "10.0.0.0",
		}

		result, err := template.Evaluate(vars, EvaluateOpts{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte("address: 10.0.0.0:4222\n")))
	})

	It("raises error when interpolating an unsupported type in the middle of a string", func() {
		template := NewTemplate([]byte("address: ((definition)):((eulers_number))"))
		vars := StaticVariables{
			"eulers_number": 2.717,
			"definition":    "natural_log",
		}

		_, err := template.Evaluate(vars, EvaluateOpts{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("float64"))
		Expect(err.Error()).To(ContainSubstring("eulers_number"))
	})

	It("can interpolate a single key multiple times in the middle of a string", func() {
		template := NewTemplate([]byte("acct_and_password: ((user)):((user))"))
		vars := StaticVariables{
			"user": "nats",
		}

		result, err := template.Evaluate(vars, EvaluateOpts{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte("acct_and_password: nats:nats\n")))
	})

	It("can interpolate values into the middle of a key", func() {
		template := NewTemplate([]byte("((iaas))_cpi: props"))
		vars := StaticVariables{
			"iaas": "aws",
		}

		result, err := template.Evaluate(vars, EvaluateOpts{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte("aws_cpi: props\n")))
	})

	It("can interpolate the same value multiple times into a byte slice", func() {
		template := NewTemplate([]byte("((key)): ((key))"))
		vars := StaticVariables{"key": "foo"}

		result, err := template.Evaluate(vars, EvaluateOpts{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte("foo: foo\n")))
	})

	It("can interpolate values with strange newlines", func() {
		template := NewTemplate([]byte("((key))"))
		vars := StaticVariables{"key": "this\nhas\nmany\nlines"}

		result, err := template.Evaluate(vars, EvaluateOpts{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte("|-\n  this\n  has\n  many\n  lines\n")))
	})

	It("ignores if operation is not specified", func() {
		template := NewTemplate([]byte("((key))"))
		vars := StaticVariables{"key": "val"}

		result, err := template.Evaluate(vars, EvaluateOpts{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte("val\n")))
	})

	It("ignores an invalid input", func() {
		template := NewTemplate([]byte("(()"))
		vars := StaticVariables{}

		result, err := template.Evaluate(vars, EvaluateOpts{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte("(()\n")))
	})

	It("allow var_source name in variable keys", func() {
		template := NewTemplate([]byte("abc: ((dummy:key))"))
		vars := NamedVariables{
			"dummy": StaticVariables{"key": "val"},
		}

		result, err := template.Evaluate(vars, EvaluateOpts{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte("abc: val\n")))
	})

	It("allows to access sub key of an interpolated value via dot syntax", func() {
		template := NewTemplate([]byte("((key.subkey))"))
		vars := StaticVariables{
			"key": map[interface{}]interface{}{"subkey": "e"},
		}

		result, err := template.Evaluate(vars, EvaluateOpts{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte("e\n")))
	})

	It("returns an error if variable is not found and is being used with a sub key", func() {
		template := NewTemplate([]byte("((key.subkey_not_found))"))
		vars := StaticVariables{}

		_, err := template.Evaluate(vars, EvaluateOpts{ExpectAllKeys: true})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("undefined vars: key"))
	})

	It("returns an error if accessing sub key of an interpolated value fails", func() {
		template := NewTemplate([]byte("((key.subkey_not_found))"))
		vars := StaticVariables{
			"key": map[interface{}]interface{}{"subkey": "e"},
		}

		_, err := template.Evaluate(vars, EvaluateOpts{ExpectAllKeys: true})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("missing field 'subkey_not_found' in var: key.subkey_not_found"))
	})

	It("returns error if finding variable fails", func() {
		template := NewTemplate([]byte("((key))"))
		vars := &FakeVariables{GetErr: errors.New("fake-err")}

		_, err := template.Evaluate(vars, EvaluateOpts{})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("fake-err"))
	})
})
