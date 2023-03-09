package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"

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
	AuthKey        *jose.JSONWebKey `json:"auth_key"`
	ControlPort    uint             `json:"control_port"`
	Publisher      net.IP           `json:"publisher"`
	Peers          []Peer           `json:"peers"`
	Zones          map[string]Zone  `json:"zones"`
}

type Peer struct {
	IP  net.IP           `json:"ip"`
	Key *jose.JSONWebKey `json:"key"`
}

type Zone struct {
	SOA     dnsconfig.SOA `json:"soa"`
	NS      dnsconfig.NS  `json:"ns"`
	Records []Record      `json:"records"`
}

type Record struct {
	Name  string           `json:"name"`
	Value dnsconfig.Record `json:"value"`
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

	if err := ToDisk(c.FilenamePrefix+".json", c.SaveToJSON); err != nil {
		return err
	}

	if err := ToDisk(c.FilenamePrefix+".yaml", c.SaveToYAML); err != nil {
		return err
	}

	return nil
}

func (c Config) SaveToJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

func (c Config) SaveToYAML() ([]byte, error) {
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

	return c, nil
}
