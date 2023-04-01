package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
)

type DumperFunc func(io.Writer) error
type LoaderFunc func(io.Reader) error

func (c *Config) LoadJSON(r io.Reader) error {
	rdr, wtr := io.Pipe()
	errChan := make(chan error, 1)
	go func() {
		_, err := c.chain.Add(rdr, HashFunc())
		errChan <- err
	}()

	if err := json.NewDecoder(io.TeeReader(r, wtr)).Decode(c); err != nil {
		wtr.CloseWithError(err)
		return err
	}

	wtr.Close()
	return <-errChan
}

func (c *Config) LoadYAML(r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, c)
}

func ToDisk(filename string, dumperFunc DumperFunc) error {
	f, err := os.Create(filename)
	if err != nil {
		return errors.Join(ErrDump, err)
	}
	defer f.Close()

	if err := dumperFunc(f); err != nil {
		return errors.Join(ErrDump, err)
	}

	return nil
}

func (c *Config) FromDisk(filename string, loaderFunc LoaderFunc) error {
	f, err := os.Open(filename)
	if err != nil {
		return errors.Join(ErrLoad, err)
	}
	defer f.Close()

	if err := loaderFunc(f); err != nil {
		return errors.Join(ErrLoad, err)
	}

	return c.postLoad(filename)
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

func (c *Config) SaveJSON(w io.Writer) error {
	c.trimZones()

	return json.NewEncoder(w).Encode(c)
}

func (c *Config) SaveYAML(w io.Writer) error {
	c.trimZones()

	out, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	_, err = io.Copy(w, bytes.NewBuffer(out))
	return err
}
