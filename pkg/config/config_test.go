package config

import (
	"errors"
	"io"
	"net"
	"os"
	"path"
	"reflect"
	"testing"

	"github.com/erikh/border/pkg/josekit"
	"github.com/erikh/go-hashchain"
)

func errorSave(io.Writer) error {
	return errors.New("intentional error")
}

func errorLoad(io.Reader) (*Config, error) {
	return nil, errors.New("intentional error")
}

func TestErrors(t *testing.T) {
	if err := ToDisk(os.DevNull, errorSave); err == nil {
		t.Fatal("ToDisk did not error")
	}

	c := New(hashchain.New(nil))

	if _, err := c.FromDisk(os.DevNull, errorLoad); err == nil {
		t.Fatal("FromDisk did not error")
	}
}

func TestMarshal(t *testing.T) {
	// it doesn't matter what we save for this test.
	// NOTE: yaml save/load is a little more finicky than json. These types must
	// be filled in or the comparisons will not be equal. They cannot just be
	// nil.
	key, err := josekit.MakeKey("peer")
	if err != nil {
		t.Fatal(err)
	}

	c := New(hashchain.New(nil))
	c.Peers = []*Peer{
		{
			ControlServer: ":5309",
			Key:           key,
			IPs:           []net.IP{net.ParseIP("127.0.0.1")},
		},
	}
	c.Zones = map[string]*Zone{}

	dir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	c.FilenamePrefix = path.Join(dir, "config")

	p := path.Join(dir, "config.json")

	if err := ToDisk(p, c.SaveJSON); err != nil {
		t.Fatal(err)
	}

	c2 := New(hashchain.New(nil))

	c2, err = c2.FromDisk(p, LoadJSON)
	if err != nil {
		t.Fatal(err)
	}

	peer1, err := c.FindPeer("peer")
	if err != nil {
		t.Fatalf("during search of peer1: %v", err)
	}

	peer2, err := c.FindPeer("peer")
	if err != nil {
		t.Fatalf("during search of peer2: %v", err)
	}

	// why can't reflect.DeepEqual compare pointer innards. Frustrating!
	// This is not an exhaustive comparison because I am a human being and will
	// die eventually, and I don't want to spend the rest of my life adding shit
	// here
	if c.Listen.Control != c2.Listen.Control || !reflect.DeepEqual(peer1.IPs, peer2.IPs) || !reflect.DeepEqual(peer1.Key.Key, peer2.Key.Key) {
		t.Logf("%#v - %#v", c, c2)
		t.Fatal("loaded content did not equal saved")
	}

	p = path.Join(dir, "config.yaml")

	if err := ToDisk(p, c.SaveYAML); err != nil {
		t.Fatal(err)
	}

	c2 = New(hashchain.New(nil))

	c2, err = c2.FromDisk(p, LoadYAML)
	if err != nil {
		t.Fatal(err)
	}

	peer1, err = c.FindPeer("peer")
	if err != nil {
		t.Fatalf("during search of peer1: %v", err)
	}

	peer2, err = c.FindPeer("peer")
	if err != nil {
		t.Fatalf("during search of peer2: %v", err)
	}

	// why can't reflect.DeepEqual compare pointer innards. Frustrating!
	// This is not an exhaustive comparison because I am a human being and will
	// die eventually, and I don't want to spend the rest of my life adding shit
	// here
	if c.Listen.Control != c2.Listen.Control || !reflect.DeepEqual(peer1.IPs, peer2.IPs) || !reflect.DeepEqual(peer1.Key.Key, peer2.Key.Key) {
		t.Logf("%#v - %#v", c, c2)
		t.Fatal("loaded content did not equal saved")
	}
}
