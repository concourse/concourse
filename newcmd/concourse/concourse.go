package main

import (
	"github.com/concourse/concourse/atc/atccmd"
	"github.com/concourse/concourse/worker/land"
	"github.com/concourse/concourse/worker/retire"
	"github.com/concourse/concourse/worker/workercmd"
	"github.com/jessevdk/go-flags"
)

var (
	cfgFile string

	concourseCmd = &cobra.Command{
		Use:   "concourse",
		Short: "TODO",
		Long:  `TODO`,
	}
)

func Execute() error {
	return concourseCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.concourse.yaml)")

	// XXX: rootCmd.AddCommand(VersionCommand) this won't be --version like before
	rootCmd.AddCommand(WebCommand)
	// rootCmd.AddCommand(workercmd.WorkerCommand)
	// rootCmd.AddCommand(atccmd.Migration)
	// rootCmd.AddCommand(QuickstartCommand)
	// rootCmd.AddCommand(land.LandWorkerCommand)
	// rootCmd.AddCommand(retire.RetireWorkerCommand)
	// rootCmd.AddCommand(GenerateKeyCommand)
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			er(err)
		}

		// Search config in home directory with name ".concourse"
		viper.AddConfigPath(home)
		viper.SetConfigName(".concourse")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

// type ConcourseCommand struct {
// 	Version func() `short:"v" long:"version" description:"Print the version of Concourse and exit"`

// 	Web     WebCommand              `command:"web"     description:"Run the web UI and build scheduler."`
// 	Worker  workercmd.WorkerCommand `command:"worker"  description:"Run and register a worker."`
// 	Migrate atccmd.Migration        `command:"migrate" description:"Run database migrations."`

// 	Quickstart QuickstartCommand `command:"quickstart" description:"Run both 'web' and 'worker' together, auto-wired. Not recommended for production."`

// 	LandWorker   land.LandWorkerCommand     `command:"land-worker" description:"Safely drain a worker's assignments for temporary downtime."`
// 	RetireWorker retire.RetireWorkerCommand `command:"retire-worker" description:"Safely remove a worker from the cluster permanently."`

// 	GenerateKey GenerateKeyCommand `command:"generate-key" description:"Generate RSA key for use with Concourse components."`
// }

// func (cmd ConcourseCommand) LessenRequirements(parser *flags.Parser) {
// 	cmd.Quickstart.LessenRequirements(parser.Find("quickstart"))
// 	cmd.Web.LessenRequirements(parser.Find("web"))
// 	cmd.Worker.LessenRequirements("", parser.Find("worker"))
// }
