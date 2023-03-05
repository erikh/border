package config

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"

	"github.com/ghodss/yaml"
)

var (
	ErrDump = errors.New("while dumping configuration to disk")
	ErrLoad = errors.New("while loading configuration from disk")
)

type Config struct {
	ControlPort uint            `json:"control_port"`
	Publisher   net.IP          `json:"publisher"`
	Peers       []Peer          `json:"peers"`
	Zones       map[string]Zone `json:"zones"`
}

type Peer struct {
	IP  net.IP `json:"ip"`
	Key string `json:"key"`
}

type Zone struct {
	SOA     SOA      `json:"soa"`
	Records []Record `json:"records"`
}

// FIXME can this be stored with miekg/dns types?
type SOA struct {
	Domain string `json:"domain"`
	Admin  string `json:"admin"`
	// FIXME look up the different types of expiry TTLs, I forget now
	TTL    uint `json:"ttl"`
	Serial uint `json:"serial"`
}

type Record struct {
	Name string `json:"name"`
	Type string `json:"type"`
	// FIXME additional data goes here, but needs to be an interface of something, dunno what yet.
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

func (c Config) SaveToJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

func (c Config) SaveToYAML() ([]byte, error) {
	return yaml.Marshal(c)
}

func ToDisk(filename string, dumperFunc func() ([]byte, error)) error {
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

func FromDisk(filename string, loaderFunc func([]byte) (Config, error)) (Config, error) {
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
