package controlserver

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/erikh/border/pkg/config"
	"github.com/erikh/border/pkg/josekit"
)

func makeMultiPeerConfig(t *testing.T, peers uint) *config.Config {
	authKey, err := josekit.MakeKey("authkey")
	if err != nil {
		t.Fatal(err)
	}

	peerList := []*config.Peer{}

	for i := uint(0); i < peers; i++ {
		jwk, err := josekit.MakeKey(fmt.Sprintf("peer%d", i))
		if err != nil {
			t.Fatal(err)
		}

		peerList = append(peerList, &config.Peer{
			Key:           jwk,
			IPs:           []net.IP{net.ParseIP("127.0.0.1")},
			ControlServer: fmt.Sprintf("127.0.0.1:530%d", i),
		})
	}

	dir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	c := &config.Config{
		Peers:          peerList,
		FilenamePrefix: filepath.Join(dir, "config"),
		AuthKey:        authKey,
	}

	c.InitReload()
	return c
}

func spawnServers(t *testing.T, c *config.Config) []*Server {
	servers := []*Server{}

	for i, peer := range c.Peers {
		server, err := Start(c, peer, fmt.Sprintf("127.0.0.1:530%d", i), 10*time.Millisecond, 10*time.Millisecond)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			server.Shutdown(ctx) // nolint:errcheck
		})

		servers = append(servers, server)
	}

	return servers
}

func TestElection(t *testing.T) {
	const count = 8

	c := makeMultiPeerConfig(t, count)
	spawnServers(t, c)
}
