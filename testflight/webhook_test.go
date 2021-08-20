package testflight_test

import (
	"bytes"
	"net/http"
	"regexp"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Team-scoped webhooks", func() {
	It("should create checks for resources subscribed to webhooks", func() {
		setAndUnpausePipeline("fixtures/webhook.yml")

		hasCheck := func() bool {
			sess := fly("builds")
			return bytes.Contains(sess.Buffer().Contents(), []byte(inPipeline("some-resource/check")))
		}

		sess := fly("set-webhook",
			"--webhook", "some-webhook",
			"--type", "some-type",
			"-n",
		)

		var webhookURL string
		{
			webhookURLRegex := regexp.MustCompile(`(?m:the following webhook URL:\s*((?:https?)://\S+))`)
			submatch := webhookURLRegex.FindSubmatch(sess.Buffer().Contents())
			Expect(submatch).To(HaveLen(2))
			webhookURL = string(submatch[1])
		}

		By("emitting a webhook response that doesn't match the filter")
		resp, err := http.Post(webhookURL, "application/json", strings.NewReader(`{"unexpected": "payload"}`))
		Expect(err).ToNot(HaveOccurred())
		resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		Consistently(hasCheck, 5*time.Second, 1*time.Second).Should(BeFalse())

		By("emitting a webhook response that matches the filter")
		resp, err = http.Post(webhookURL, "application/json", strings.NewReader(`{"expected": "payload"}`))
		Expect(err).ToNot(HaveOccurred())
		resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusCreated))

		Expect(hasCheck()).To(BeTrue())
	})
})
