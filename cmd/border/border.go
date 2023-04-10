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

	"github.com/erikh/border/pkg/acmekit"
	"github.com/erikh/border/pkg/api"
	"github.com/erikh/border/pkg/config"
	"github.com/erikh/border/pkg/controlclient"
	"github.com/erikh/border/pkg/josekit"
	"github.com/erikh/border/pkg/launcher"
	"github.com/erikh/go-hashchain"
	"github.com/ghodss/yaml"
	"github.com/go-jose/go-jose/v3"
	"github.com/peterbourgon/ff/ffcli"
	"golang.org/x/sys/unix"
)

var (
	appFlagSet       = flag.NewFlagSet("border", flag.ExitOnError)
	configFile       = appFlagSet.String("c", "/etc/border/config.yaml", "configuration file path")
	clientConfigFile = appFlagSet.String("client", "/etc/border/client.yaml", "client configuration file path")

	serveFlagSet       = flag.NewFlagSet("border serve", flag.ExitOnError)
	clientFlagSet      = flag.NewFlagSet("border client", flag.ExitOnError)
	keyGenerateFlagSet = flag.NewFlagSet("border keygenerate", flag.ExitOnError)

	acmeGenerateFlagSet = flag.NewFlagSet("border acmegenerate", flag.ExitOnError)
	acmeDirectory       = acmeGenerateFlagSet.String("directory", "", "Third-party ACME server's directory URL")
	acmeNoVerify        = acmeGenerateFlagSet.Bool("noverify", false, "Verify TLS certificates used to talk to the ACME server")
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
					{
						Name:      "identifypublisher",
						Usage:     "border client identifypublisher",
						ShortHelp: "Produce the name of the current elected publisher peer",
						Exec:      clientIdentifyPublisher,
					},
				},
			},
			{
				Name:      "keygenerate",
				Usage:     "border keygenerate <name>",
				ShortHelp: "Generate a new authentication key for use in border",
				FlagSet:   keyGenerateFlagSet,
				Exec:      keyGenerate,
			},
			{
				Name:      "acmegenerate",
				Usage:     "border acmegenerate <email>",
				ShortHelp: "Generate a new ACME account for use with border",
				FlagSet:   acmeGenerateFlagSet,
				Exec:      acmeGenerate,
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

	c := config.New(hashchain.New(nil))

	c, err := c.FromDisk(*configFile, config.LoadYAML)
	if err != nil {
		return err
	}

	// save and reload to initialize our internal JSON configuration. This is
	// used when exchanging configuration between peers, so this is a bootstrap
	// and only needs to be performed once.
	if err := c.Save(); err != nil {
		return err
	}

	if err := c.Reload(); err != nil {
		return err
	}

	<-c.ReloadChan()

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

	if _, err := client.SendRequest(&authCheck, false); err != nil {
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

	_, err = client.Exchange(req, false)
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

	if _, err := client.Exchange(&api.ConfigReloadRequest{}, false); err != nil {
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

	c := config.New(hashchain.New(nil))

	c, err = c.FromDisk(args[0], config.LoadYAML)
	if err != nil {
		return err
	}

	if _, err := client.Exchange(&api.ConfigUpdateRequest{Config: c}, false); err != nil {
		return fmt.Errorf("Error updating configuration: %w", err)
	}

	fmt.Println("OK")
	return nil
}

func clientIdentifyPublisher(args []string) error {
	client, err := controlclient.Load(*clientConfigFile)
	if err != nil {
		return fmt.Errorf("Could not load client configuration at %q: %w", *clientConfigFile, err)
	}

	resp, err := client.Exchange(&api.IdentifyPublisherRequest{}, false)
	if err != nil {
		return fmt.Errorf("Error identifying publisher: %w", err)
	}

	fmt.Println("Publisher:", resp.(*api.IdentifyPublisherResponse).Publisher)
	return nil
}

func keyGenerate(args []string) error {
	if len(args) != 1 {
		return errors.New("Please provide a key id as an argument")
	}

	key, err := josekit.MakeKey(args[0])
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

func acmeGenerate(args []string) error {
	if len(args) != 1 {
		return errors.New("Please provide a contact email address for use with the ACME server")
	}

	ap := acmekit.ACMEParams{
		IgnoreVerify: *acmeNoVerify,
		Directory:    *acmeDirectory,
		ContactInfo:  []string{"mailto:" + args[0]},
	}

	if err := ap.CreateAccount(context.Background()); err != nil {
		return fmt.Errorf("While creating ACME account: %w", err)
	}

	out, err := yaml.Marshal(struct {
		ACME acmekit.ACMEParams `json:"acme"`
	}{ACME: ap})

	if err != nil {
		return fmt.Errorf("Unable to marshal YAML configuration: %w", err)
	}

	fmt.Println("# Please paste this into your configuration file")
	fmt.Println(string(out))

	return nil
}
