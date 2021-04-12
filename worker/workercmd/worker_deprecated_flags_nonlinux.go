// +build !linux

package workercmd

import "github.com/spf13/cobra"

func InitializeRuntimeFlagsDEPRECATED(c *cobra.Command, flags *WorkerCommand) {
	c.Flags().DurationVar(&flags.Guardian.RequestTimeout, "garden-request-timeout", CmdDefaults.Guardian.RequestTimeout, "How long to wait for requests to the Garden server to complete. 0 means no timeout.")
}
