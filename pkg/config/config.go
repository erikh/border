package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/erikh/border/pkg/dnsconfig"
	"github.com/erikh/border/pkg/lb"
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
	SOA     dnsconfig.SOA `json:"soa"`
	NS      dnsconfig.NS  `json:"ns"`
	Records []*Record     `json:"records"`
}

type Record struct {
	Type         string           `json:"type"`
	Name         string           `json:"name"`
	LiteralValue map[string]any   `json:"value"`
	Value        dnsconfig.Record `json:"-"`
}

func trimDot(key string) string {
	if strings.HasSuffix(key, ".") {
		key = strings.TrimSuffix(key, ".")
	}

	return key
}

func addDot(key string) string {
	if !strings.HasSuffix(key, ".") {
		key += "."
	}

	return key
}

// Trim the trailing dot from zone records. Used in saving the configuration.
func (c *Config) trimZones() {
	newZones := map[string]*Zone{}

	for key, zone := range c.Zones {
		zone.SOA.Domain = trimDot(zone.SOA.Domain)
		zone.SOA.Admin = trimDot(zone.SOA.Admin)

		newServers := []string{}

		for _, server := range zone.NS.Servers {
			newServers = append(newServers, trimDot(server))
		}

		zone.NS.Servers = newServers

		for _, record := range zone.Records {
			record.Name = trimDot(record.Name)
		}

		newZones[trimDot(key)] = zone
	}

	c.Zones = newZones
}

// Decorate zones with a trailing dot. Used in loading the configuration.
func (c *Config) decorateZones() {
	newZones := map[string]*Zone{}

	for key, zone := range c.Zones {
		zone.SOA.Domain = addDot(zone.SOA.Domain)
		zone.SOA.Admin = addDot(zone.SOA.Admin)

		newServers := []string{}

		for _, server := range zone.NS.Servers {
			newServers = append(newServers, addDot(server))
		}

		zone.NS.Servers = newServers

		for _, record := range zone.Records {
			record.Name = addDot(record.Name)
		}

		newZones[addDot(key)] = zone
	}

	c.Zones = newZones
}

// decompose the "literal value" into a dnsconfig struct record, to be
// converted later into all sorts of useful stuff.
//
// this could probably be done much better with struct tags; I'm just too lazy
// at this point.
func (c *Config) convertLiterals() error {
	for _, z := range c.Zones {
		if z.NS.TTL == 0 {
			z.NS.TTL = z.SOA.MinTTL
		}

		for _, r := range z.Records {
			switch r.Type {
			case dnsconfig.TypeA:
				orig, ok := r.LiteralValue["addresses"]
				if !ok || len(orig.([]any)) == 0 {
					return fmt.Errorf("No addresses for %q A record", r.Name)
				}

				addresses := []net.IP{}

				for _, addr := range r.LiteralValue["addresses"].([]any) {
					addresses = append(addresses, net.ParseIP(addr.(string)))
				}

				var ttl uint32
				origTTL, ok := r.LiteralValue["ttl"]

				if ok {
					ttl = uint32(origTTL.(float64))
				} else {
					ttl = z.SOA.MinTTL
				}

				r.Value = &dnsconfig.A{
					Addresses: addresses,
					TTL:       ttl,
				}
			case dnsconfig.TypeLB:
				listeners := []string{}

				for _, listener := range r.LiteralValue["listeners"].([]string) {
					host, port, err := net.SplitHostPort(listener)
					if err != nil {
						return fmt.Errorf("Could not parse listener %q: %v", listener, err)
					}

					peer, ok := c.Peers[host]
					if !ok {
						return fmt.Errorf("Peer %q does not exist in peers list", host)
					}

					listeners = append(listeners, net.JoinHostPort(peer.IP.String(), port))
				}

				kind, ok := r.LiteralValue["kind"].(lb.Kind)
				if !ok {
					return errors.New("kind was not specified in LB record")
				}

				switch kind {
				case lb.BalanceTCP:
				default:
					return fmt.Errorf("Kind was invalid: %q", kind)
				}

				backends, ok := r.LiteralValue["backends"].([]string)
				if !ok {
					return errors.New("lb backends were unspecified or invalid")
				}

				sc, ok := r.LiteralValue["simultaneous_connections"].(uint)
				if !ok {
					sc = dnsconfig.DefaultSimultaneousConnections
				}

				mc, ok := r.LiteralValue["max_connections_per_address"].(uint64)
				if !ok {
					mc = dnsconfig.DefaultMaxConnectionsPerAddress
				}

				ttl, ok := r.LiteralValue["ttl"].(uint32)
				if !ok {
					ttl = z.SOA.MinTTL
				}

				r.Value = &dnsconfig.LB{
					Listeners:                listeners,
					Kind:                     kind,
					Backends:                 backends,
					SimultaneousConnections:  sc,
					MaxConnectionsPerAddress: mc,
					TTL:                      ttl,
				}
			default:
				return fmt.Errorf("invalid type for record %q", r.Name)
			}
		}
	}

	return nil
}

func LoadJSON(data []byte) (Config, error) {
	var c Config
	err := json.Unmarshal(data, &c)
	return c, err
}

func LoadYAML(data []byte) (Config, error) {
	var c Config
	err := yaml.Unmarshal(data, &c)
	return c, err
}

func (c Config) Save() error {
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

type DumperFunc func() ([]byte, error)
type LoaderFunc func([]byte) (Config, error)

func ToDisk(filename string, dumperFunc DumperFunc) error {
	b, err := dumperFunc()
	if err != nil {
		return errors.Join(ErrDump, err)
	}

	f, err := os.Create(filename)
	if err != nil {
		return errors.Join(ErrDump, err)
	}
	defer f.Close()

	if _, err := f.Write(b); err != nil {
		return errors.Join(ErrDump, err)
	}

	return nil
}

func FromDisk(filename string, loaderFunc LoaderFunc) (Config, error) {
	var c Config

	f, err := os.Open(filename)
	if err != nil {
		return c, errors.Join(ErrLoad, err)
	}
	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil {
		return c, errors.Join(ErrLoad, err)
	}

	c, err = loaderFunc(b)
	if err != nil {
		return c, errors.Join(ErrLoad, err)
	}

	// I'm going to hell for this
	c.FilenamePrefix = strings.TrimSuffix(filename, filepath.Ext(filename))

	if c.Peers == nil {
		c.Peers = map[string]*Peer{}
	}

	c.convertLiterals()
	c.decorateZones()

	return c, nil
}
