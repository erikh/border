package config

import (
	"errors"
	"os"
	"path"
	"reflect"
	"testing"
)

func errorSave() ([]byte, error) {
	return nil, errors.New("intentional error")
}

func errorLoad(data []byte) (Config, error) {
	return Config{}, errors.New("intentional error")
}

func TestErrors(t *testing.T) {
	if err := ToDisk(os.DevNull, errorSave); err == nil {
		t.Fatal("ToDisk did not error")
	}

	if _, err := FromDisk(os.DevNull, errorLoad); err == nil {
		t.Fatal("FromDisk did not error")
	}
}

func TestMarshal(t *testing.T) {
	// it doesn't matter what we save for this test.
	// NOTE: yaml save/load is a little more finicky than json. These types must
	// be filled in or the comparisons will not be equal. They cannot just be
	// nil.
	c := Config{
		ControlPort: 8675309,
		Peers:       []Peer{},
		Zones:       map[string]Zone{},
	}

	dir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	p := path.Join(dir, "config.json")

	if err := ToDisk(p, c.SaveToJSON); err != nil {
		t.Fatal(err)
	}

	c2, err := FromDisk(p, LoadJSON)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(c, c2) {
		t.Fatal("loaded content did not equal saved")
	}

	p = path.Join(dir, "config.yaml")

	if err := ToDisk(p, c.SaveToYAML); err != nil {
		t.Fatal(err)
	}

	c2, err = FromDisk(p, LoadYAML)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(c, c2) {
		t.Logf("%#v - %#v", c, c2)
		t.Fatal("loaded content did not equal saved")
	}
}