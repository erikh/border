package election

import (
	"log"
	"sync"
	"time"

	"github.com/erikh/border/pkg/api"
	"github.com/erikh/border/pkg/config"
	"github.com/erikh/border/pkg/controlclient"
)

type Election struct {
	config      *config.Config
	uptimes     map[string]time.Duration
	uptimeMutex sync.RWMutex
}

func NewElection(c *config.Config) *Election {
	return &Election{
		config:  c,
		uptimes: map[string]time.Duration{},
	}
}

func (e *Election) Vote() (*config.Peer, error) {
	e.gatherUptimes()

	e.uptimeMutex.RLock()
	defer e.uptimeMutex.RUnlock()

	var (
		oldestPeer   string
		oldestUptime time.Duration
	)

	for peerName, uptime := range e.uptimes {
		if uptime > oldestUptime {
			oldestPeer = peerName
			oldestUptime = uptime
		}
	}

	return e.config.FindPeer(oldestPeer)
}

func (e *Election) getUptime(peer *config.Peer) error {
	client := controlclient.FromPeer(peer)
	resp, err := client.Exchange(&api.UptimeRequest{}, true)
	if err != nil {
		return err
	}

	e.uptimeMutex.Lock()
	defer e.uptimeMutex.Unlock()
	e.uptimes[peer.Name()] = resp.(*api.UptimeResponse).Uptime

	return nil
}

func (e *Election) gatherUptimes() {
	wg := &sync.WaitGroup{}
	wg.Add(len(e.config.Peers))

	for _, peer := range e.config.Peers {
		go func(e *Election, peer *config.Peer) {
			if err := e.getUptime(peer); err != nil {
				log.Printf("Peer %q could not be reached, pruning for now: %v", peer.Name(), err)
				e.config.RemovePeer(peer)
			}
			wg.Done()
		}(e, peer)
	}

	wg.Wait()
}
