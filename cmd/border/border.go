package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/erikh/border/pkg/config"
	"github.com/erikh/border/pkg/controlserver"
	"github.com/erikh/border/pkg/dnsserver"
	"github.com/erikh/border/pkg/josekit"
	"github.com/ghodss/yaml"
	"github.com/peterbourgon/ff/ffcli"
	"golang.org/x/sys/unix"
)

var (
	appFlagSet         = flag.NewFlagSet("border", flag.ExitOnError)
	serveFlagSet       = flag.NewFlagSet("border serve", flag.ExitOnError)
	clientFlagSet      = flag.NewFlagSet("border client", flag.ExitOnError)
	keyGenerateFlagSet = flag.NewFlagSet("border keygenerate", flag.ExitOnError)
	configFile         = appFlagSet.String("c", "/etc/border/config.yaml", "configuration file path")
	keyID              = keyGenerateFlagSet.String("id", "control", "key ID (kid) of JSON Web Key")
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
			{
				Name:      "keygenerate",
				Usage:     "Generate a new authentication key for use in border",
				ShortHelp: "Generate a new authentication key for use in border",
				FlagSet:   keyGenerateFlagSet,
				Exec:      keyGenerate,
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
	c, err := config.FromDisk(*configFile, config.LoadYAML)
	if err != nil {
		return err
	}

	cs, err := controlserver.Start(c, c.Listen.Control, controlserver.NonceExpiration, 100*time.Millisecond)
	if err != nil {
		return err
	}

	dnsserver := dnsserver.DNSServer{
		Zones: c.Zones,
	}

	if err := dnsserver.Start(c.Listen.DNS); err != nil {
		return err
	}

	sigChan := make(chan os.Signal, 1)
	outerContext, outerCancel := context.WithCancel(context.Background())

	go func() {
		<-sigChan

		sw := c.ShutdownWait
		if sw == 0 {
			sw = time.Second
		}

		ctx, cancel := context.WithTimeout(outerContext, sw)
		defer cancel()
		defer outerCancel()

		dnsserver.Shutdown()
		cs.Shutdown(ctx)
	}()

	signal.Notify(sigChan, unix.SIGTERM, unix.SIGINT)

	select {
	case <-outerContext.Done():
		fmt.Fprintln(os.Stderr, "terminating")
		os.Exit(0)
	}

	return nil
}

func clientAddPeer(args []string) error {
	fmt.Println(*configFile)
	return nil
}

func clientUpdateConfig(args []string) error {
	fmt.Println(*configFile)
	return nil
}

func keyGenerate(args []string) error {
	key, err := josekit.MakeKey(*keyID)
	if err != nil {
		return err
	}

	byt, err := yaml.Marshal(key)
	if err != nil {
		return err
	}

	fmt.Println(string(byt))

	return nil
}
