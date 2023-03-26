package config

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/erikh/border/pkg/dnsconfig"
	"github.com/go-jose/go-jose/v3"
)

var (
	ErrDump         = errors.New("while dumping configuration to disk")
	ErrLoad         = errors.New("while loading configuration from disk")
	ErrPeerNotFound = errors.New("peer not found")
)

var FileMutex sync.RWMutex
var EditMutex sync.Mutex

type Config struct {
	FilenamePrefix string           `json:"-"` // prefix of filename to save to and read from
	ShutdownWait   time.Duration    `json:"shutdown_wait"`
	AuthKey        *jose.JSONWebKey `json:"auth_key"`
	Listen         ListenConfig     `json:"listen"`
	Publisher      *Peer            `json:"publisher"`
	Peers          []*Peer          `json:"peers"`
	Zones          map[string]*Zone `json:"zones"`

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

func (c *Config) InitReload() {
	c.reload = make(chan struct{}, 1)
}

func (c *Config) ReloadChan() <-chan struct{} {
	return c.reload
}

func (c *Config) Reload() error {
	newConfig, err := FromDisk(c.FilenamePrefix+".yaml", LoadYAML)
	if err != nil {
		return fmt.Errorf("While reloading configuration: %w", err)
	}

	EditMutex.Lock()
	defer EditMutex.Unlock()
	*c = *newConfig
	c.reload <- struct{}{}

	return nil
}

func (c *Config) FindPeer(name string) (*Peer, error) {
	// FIXME should take a mutex
	for _, peer := range c.Peers {
		if peer.Name() == name {
			return peer, nil
		}
	}

	return nil, ErrPeerNotFound
}

func (c *Config) SetPeers(peers []*Peer) {
	EditMutex.Lock()
	defer EditMutex.Unlock()
	c.Peers = peers
}

func (c *Config) SetPublisher(publisher *Peer) {
	EditMutex.Lock()
	defer EditMutex.Unlock()
	c.Publisher = publisher
}
