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
	"github.com/erikh/border/pkg/josekit"
	"github.com/erikh/border/pkg/launcher"
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
						Usage:     "border client addpeer <YAML JWK keyfile> <ip (repeating)>",
						ShortHelp: "Add a peer to the quorum. Use keygenerate to generate a keyfile.",
						Exec:      clientAddPeer,
					},
					{
						Name:      "updateconfig",
						Usage:     "border client updateconfig <config file>",
						ShortHelp: "Update the configuration remotely",
						Exec:      clientUpdateConfig,
					},
					{
						Name:      "reloadconfig",
						Usage:     "border client reloadconfig",
						ShortHelp: "Force a reload of the configuration",
						Exec:      clientReloadConfig,
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
	if len(args) != 1 {
		return fmt.Errorf("Please provide a peer name to serve from")
	}

	c, err := config.FromDisk(*configFile, config.LoadYAML)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := &launcher.Server{}
	if err := server.Launch(args[0], c); err != nil {
		return err
	}

	sigChan := make(chan os.Signal, 1)

	go func() {
		<-sigChan

		sw := c.ShutdownWait
		if sw == 0 {
			sw = time.Second
		}

		innerCtx, innerCancel := context.WithTimeout(ctx, sw)
		defer cancel()
		defer innerCancel()

		server.Shutdown(innerCtx) // nolint:errcheck
	}()

	signal.Notify(sigChan, unix.SIGTERM, unix.SIGINT)

	<-ctx.Done()
	fmt.Fprintln(os.Stderr, "terminating")

	return nil
}

func clientAuthCheck(args []string) error {
	client, err := controlclient.Load(*clientConfigFile)
	if err != nil {
		return fmt.Errorf("Could not load client configuration at %q: %w", *clientConfigFile, err)
	}

	authCheck := make(api.AuthCheck, api.NonceSize)

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

	if len(args) < 2 {
		return errors.New("Please provide a key file and list of peer IPs")
	}

	var jwk jose.JSONWebKey

	byt, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("Could not read key file: %w", err)
	}

	if err := yaml.Unmarshal(byt, &jwk); err != nil {
		return fmt.Errorf("Could not unmarshal JWK YAML: %w", err)
	}

	ips := []net.IP{}

	for _, ipstr := range args[1:] {
		ip := net.ParseIP(ipstr)
		if ip == nil {
			return fmt.Errorf("IP %q is not a valid IP address", ipstr)
		}

		ips = append(ips, ip)
	}

	peer := &config.Peer{
		Key: &jwk,
		IPs: ips,
	}

	req := &api.PeerRegistrationRequest{
		Peer: peer,
	}

	_, err = client.Exchange(req)
	return err
}

func clientReloadConfig(args []string) error {
	client, err := controlclient.Load(*clientConfigFile)
	if err != nil {
		return fmt.Errorf("Could not load client configuration at %q: %w", *clientConfigFile, err)
	}

	if len(args) != 0 {
		return errors.New("Invalid Arguments")
	}

	if _, err := client.Exchange(&api.ConfigReloadRequest{}); err != nil {
		return fmt.Errorf("Error updating configuration: %w", err)
	}

	fmt.Println("OK")
	return nil
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

	if _, err := client.Exchange(&api.ConfigUpdateRequest{Config: c}); err != nil {
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
