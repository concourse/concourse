package k8s_test

import (
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("DNS Resolution", func() {

	var atc Endpoint

	BeforeEach(func() {
		setReleaseNameAndNamespace("dp")
	})

	AfterEach(func() {
		cleanupReleases()
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

	const containerdRuntime = "containerd"
	const guardianRuntime = "guardian"

	var setupDeployment = func(runtime string, dnsProxyEnable, dnsServer string) {
		args := []string{
			`--set=worker.replicas=1`,
			`--set-string=concourse.worker.runtime=` + runtime,
		}
		switch {
		case runtime == containerdRuntime:
			args = append(args, `--set-string=concourse.worker.containerd.dnsProxyEnable=`+dnsProxyEnable)
			if dnsServer != "" {
				args = append(args,
					`--set=concourse.worker.containerd.dnsServers=['`+dnsServer+`']`,
				)
			}
		case runtime == guardianRuntime:
			args = append(args, `--set-string=concourse.worker.garden.dnsProxyEnable=`+dnsProxyEnable)
			if dnsServer != "" {
				// garden flags aren't explicityly defined in the chart, so add them as env vars directly
				args = append(args,
					`--set=worker.env[0].name=CONCOURSE_GARDEN_DNS_SERVER`,
					`--set=worker.env[0].value=`+dnsServer)
			}
		default:
			log.Fatalf("Invalid runtime type %s. Test aborted.", runtime)
			return
		}

		deployConcourseChart(releaseName, args...)
		atc = waitAndLogin(namespace, releaseName+"-web")
	}

	expectedDnsProxyBehaviour := func(runtime string) {
		DescribeTable("different proxy settings",
			func(c Case) {
				setupDeployment(runtime, c.enableDnsProxy, c.dnsServer)

				sess := fly.Start("execute", "-c", "tasks/dns-proxy-task.yml", "-v", "url="+c.addressFunction())
				<-sess.Exited

				if !c.shouldWork {
					Expect(sess.ExitCode()).ToNot(BeZero())
					return
				}

				Expect(sess.ExitCode()).To(BeZero())
			},
			// local proxy will pass through requests to the k8s native dns
			Entry("Proxy Enabled, with full service name", Case{
				enableDnsProxy:  "true",
				addressFunction: fullAddress,
				shouldWork:      true,
			}),
			// the short address is expanded into the full addresses with the help of
			// the `search` configuration in the resolv.conf. But our implementation
			// of the proxy server kinda just ignores this configuration. So short
			// addresses won't ever work
			Entry("Proxy Enabled, with short service name", Case{
				enableDnsProxy:  "true",
				addressFunction: shortAddress,
				shouldWork:      false,
			}),
			// no dns proxy = the super powerful k8s native dns
			Entry("Proxy Disabled, with full service name", Case{
				enableDnsProxy:  "false",
				addressFunction: fullAddress,
				shouldWork:      true,
			}),
			// no dns proxy = the super powerful k8s native dns
			Entry("Proxy Disabled, with short service name", Case{
				enableDnsProxy:  "false",
				addressFunction: shortAddress,
				shouldWork:      true,
			}),
			// the public 8.8.8.8 server won't be able to resolve k8s addresses
			Entry("Adding extra dns server, with Proxy Disabled and full address", Case{
				enableDnsProxy:  "false",
				dnsServer:       "8.8.8.8",
				addressFunction: fullAddress,
				shouldWork:      false,
			}),
			// dns requests hits the 8.8.8.8 server first, as long as it doesn't time
			// out, it should fail to resolve and end the search chain, skipping the proxy
			Entry("Adding extra dns server, with Proxy Enabled and full address", Case{
				enableDnsProxy:  "true",
				dnsServer:       "8.8.8.8",
				addressFunction: fullAddress,
				shouldWork:      false,
			}),
		)
	}

	Context("with gdn backend", func() {
		expectedDnsProxyBehaviour(guardianRuntime)
	})

	Context("with containerd backend", func() {
		expectedDnsProxyBehaviour(containerdRuntime)
	})
})
