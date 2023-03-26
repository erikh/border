package election

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/erikh/border/pkg/api"
	"github.com/erikh/border/pkg/config"
	"github.com/erikh/border/pkg/controlclient"
)

type Election struct {
	context        context.Context
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

func NewElection(ctx context.Context, ec ElectionContext) *Election {
	return &Election{
		context:  ctx,
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

func (e *Election) ElectoratePeer() (string, error) {
	if e.electoratePeer == "" {
		if err := e.getElectorate(); err != nil {
			return "", err
		}
	}

	return e.electoratePeer, nil
}

func (e *Election) RegisterVote(me, chosen *config.Peer) {
	e.voter.RegisterVote(me, chosen)
}

func (e *Election) Perform() (*config.Peer, uint, error) {
	electoratePeer, err := e.ElectoratePeer()
	if err != nil {
		return nil, 0, err
	}

	peer, err := e.config.FindPeer(electoratePeer)
	if err != nil {
		return nil, 0, errors.New("electoratePeer was invalid")
	}

	client := controlclient.FromPeer(peer)

	if _, err := client.Exchange(&api.ElectionVoteRequest{Index: e.index, Me: e.me.Name(), Peer: peer.Name()}); err != nil {
		return nil, 0, err
	}

	// loop until we get a new publisher back. The repeated index must be larger
	// than the known index, to ensure freshness.
	for {
		select {
		case <-e.context.Done():
			return nil, 0, e.context.Err()
		default:
			time.Sleep(100 * time.Millisecond)
		}

		resp, err := client.Exchange(&api.IdentifyPublisherRequest{})
		if err != nil {
			return nil, 0, err
		}

		publisher := resp.(*api.IdentifyPublisherResponse)
		if publisher.EstablishedIndex <= e.index {
			log.Print("Vote has not occurred yet, according to index")
			continue
		} else if publisher.EstablishedIndex > e.index {
			return nil, e.index, errors.New("Vote has already expired")
		}

		peer, err := e.config.FindPeer(publisher.Publisher)
		return peer, publisher.EstablishedIndex, err
	}
}

func (e *Election) getElectorate() error {
	if e.electoratePeer != "" {
		return nil
	}

	if err := e.gatherUptimes(); err != nil {
		return err
	}

	e.uptimeMutex.RLock()
	defer e.uptimeMutex.RUnlock()

	var (
		electoratePeer   string
		electorateUptime time.Duration
	)

	for choice, uptime := range e.uptimes {
		if electoratePeer == "" || electorateUptime > uptime {
			electoratePeer = choice
			electorateUptime = uptime
		}
	}

	e.electoratePeer = electoratePeer

	return nil
}

func (e *Election) getUptime(peer *config.Peer) error {
	client := controlclient.FromPeer(peer)
	resp, err := client.Exchange(&api.UptimeRequest{})
	if err != nil {
		return err
	}

	e.uptimeMutex.Lock()
	defer e.uptimeMutex.Unlock()
	e.uptimes[peer.Name()] = resp.(*api.UptimeResponse).Uptime

	return nil
}

func (e *Election) gatherUptimes() error {
	errChan := make(chan error, len(e.config.Peers))

	for _, peer := range e.config.Peers {
		go func(e *Election, peer *config.Peer) {
			errChan <- e.getUptime(peer)
		}(e, peer)
	}

	for i := 0; i < len(e.config.Peers); i++ {
		select {
		case <-e.context.Done():
			return e.context.Err()
		case err := <-errChan:
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (e *Election) Voter() *Voter {
	return e.voter
}
