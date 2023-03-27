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
	config         *config.Config
	me             *config.Peer
	voter          *Voter
	uptimes        map[string]time.Duration
	index          uint
	bootTime       time.Time
	electoratePeer string
	uptimeMutex    sync.RWMutex
}

type ElectionContext struct {
	Config   *config.Config
	Me       *config.Peer
	Index    uint
	BootTime time.Time
}

func NewElection(ec ElectionContext) *Election {
	return &Election{
		config:   ec.Config,
		me:       ec.Me,
		voter:    NewVoter(ec.Config, ec.Index),
		uptimes:  map[string]time.Duration{},
		index:    ec.Index,
		bootTime: ec.BootTime,
	}
}

func (e *Election) Index() uint {
	return e.index
}

func (e *Election) ElectoratePeer() string {
	if e.electoratePeer == "" {
		e.getElectorate()
	}

	return e.electoratePeer
}

func (e *Election) RegisterVote(me, chosen *config.Peer) {
	e.voter.RegisterVote(me, chosen)
}

func (e *Election) getElectorate() {
	if e.electoratePeer != "" {
		return
	}

	e.gatherUptimes()

	e.uptimeMutex.RLock()
	defer e.uptimeMutex.RUnlock()

	var (
		electoratePeer   string
		electorateUptime time.Duration
	)

	for choice, uptime := range e.uptimes {
		if electoratePeer == "" || electorateUptime < uptime {
			electoratePeer = choice
			electorateUptime = uptime
		}
	}

	e.electoratePeer = electoratePeer
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

func (e *Election) Voter() *Voter {
	return e.voter
}
