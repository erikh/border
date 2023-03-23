package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
)

type DumperFunc func() ([]byte, error)
type LoaderFunc func([]byte) (*Config, error)

func LoadJSON(data []byte) (*Config, error) {
	var c Config
	err := json.Unmarshal(data, &c)
	return &c, err
}

func LoadYAML(data []byte) (*Config, error) {
	var c Config
	err := yaml.Unmarshal(data, &c)
	return &c, err
}

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

func FromDisk(filename string, loaderFunc LoaderFunc) (*Config, error) {
	var c *Config

	f, err := os.Open(filename)
	if err != nil {
		return nil, errors.Join(ErrLoad, err)
	}
	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil {
		return nil, errors.Join(ErrLoad, err)
	}

	c, err = loaderFunc(b)
	if err != nil {
		return nil, errors.Join(ErrLoad, err)
	}

	return c, c.postLoad(filename)
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
	c.InitReload()

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
