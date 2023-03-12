package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/erikh/border/pkg/api"
	"github.com/erikh/border/pkg/config"
	"github.com/erikh/border/pkg/controlclient"
	"github.com/erikh/border/pkg/controlserver"
	"github.com/erikh/border/pkg/dnsserver"
	"github.com/erikh/border/pkg/josekit"
	"github.com/ghodss/yaml"
	"github.com/go-jose/go-jose/v3"
	"github.com/peterbourgon/ff/ffcli"
	"golang.org/x/sys/unix"
)

var (
	appFlagSet         = flag.NewFlagSet("border", flag.ExitOnError)
	serveFlagSet       = flag.NewFlagSet("border serve", flag.ExitOnError)
	clientFlagSet      = flag.NewFlagSet("border client", flag.ExitOnError)
	keyGenerateFlagSet = flag.NewFlagSet("border keygenerate", flag.ExitOnError)
	configFile         = appFlagSet.String("c", "/etc/border/config.yaml", "configuration file path")
	clientConfigFile   = appFlagSet.String("client", "/etc/border/client.yaml", "client configuration file path")
	keyID              = keyGenerateFlagSet.String("id", "control", "key ID (kid) of JSON Web Key")
)

func main() {
	app := &ffcli.Command{
		Usage:   "border [flags] <subcommand> [args]",
		FlagSet: appFlagSet,
		Subcommands: []*ffcli.Command{
			{
				Name:      "serve",
				Usage:     "border serve",
				ShortHelp: "Start the border service",
				FlagSet:   serveFlagSet,
				Exec:      serve,
			},
			{
				Name:      "client",
				Usage:     "border client --help",
				ShortHelp: "Talk to the border service",
				FlagSet:   clientFlagSet,
				Subcommands: []*ffcli.Command{
					{
						Name:      "authcheck",
						Usage:     "border client authcheck",
						ShortHelp: "Validates that auth is working without doing anything else",
						Exec:      clientAuthCheck,
					},
					{
						Name:      "addpeer",
						Usage:     "border client addpeer <ip> <YAML JWK keyfile>",
						ShortHelp: "Add a peer to the quorum. Use keygenerate to generate a keyfile.",
						Exec:      clientAddPeer,
					},
					{
						Name:      "updateconfig",
						Usage:     "border client updateconfig <config file>",
						ShortHelp: "Update the configuration remotely",
						Exec:      clientUpdateConfig,
					},
				},
			},
			{
				Name:      "keygenerate",
				Usage:     "border keygenerate",
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

func clientAuthCheck(args []string) error {
	client, err := controlclient.Load(*clientConfigFile)
	if err != nil {
		return fmt.Errorf("Could not load client configuration at %q: %w", *clientConfigFile, err)
	}

	authCheck := make(api.AuthCheck, controlserver.NonceSize)

	if _, err := client.SendRequest(api.PathAuthCheck, &authCheck); err != nil {
		return fmt.Errorf("Authentication failed: %w", err)
	}

	fmt.Println("OK")
	return nil
}

func clientAddPeer(args []string) error {
	client, err := controlclient.Load(*clientConfigFile)
	if err != nil {
		return fmt.Errorf("Could not load client configuration at %q: %w", *clientConfigFile, err)
	}

	if len(args) != 2 {
		return errors.New("Please provide a peer IP and key file")
	}

	var jwk jose.JSONWebKey

	ip := net.ParseIP(args[0])
	if ip == nil {
		return fmt.Errorf("IP %q is not a valid IP address", args[0])
	}

	byt, err := os.ReadFile(args[1])
	if err != nil {
		return fmt.Errorf("Could not read key file: %w", err)
	}

	if err := yaml.Unmarshal(byt, &jwk); err != nil {
		return fmt.Errorf("Could not unmarshal JWK YAML: %w", err)
	}

	peer := config.Peer{
		Key: &jwk,
		IP:  ip,
	}

	req := &api.PeerRegistrationRequest{
		Peer: peer,
	}

	resp := api.NilResponse{}
	return client.Exchange(api.PathPeerRegistration, req, &resp)
}

func clientUpdateConfig(args []string) error {
	client, err := controlclient.Load(*clientConfigFile)
	if err != nil {
		return fmt.Errorf("Could not load client configuration at %q: %w", *clientConfigFile, err)
	}

	if len(args) != 1 {
		return errors.New("Please provide a config file to load")
	}

	c, err := config.FromDisk(args[0], config.LoadYAML)
	if err != nil {
		return err
	}

	if err := client.Exchange(api.PathConfigUpdate, &api.ConfigUpdateRequest{Config: c}, &api.NilResponse{}); err != nil {
		return fmt.Errorf("Error updating configuration: %w", err)
	}

	fmt.Println("OK")
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
