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

	// this loop is pretty hairy, so I've attempted to over-comment it.
	for _, peer := range c.Peers {
		client := makeClient(peer.ControlServer, peer.Key)

		// starting an election on all peers will cause it to agree on a
		// electorate, or where the votes will be submitted, based on the uptime of
		// all the peers in nanoseconds. The highest uptime wins.
		resp, err := client.Exchange(&api.StartElectionRequest{}, true)

		// if the election is incomplete, it's fine, we just have to take some
		// special cases. This is because the first peer was not the electorate, so
		// later on, we have to start a vote prematurely. On the second iteration,
		// the "start election" code complains that one is already in progress.
		if err != nil && !strings.Contains(err.Error(), ErrElectionIncomplete.Error()) {
			t.Fatal(err)
		}

		// if we get an error up there, resp will be nil, so don't try to calculate
		// the electorate. Because of the condition to get here, it's already been
		// picked anyway.
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

		// an invalid election index should always fail.
		_, err = electorateClient.Exchange(&api.ElectionVoteRequest{Index: 0, Me: peer.Name(), Peer: peer.Name()}, true)
		if err == nil {
			t.Fatal("accepted vote when shouldn't have: invalid index")
		}

		// if we aren't the electorate:
		if electoratePeerName != peer.Name() {
			// start an election with the electorate already picked. this is why the
			// special casing around starting an election above is necessary. No
			// votes will work until the election has started.
			electorateClient.Exchange(&api.StartElectionRequest{}, true)

			// try to register a vote on the peer, which should fail, because it's
			// not the electorate.
			_, err = client.Exchange(&api.ElectionVoteRequest{Index: index, Me: peer.Name(), Peer: peer.Name()}, true)
			if err == nil {
				t.Fatal("accepted vote when shouldn't have: invalid electorate")
			}

			// try to identify the publisher, again, not the electorate
			_, err = client.Exchange(&api.IdentifyPublisherRequest{}, true)
			if err == nil {
				t.Fatal("Invalid identification of publisher")
			}
		}

		// finally, elect someone
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

	// assuming all is well, identifying the publisher should work at this point,
	// which will just be the electoratePeer because that's who we voted for.
	resp, err := electorateClient.Exchange(&api.IdentifyPublisherRequest{}, true)
	if err != nil {
		t.Fatal(err)
	}

	// validate the parameters to ensure nothing is racing / out of whack.
	ipr := resp.(*api.IdentifyPublisherResponse)

	if ipr.EstablishedIndex != index {
		t.Fatal("Identified publisher does not have the right index")
	}

	if ipr.Publisher != electoratePeerName {
		t.Fatal("Identified publisher was not the electorate; should be")
	}
}
