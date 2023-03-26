package controlserver

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erikh/border/pkg/api"
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

	var (
		electoratePeerName string
		index              uint
	)

	for _, peer := range c.Peers {
		client := makeClient(peer.ControlServer, peer.Key)
		resp, err := client.Exchange(&api.StartElectionRequest{}, true)
		if err != nil && !strings.Contains(err.Error(), ErrElectionIncomplete.Error()) {
			t.Fatal(err)
		}

		if resp != nil {
			ep := resp.(*api.StartElectionResponse).ElectoratePeer
			idx := resp.(*api.StartElectionResponse).Index

			if electoratePeerName != "" && ep != electoratePeerName {
				t.Fatal("discrete peers were picked")
			}

			if index != 0 && idx != index {
				t.Fatal("Multiple indexes were created")
			}

			electoratePeerName = ep
			index = idx
		}

		electoratePeer, err := c.FindPeer(electoratePeerName)
		if err != nil {
			t.Fatal(err)
		}

		electorateClient := makeClient(electoratePeer.ControlServer, electoratePeer.Key)

		_, err = electorateClient.Exchange(&api.ElectionVoteRequest{Index: 0, Me: peer.Name(), Peer: peer.Name()}, true)
		if err == nil {
			t.Fatal("accepted vote when shouldn't have: invalid index")
		}

		if electoratePeerName != peer.Name() {
			electorateClient.Exchange(&api.StartElectionRequest{}, true)

			_, err = client.Exchange(&api.ElectionVoteRequest{Index: index, Me: peer.Name(), Peer: peer.Name()}, true)
			if err == nil {
				t.Fatal("accepted vote when shouldn't have: invalid electorate")
			}

			_, err = client.Exchange(&api.IdentifyPublisherRequest{}, true)
			if err == nil {
				t.Fatal("Invalid identification of publisher")
			}
		}

		_, err = electorateClient.Exchange(&api.ElectionVoteRequest{Index: index, Me: peer.Name(), Peer: electoratePeerName}, true)
		if err != nil {
			t.Fatal(err)
		}

	}

	electoratePeer, err := c.FindPeer(electoratePeerName)
	if err != nil {
		t.Fatal(err)
	}

	electorateClient := makeClient(electoratePeer.ControlServer, electoratePeer.Key)
	resp, err := electorateClient.Exchange(&api.IdentifyPublisherRequest{}, true)
	if err != nil {
		t.Fatal(err)
	}

	ipr := resp.(*api.IdentifyPublisherResponse)

	if ipr.EstablishedIndex != index {
		t.Fatal("Identified publisher does not have the right index")
	}

	if ipr.Publisher != electoratePeerName {
		t.Fatal("Identified publisher was not the electorate; should be")
	}
}
