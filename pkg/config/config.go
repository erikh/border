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
	ErrDump = errors.New("while dumping configuration to disk")
	ErrLoad = errors.New("while loading configuration from disk")
)

var FileMutex sync.RWMutex

type Config struct {
	FilenamePrefix string           `json:"-"` // prefix of filename to save to and read from
	ShutdownWait   time.Duration    `json:"shutdown_wait"`
	AuthKey        *jose.JSONWebKey `json:"auth_key"`
	Listen         ListenConfig     `json:"listen"`
	Publisher      net.IP           `json:"publisher"`
	Peers          map[string]*Peer `json:"peers"`
	Zones          map[string]*Zone `json:"zones"`

	reload chan struct{}
}

type ListenConfig struct {
	Control string `json:"control"`
	DNS     string `json:"dns"`
}

type Peer struct {
	IPs []net.IP         `json:"ips"`
	Key *jose.JSONWebKey `json:"key"`
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

	*c = *newConfig
	c.reload <- struct{}{}

	return nil
}
