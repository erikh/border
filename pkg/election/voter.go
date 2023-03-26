package election

import (
	"sync"

	"github.com/erikh/border/pkg/config"
)

type Voter struct {
	config  *config.Config
	choices map[string]string
	index   uint
	mutex   sync.RWMutex
}

func NewVoter(c *config.Config, index uint) *Voter {
	return &Voter{
		config:  c,
		choices: map[string]string{},
		index:   index,
	}
}

func (v *Voter) Index() uint {
	return v.index
}

func (v *Voter) RegisterVote(voter, peer *config.Peer) {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	if _, ok := v.choices[voter.Name()]; !ok {
		v.choices[voter.Name()] = peer.Name()
	}
}

func (v *Voter) ReadyToVote() bool {
	v.mutex.RLock()
	defer v.mutex.RUnlock()

	for _, peer := range v.config.Peers {
		if _, ok := v.choices[peer.Name()]; !ok {
			return false
		}
	}

	return true
}

func (v *Voter) Vote() (*config.Peer, error) {
	v.mutex.RLock()
	defer v.mutex.RUnlock()

	choices := map[string]uint{}

	for _, choice := range v.choices {
		choices[choice]++
	}

	var (
		highestChoice      string
		highestChoiceCount uint
	)

	for choice, count := range choices {
		if highestChoice == "" || highestChoiceCount < count {
			highestChoice = choice
			highestChoiceCount = count
		}
	}

	return v.config.FindPeer(highestChoice)
}
