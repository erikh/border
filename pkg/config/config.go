package config

import (
	"crypto/sha512"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/erikh/border/pkg/acmekit"
	"github.com/erikh/border/pkg/dnsconfig"
	"github.com/erikh/go-hashchain"
	"github.com/go-jose/go-jose/v3"
	"github.com/mholt/acmez/acme"
)

var (
	ErrDump         = errors.New("while dumping configuration to disk")
	ErrLoad         = errors.New("while loading configuration from disk")
	ErrPeerNotFound = errors.New("peer not found")
)

var (
	FileMutex sync.RWMutex
	EditMutex sync.RWMutex
)

var HashFunc = sha512.New

type Config struct {
	FilenamePrefix string                    `json:"-"` // prefix of filename to save to and read from
	Publisher      *Peer                     `json:"-"` // peer that's the publisher
	Me             *Peer                     `json:"-"` // this peer
	ACMEChallenges map[string]acme.Challenge `json:"-"` // domain -> peers map of challenge payloads
	ACMEReady      map[string][]*Peer        `json:"-"` // domain -> peers map of ready peers for challenge

	ACME         *acmekit.ACMEParams `json:"acme"`
	ShutdownWait time.Duration       `json:"shutdown_wait"`
	AuthKey      *jose.JSONWebKey    `json:"auth_key"`
	Listen       ListenConfig        `json:"listen"`
	Peers        []*Peer             `json:"peers"`
	Zones        map[string]*Zone    `json:"zones"`

	chain  *hashchain.Chain
	reload chan struct{}
}

type ListenConfig struct {
	DNS     string `json:"dns"`
	Control string `json:"control"`
}

type Peer struct {
	IPs           []net.IP         `json:"ips"`
	ControlServer string           `json:"control_server"`
	Key           *jose.JSONWebKey `json:"key"`
}

func (p *Peer) Name() string {
	return p.Key.KeyID
}

type Zone struct {
	SOA     *dnsconfig.SOA `json:"soa"`
	NS      *dnsconfig.NS  `json:"ns"`
	Records []*Record      `json:"records"`
}

func New(chain *hashchain.Chain) *Config {
	return &Config{chain: chain, reload: make(chan struct{}, 1)}
}

func (c *Config) Chain() *hashchain.Chain {
	EditMutex.RLock()
	defer EditMutex.RUnlock()
	return c.chain
}

func (c *Config) SetChain(chain *hashchain.Chain) {
	EditMutex.Lock()
	defer EditMutex.Unlock()
	c.chain = chain
}

func (c *Config) ReloadChan() <-chan struct{} {
	EditMutex.RLock()
	defer EditMutex.RUnlock()
	return c.reload
}

func (c *Config) Reload() error {
	newConfig, err := c.FromDisk(c.FilenamePrefix+".json", LoadJSON)
	if err != nil {
		return fmt.Errorf("While reloading configuration: %w", err)
	}

	EditMutex.Lock()
	publisher := c.Publisher
	reload := c.reload
	chain := c.chain
	EditMutex.Unlock()

	c.CopyFrom(newConfig)

	EditMutex.Lock()
	c.Publisher = publisher
	c.reload = reload
	c.chain = chain
	EditMutex.Unlock()

	c.reload <- struct{}{}
	return nil
}

func (c *Config) CopyFrom(newConfig *Config) {
	EditMutex.Lock()
	defer EditMutex.Unlock()
	c.ShutdownWait = newConfig.ShutdownWait
	c.AuthKey = newConfig.AuthKey
	c.Listen = newConfig.Listen
	c.Peers = newConfig.Peers
	c.Zones = newConfig.Zones
}

func (c *Config) FindPeer(name string) (*Peer, error) {
	EditMutex.RLock()
	defer EditMutex.RUnlock()

	for _, peer := range c.Peers {
		if peer.Name() == name {
			return peer, nil
		}
	}

	return nil, ErrPeerNotFound
}

func (c *Config) RemovePeer(peer *Peer) {
	peers := []*Peer{}
	EditMutex.RLock()

	for _, origPeer := range c.Peers {
		if peer.Name() != origPeer.Name() {
			peers = append(peers, origPeer)
		}
	}

	EditMutex.RUnlock()
	c.SetPeers(peers)
}

func (c *Config) SetPeers(peers []*Peer) {
	EditMutex.Lock()
	defer EditMutex.Unlock()
	c.Peers = peers
}

func (c *Config) AddPeer(peer *Peer) {
	EditMutex.Lock()
	defer EditMutex.Unlock()
	c.Peers = append(c.Peers, peer)
}

func (c *Config) GetMe() *Peer {
	EditMutex.RLock()
	defer EditMutex.RUnlock()
	return c.Me
}

func (c *Config) GetPublisher() *Peer {
	EditMutex.RLock()
	defer EditMutex.RUnlock()
	return c.Publisher
}

func (c *Config) SetPublisher(publisher *Peer) {
	EditMutex.Lock()
	defer EditMutex.Unlock()
	c.Publisher = publisher
}

func (c *Config) ACMECacheChallenge(domain string, chal acme.Challenge) {
	EditMutex.Lock()
	defer EditMutex.Unlock()
	c.ACMEChallenges[domain] = chal
}
