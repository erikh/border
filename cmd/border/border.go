package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/ffcli"
)

var (
	appFlagSet    = flag.NewFlagSet("border", flag.ExitOnError)
	serveFlagSet  = flag.NewFlagSet("border serve", flag.ExitOnError)
	clientFlagSet = flag.NewFlagSet("border client", flag.ExitOnError)
	config        = appFlagSet.String("c", "/etc/border/config.yaml", "configuration file path")
)

func main() {
	app := &ffcli.Command{
		Usage:   "border [flags] <subcommand> [args]",
		FlagSet: appFlagSet,
		Subcommands: []*ffcli.Command{
			{
				Name:      "serve",
				Usage:     "Start the border service",
				ShortHelp: "Start the border service",
				FlagSet:   serveFlagSet,
				Exec:      serve,
			},
			{
				Name:      "client",
				Usage:     "Talk to the border service",
				ShortHelp: "Talk to the border service",
				FlagSet:   clientFlagSet,
				Subcommands: []*ffcli.Command{
					{
						Name:      "addpeer",
						Usage:     "Add a peer to the quorum",
						ShortHelp: "Add a peer to the quorum",
						Exec:      clientAddPeer,
					},
					{
						Name:      "updateconfig",
						Usage:     "Update the configuration remotely",
						ShortHelp: "Update the configuration remotely",
						Exec:      clientUpdateConfig,
					},
				},
			},
		},
	}

	if len(os.Args) == 1 {
		fmt.Fprintln(os.Stderr, ffcli.DefaultUsageFunc(app))
	}

	if err := app.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func serve(args []string) error {
	fmt.Println(*config)
	return nil
}

func clientAddPeer(args []string) error {
	fmt.Println(*config)
	return nil
}

func clientUpdateConfig(args []string) error {
	fmt.Println(*config)
	return nil
}
