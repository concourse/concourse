//go:build linux

package baggageclaimcmd_test

import (
	"os/exec"

	"code.cloudfoundry.org/lager/v3/lagertest"
	"github.com/concourse/concourse/worker/baggageclaim/baggageclaimcmd"
	"github.com/concourse/concourse/worker/baggageclaim/uidgid"
	bespec "github.com/concourse/concourse/worker/runtime/spec"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Baggage Claim Command Runner", func() {
	Describe("Namespace Selector", func() {
		logger := lagertest.NewTestLogger("test")
		Context("when user namespaces are disabled", func() {
			var cmd baggageclaimcmd.BaggageclaimCommand
			cmd.DisableUserNamespaces = true
			It("selects the NoopNamespacer", func() {
				priv, unpriv := cmd.SelectNamespacers(logger)
				Expect(priv).To(Equal(uidgid.NoopNamespacer{}))
				Expect(unpriv).To(Equal(uidgid.NoopNamespacer{}))
			})
		})
		Context("when in full privilege mode", func() {
			var cmd baggageclaimcmd.BaggageclaimCommand
			cmd.PrivilegedMode = bespec.FullPrivilegedMode

			It("selects separate privileged and unprivileged namespacers", func() {
				priv, unpriv := cmd.SelectNamespacers(logger)
				var testCmd exec.Cmd
				priv.NamespaceCommand(&testCmd)
				Expect(testCmd.SysProcAttr.UidMappings[0].HostID).To(Equal(0))
				unpriv.NamespaceCommand(&testCmd)
				Expect(testCmd.SysProcAttr.UidMappings[0].HostID).To(Not(Equal(0)))
			})
		})
		Context("when in FUSE-only privilege mode", func() {
			var cmd baggageclaimcmd.BaggageclaimCommand
			cmd.PrivilegedMode = bespec.FUSEOnlyPrivilegedMode

			It("selects the unprivileged namespacer for both privilege levels", func() {
				priv, unpriv := cmd.SelectNamespacers(logger)
				var testCmd exec.Cmd
				priv.NamespaceCommand(&testCmd)
				Expect(testCmd.SysProcAttr.UidMappings[0].HostID).To(Not(Equal(0)))
				unpriv.NamespaceCommand(&testCmd)
				Expect(testCmd.SysProcAttr.UidMappings[0].HostID).To(Not(Equal(0)))
			})
		})
		Context("when in ignore privilege mode", func() {
			var cmd baggageclaimcmd.BaggageclaimCommand
			cmd.PrivilegedMode = bespec.IgnorePrivilegedMode

			It("selects the unprivileged namespacer for both privilege levels", func() {
				priv, unpriv := cmd.SelectNamespacers(logger)
				var testCmd exec.Cmd
				priv.NamespaceCommand(&testCmd)
				Expect(testCmd.SysProcAttr.UidMappings[0].HostID).To(Not(Equal(0)))
				unpriv.NamespaceCommand(&testCmd)
				Expect(testCmd.SysProcAttr.UidMappings[0].HostID).To(Not(Equal(0)))
			})
		})
	})
})
