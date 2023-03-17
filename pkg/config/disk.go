package config

import (
	"encoding/json"
	"errors"
	"io"
	"os"

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
