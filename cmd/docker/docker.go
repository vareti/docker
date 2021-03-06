package main

import (
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/command"
	"github.com/docker/docker/cli/command/commands"
	cliflags "github.com/docker/docker/cli/flags"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/docker/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func newDockerCommand(dockerCli *command.DockerCli) *cobra.Command {
	opts := cliflags.NewClientOptions()
	var flags *pflag.FlagSet

	cmd := &cobra.Command{
		Use:              "docker [OPTIONS] COMMAND [arg...]",
		Short:            "A self-sufficient runtime for containers",
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
		Args:             noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Version {
				showVersion()
				return nil
			}
			cmd.SetOutput(dockerCli.Err())
			cmd.HelpFunc()(cmd, args)
			return nil
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// flags must be the top-level command flags, not cmd.Flags()
			opts.Common.SetDefaultOptions(flags)
			dockerPreRun(opts)
			return dockerCli.Initialize(opts)
		},
	}
	cli.SetupRootCommand(cmd)

	cmd.SetHelpFunc(func(ccmd *cobra.Command, args []string) {
		var err error
		if dockerCli.Client() == nil {
			// flags must be the top-level command flags, not cmd.Flags()
			opts.Common.SetDefaultOptions(flags)
			dockerPreRun(opts)
			err = dockerCli.Initialize(opts)
		}
		if err != nil || !dockerCli.HasExperimental() {
			hideExperimentalFeatures(ccmd)
		}
		if err := ccmd.Help(); err != nil {
			ccmd.Println(err)
		}
	})

	flags = cmd.Flags()
	flags.BoolVarP(&opts.Version, "version", "v", false, "Print version information and quit")
	flags.StringVar(&opts.ConfigDir, "config", cliconfig.ConfigDir(), "Location of client config files")
	opts.Common.InstallFlags(flags)

	cmd.SetOutput(dockerCli.Out())
	cmd.AddCommand(newDaemonCommand())
	commands.AddCommands(cmd, dockerCli)

	return cmd
}

func noArgs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	return fmt.Errorf(
		"docker: '%s' is not a docker command.\nSee 'docker --help'%s", args[0], ".")
}

func main() {
	// Set terminal emulation based on platform as required.
	stdin, stdout, stderr := term.StdStreams()
	logrus.SetOutput(stderr)

	dockerCli := command.NewDockerCli(stdin, stdout, stderr)
	cmd := newDockerCommand(dockerCli)

	if err := cmd.Execute(); err != nil {
		if sterr, ok := err.(cli.StatusError); ok {
			if sterr.Status != "" {
				fmt.Fprintln(stderr, sterr.Status)
			}
			// StatusError should only be used for errors, and all errors should
			// have a non-zero exit status, so never exit with 0
			if sterr.StatusCode == 0 {
				os.Exit(1)
			}
			os.Exit(sterr.StatusCode)
		}
		fmt.Fprintln(stderr, err)
		os.Exit(1)
	}
}

func showVersion() {
	fmt.Printf("Docker version %s, build %s\n", dockerversion.Version, dockerversion.GitCommit)
}

func dockerPreRun(opts *cliflags.ClientOptions) {
	cliflags.SetLogLevel(opts.Common.LogLevel)

	if opts.ConfigDir != "" {
		cliconfig.SetConfigDir(opts.ConfigDir)
	}

	if opts.Common.Debug {
		utils.EnableDebug()
	}
}

func hideExperimentalFeatures(cmd *cobra.Command) {
	// hide flags
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if _, ok := f.Annotations["experimental"]; ok {
			f.Hidden = true
		}
	})

	for _, subcmd := range cmd.Commands() {
		// hide subcommands
		if _, ok := subcmd.Tags["experimental"]; ok {
			subcmd.Hidden = true
		}
	}
}
