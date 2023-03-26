package election

import (
	"context"
	"errors"
	"log"
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
	index          uint
	bootTime       time.Time
	electoratePeer string
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
		voter:    NewVoter(ec.Config),
		index:    ec.Index,
		bootTime: ec.BootTime,
	}
}

func (e *Election) Index() uint {
	return e.index
}

func (e *Election) ElectoratePeer() (string, error) {
	if err := e.getElectorate(); err != nil {
		return "", err
	}

	return e.electoratePeer, nil
}

func (e *Election) Perform() (*config.Peer, uint, error) {
	// several strategies to choose the arbiter for the election, called the
	// "electorate" here:
	//
	// - start an election, if one is already running, the chosen electorate will
	//   be returned
	// - gather all uptimes, choose the oldest machine and make that the
	//   electorate
	// - otherwise, return an error
	if !e.voter.ReadyToVote() {
		electorateChan := make(chan string, len(e.config.Peers))
		errChan := make(chan error, len(e.config.Peers))

		for _, peer := range e.config.Peers {
			go func(e *Election, peer *config.Peer) {
				client := controlclient.FromPeer(peer)
				resp, err := client.Exchange(&api.StartElectionRequest{})
				if err != nil {
					errChan <- err
					return
				}

				electorateChan <- resp.(*api.StartElectionResponse).ElectoratePeer
				errChan <- nil
			}(e, peer)
		}

		var existingElectorate string

		for i := 0; i < len(e.config.Peers); i++ {
			select {
			case <-e.context.Done():
				return nil, 0, e.context.Err()
			case err := <-errChan:
				if err != nil {
					return nil, 0, err
				}
			case peerName := <-electorateChan:
				if existingElectorate == "" {
					existingElectorate = peerName
				} else if existingElectorate != peerName {
					if err := e.getElectorate(); err != nil {
						return nil, 0, err
					}
				}
			}
		}
	}

	if !e.voter.ReadyToVote() {
		if err := e.gatherUptimes(); err != nil {
			return nil, 0, err
		}
	}

	if !e.voter.ReadyToVote() {
		return nil, 0, errors.New("Could not gather full peer information during election process")
	}

	// submit our uptime data to the electorate
	peer, err := e.config.FindPeer(e.electoratePeer)
	if err != nil {
		return nil, 0, errors.New("electoratePeer was invalid")
	}

	client := controlclient.FromPeer(peer)

	if _, err := client.Exchange(&api.ElectionVoteRequest{Me: e.me.Name(), Uptime: time.Since(e.bootTime)}); err != nil {
		return nil, 0, err
	}

	// then, loop until we get a new publisher back. The repeated index must be
	// larger than the known index, to ensure freshness.
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

	peer, err := e.voter.Vote()
	if err != nil {
		return err
	}

	e.electoratePeer = peer.Name()
	return nil
}

func (e *Election) gatherUptimes() error {
	errChan := make(chan error, len(e.config.Peers))

	for _, peer := range e.config.Peers {
		go func(e *Election, peer *config.Peer) {
			client := controlclient.FromPeer(peer)
			resp, err := client.Exchange(&api.UptimeRequest{})
			if err != nil {
				errChan <- err
				return
			}

			e.voter.RegisterPeer(peer, resp.(*api.UptimeResponse).Uptime)
			errChan <- nil
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
