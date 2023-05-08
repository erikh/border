package config

import (
	"github.com/mholt/acmez/acme"
)

func (c *Config) ACMEDeleteChallenge(domain string) {
	EditMutex.Lock()
	defer EditMutex.Unlock()
	delete(c.ACMEChallenges, domain)
}

func (c *Config) ACMESetChallenge(domain string, chal acme.Challenge) {
	EditMutex.Lock()
	defer EditMutex.Unlock()
	c.ACMEChallenges[domain] = chal
}

func (c *Config) ACMEGetChallenge(domain string) (acme.Challenge, bool) {
	EditMutex.RLock()
	defer EditMutex.RUnlock()

	chal, ok := c.ACMEChallenges[domain]
	return chal, ok
}

func (c *Config) ACMESetReady(domain string, peer *Peer) {
	EditMutex.Lock()
	defer EditMutex.Unlock()

	c.ACMEReady[domain] = append(c.ACMEReady[domain], peer)
}

func (c *Config) ACMEGetReady(domain string) []*Peer {
	EditMutex.RLock()
	defer EditMutex.RUnlock()

	peers, ok := c.ACMEReady[domain]
	if !ok {
		return []*Peer{}
	}

	return peers
}
