package atc_test

import (
	"fmt"
	"github.com/concourse/concourse/atc"
	"github.com/jessevdk/go-flags"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
)

const content = "__content__"

var _ = Describe("PathFlag", func() {
	Describe("IsURL", func() {
		It("return true when path is a http url", func() {
			path := atc.PathFlag(" http://a.com ")
			Expect(path.IsURL()).To(BeTrue())
		})

		It("return true when path is a https url", func() {
			path := atc.PathFlag(" https://a.com ")
			Expect(path.IsURL()).To(BeTrue())
		})

		It("return false when path is a file path", func() {
			path := atc.PathFlag("a/b.txt")
			Expect(path.IsURL()).ToNot(BeTrue())
		})
	})

	Describe("ReadContent", func() {
		Context("local file", func() {
			var (
				tempFile *os.File
				err      error
			)
			BeforeEach(func() {
				tempFile, err = ioutil.TempFile("", "path-flag-test")
				Expect(err).To(BeNil())
				_, err = tempFile.Write([]byte(content))
				Expect(err).ToNot(HaveOccurred())
				tempFile.Close()
			})
			AfterEach(func() {
				if tempFile != nil {
					err = os.Remove(tempFile.Name())
					Expect(err).ToNot(HaveOccurred())
				}
			})
			It("should be read correctly", func() {
				path := atc.PathFlag(tempFile.Name())
				content, err := path.ReadContent()
				Expect(err).ToNot(HaveOccurred())
				Expect(content).To(Equal([]byte(content)))
			})
		})

		Context("http url", func() {
			var tempServer *httptest.Server
			BeforeEach(func() {
				tempServer = httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
					res.Write([]byte(content))
				}))
			})
			AfterEach(func() {
				if tempServer != nil {
					tempServer.Close()
				}
			})
			It("should be read correctly", func() {
				path := atc.PathFlag(tempServer.URL)
				content, err := path.ReadContent()
				Expect(err).ToNot(HaveOccurred())
				Expect(content).To(Equal([]byte(content)))
			})
			It("should fail upon invalid url", func() {
				path := atc.PathFlag("http://fake.mycompany.com/aa.yml")
				_, err := path.ReadContent()
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("UnmarshalFlag", func() {
		var (
			path         atc.PathFlag
			tempDir      string
			err          error
			file1, file2 string
		)
		BeforeEach(func() {
			path = atc.PathFlag("")

			tempDir, err = ioutil.TempDir("", "path_flag_test_unmarshalflag")
			Expect(err).ToNot(HaveOccurred())

			file1 = fmt.Sprintf("%s/aab", tempDir)
			err = ioutil.WriteFile(file1, []byte("h"), os.ModePerm)
			Expect(err).ToNot(HaveOccurred())

			file2 = fmt.Sprintf("%s/aac", tempDir)
			err = ioutil.WriteFile(file2, []byte("h"), os.ModePerm)
			Expect(err).To(BeNil())
		})
		AfterEach(func() {
			err = os.RemoveAll(tempDir)
			Expect(err).ToNot(HaveOccurred())
		})
		It("should handle url", func() {
			err = path.UnmarshalFlag("http://a.com")
			Expect(err).ToNot(HaveOccurred())
			Expect(string(path)).To(Equal("http://a.com"))
		})
		It("should handle local file", func() {
			err = path.UnmarshalFlag(file1)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(path)).To(Equal(file1))
		})
		It("should fail if file not exist", func() {
			err = path.UnmarshalFlag("dummy_file")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("path 'dummy_file' does not exist"))
			Expect(string(path)).To(Equal(""))
		})
		It("should fail if filename is incomplete", func() {
			f := fmt.Sprintf("%s/aa*", tempDir)
			err = path.UnmarshalFlag(f)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("path '%s' resolves to multiple entries: %s, %s", f, file1, file2)))
			Expect(string(path)).To(Equal(""))
		})
	})

	Describe("Complete", func() {
		var (
			path         atc.PathFlag
			tempDir      string
			err          error
			file1, file2 string
		)
		BeforeEach(func() {
			path = atc.PathFlag("")

			tempDir, err = ioutil.TempDir("", "path_flag_test_complete")
			Expect(err).ToNot(HaveOccurred())

			file1 = fmt.Sprintf("%s/aab", tempDir)
			err = ioutil.WriteFile(file1, []byte("h"), os.ModePerm)
			Expect(err).ToNot(HaveOccurred())

			file2 = fmt.Sprintf("%s/aac", tempDir)
			err = ioutil.WriteFile(file2, []byte("h"), os.ModePerm)
			Expect(err).To(BeNil())
		})
		AfterEach(func() {
			err = os.RemoveAll(tempDir)
			Expect(err).ToNot(HaveOccurred())
		})
		It("should handle url", func() {
			c := path.Complete("http://a.com")
			Expect(c).To(Equal([]flags.Completion{}))
		})
		It("should handle local file", func() {
			c := path.Complete(file1)
			Expect(c).To(Equal([]flags.Completion{
				flags.Completion{Item: file1},
			}))
		})
		It("should handle multi-matches", func() {
			f := fmt.Sprintf("%s/aa", tempDir)
			c := path.Complete(f)
			Expect(c).To(Equal([]flags.Completion{
				flags.Completion{Item: file1},
				flags.Completion{Item: file2},
			}))
		})
	})
})
