package election

import (
	"sync"
	"time"

	"github.com/erikh/border/pkg/config"
)

type Voter struct {
	config  *config.Config
	uptimes map[string]time.Duration
	mutex   sync.RWMutex
}

func NewVoter(c *config.Config) *Voter {
	return &Voter{
		config:  c,
		uptimes: map[string]time.Duration{},
	}
}

func (v *Voter) RegisterPeer(peer *config.Peer, uptime time.Duration) {
	v.mutex.Lock()
	v.uptimes[peer.Name()] = uptime
	v.mutex.Unlock()
}

func (v *Voter) ReadyToVote() bool {
	v.mutex.RLock()
	defer v.mutex.RUnlock()

	for _, peer := range v.config.Peers {
		if _, ok := v.uptimes[peer.Name()]; !ok {
			return false
		}
	}

	return true
}

func (v *Voter) Vote() (*config.Peer, error) {
	v.mutex.RLock()
	defer v.mutex.RUnlock()

	var (
		lowestUptime time.Duration
		lowestName   string
	)

	for name, uptime := range v.uptimes {
		if lowestUptime == 0 || lowestUptime > uptime {
			lowestUptime = uptime
			lowestName = name
		}
	}

	return v.config.FindPeer(lowestName)
}
