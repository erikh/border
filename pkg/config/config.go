package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/erikh/border/pkg/dnsconfig"
	"github.com/ghodss/yaml"
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
}

type ListenConfig struct {
	Control string `json:"control"`
	DNS     string `json:"dns"`
}

type Peer struct {
	IP  net.IP           `json:"ip"`
	Key *jose.JSONWebKey `json:"key"`
}

type Zone struct {
	SOA     *dnsconfig.SOA `json:"soa"`
	NS      *dnsconfig.NS  `json:"ns"`
	Records []*Record      `json:"records"`
}

func (c *Config) postLoad(filename string) error {
	// XXX I'm going to hell for this
	c.FilenamePrefix = strings.TrimSuffix(filename, filepath.Ext(filename))

	if len(c.Peers) == 0 {
		return errors.New("You must specify at least one peer")
	}

	if err := c.convertLiterals(); err != nil {
		return err
	}

	c.decorateZones()

	return nil
}

func (c *Config) Save() error {
	if c.FilenamePrefix == "" {
		return errors.New("invalid filename prefix")
	}

	fi, err := os.Stat(c.FilenamePrefix)
	if (fi != nil && fi.IsDir()) || err == nil {
		return fmt.Errorf("Filename prefix %q exists or is a directory, should not be", c.FilenamePrefix)
	}

	FileMutex.Lock()
	defer FileMutex.Unlock()

	if err := ToDisk(c.FilenamePrefix+".json.tmp", c.SaveJSON); err != nil {
		return err
	}

	if err := os.Rename(c.FilenamePrefix+".json.tmp", c.FilenamePrefix+".json"); err != nil {
		return fmt.Errorf("Could not move configuration file into place: %w", err)
	}

	if err := ToDisk(c.FilenamePrefix+".yaml.tmp", c.SaveYAML); err != nil {
		return err
	}

	if err := os.Rename(c.FilenamePrefix+".yaml.tmp", c.FilenamePrefix+".yaml"); err != nil {
		return fmt.Errorf("Could not move configuration file into place: %w", err)
	}

	return nil
}

func (c *Config) SaveJSON() ([]byte, error) {
	c.trimZones()
	return json.MarshalIndent(c, "", "  ")
}

func (c *Config) SaveYAML() ([]byte, error) {
	c.trimZones()
	return yaml.Marshal(c)
}
