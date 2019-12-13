package k8s_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("DNS Resolution", func() {

	var atc Endpoint

	BeforeEach(func() {
		setReleaseNameAndNamespace("dp")
	})

	var setupDeployment = func(dnsProxyEnable, dnsServer string) {
		args := []string{
			`--set=worker.replicas=1`,
			`--set-string=concourse.worker.garden.dnsProxyEnable=` + dnsProxyEnable,
		}
		if dnsServer != "" {
			args = append(args,
				`--set=worker.env[0].name=CONCOURSE_GARDEN_DNS_SERVER`,
				`--set=worker.env[0].value=`+dnsServer)
		}

		deployConcourseChart(releaseName, args...)
		atc = waitAndLogin(namespace, releaseName+"-web")
	}

	AfterEach(func() {
		cleanup(releaseName, namespace)
		atc.Close()
	})

	var fullAddress = func() string {
		return releaseName + "-web." + namespace + ".svc.cluster.local:8080/api/v1/info"
	}

	var shortAddress = func() string {
		return releaseName + "-web:8080/api/v1/info"
	}

	type Case struct {
		// args
		enableDnsProxy  string
		dnsServer       string
		addressFunction func() string

		// expectations
		shouldWork bool
	}

	DescribeTable("different proxy settings",
		func(c Case) {
			setupDeployment(c.enableDnsProxy, c.dnsServer)

			sess := fly.Start("execute", "-c", "../tasks/dns-proxy-task.yml", "-v", "url="+c.addressFunction())
			<-sess.Exited

			if !c.shouldWork {
				Expect(sess.ExitCode()).ToNot(BeZero())
				return
			}

			Expect(sess.ExitCode()).To(BeZero())
		},
		Entry("Proxy Enabled, with full service name", Case{
			enableDnsProxy:  "true",
			addressFunction: fullAddress,
			shouldWork:      true,
		}),
		Entry("Proxy Enabled, with short service name", Case{
			enableDnsProxy:  "true",
			addressFunction: shortAddress,
			shouldWork:      false,
		}),
		Entry("Proxy Disabled, with full service name", Case{
			enableDnsProxy:  "false",
			addressFunction: fullAddress,
			shouldWork:      true,
		}),
		Entry("Proxy Disabled, with short service name", Case{
			enableDnsProxy:  "false",
			addressFunction: shortAddress,
			shouldWork:      true,
		}),
		Entry("Adding extra dns server, with Proxy Disabled and full address", Case{
			enableDnsProxy:  "false",
			dnsServer:       "8.8.8.8",
			addressFunction: fullAddress,
			shouldWork:      false,
		}),
		Entry("Adding extra dns server, with Proxy Enabled and full address", Case{
			enableDnsProxy:  "true",
			dnsServer:       "8.8.8.8",
			addressFunction: fullAddress,
			shouldWork:      false,
		}),
	)
})
