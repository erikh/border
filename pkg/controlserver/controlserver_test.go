package controlserver

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/erikh/border/pkg/api"
	"github.com/erikh/border/pkg/config"
	"github.com/erikh/border/pkg/controlclient"
	"github.com/erikh/border/pkg/josekit"
	"github.com/erikh/go-hashchain"
	"github.com/go-jose/go-jose/v3"
)

func makeConfig(t *testing.T) *config.Config {
	jwk, err := josekit.MakeKey("foo")
	if err != nil {
		t.Fatal(err)
	}

	dir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	c := config.New(hashchain.New(nil))

	c.Peers = []*config.Peer{{
		Key: jwk,
		IPs: []net.IP{net.ParseIP("127.0.0.1")},
	}}
	c.FilenamePrefix = filepath.Join(dir, "config")
	c.AuthKey = jwk

	return c
}

func makeClient(addr string, authKey *jose.JSONWebKey) controlclient.Client {
	return controlclient.Client{
		AuthKey: authKey,
		BaseURL: fmt.Sprintf("http://%s", addr),
	}
}

func getNonce(server *Server) (*http.Response, error) {
	url := fmt.Sprintf("http://%s/%s", server.listener.Addr(), api.PathNonce)
	return http.Get(url)
}

func authCheck(server *Server, body io.Reader) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	req, err := http.NewRequest("PUT", fmt.Sprintf("http://%s/%s", server.listener.Addr(), api.PathAuthCheck), body)
	if err != nil {
		return nil, err
	}

	return http.DefaultClient.Do(req.WithContext(ctx))
}

func testHandler(t *testing.T, c *config.Config, route, typ string, payload api.Request) *Server {
	server, err := Start(c, c.Peers[0], ":0", 10*time.Millisecond, 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Shutdown(ctx) // nolint:errcheck
	})

	client := makeClient(server.listener.Addr().String(), server.config.AuthKey)

	if _, err := client.Exchange(payload, false); err != nil {
		t.Fatal(err)
	}

	return server
}

func TestStartupShutdown(t *testing.T) {
	c := makeConfig(t)

	server, err := Start(c, c.Peers[0], ":0", 10*time.Millisecond, 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := getNonce(server)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		out, _ := io.ReadAll(resp.Body)
		t.Fatalf("Nonce check failed: status code was not 200 was %d: error: %v", resp.StatusCode, string(out))
	}

	resp.Body.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}

	if _, err := getNonce(server); err == nil {
		t.Fatal("Server is still up; should no longer be")
	}
}

func TestNonce(t *testing.T) {
	c := makeConfig(t)
	server, err := Start(c, c.Peers[0], ":0", 10*time.Millisecond, 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Shutdown(ctx) // nolint:errcheck
	})

	resp, err := getNonce(server)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		out, _ := io.ReadAll(resp.Body)
		t.Fatalf("Nonce check failed: status code was not 200 was %d: error: %v", resp.StatusCode, string(out))
	}

	resp, err = authCheck(server, resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		out, _ := io.ReadAll(resp.Body)
		t.Fatalf("Nonce check failed: status code was not 200 was %d: error: %v", resp.StatusCode, string(out))
	}

	resp.Body.Close()

	// fetch a nonce, wait for it to time out, and try again. Should fail.
	resp, err = getNonce(server)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		out, _ := io.ReadAll(resp.Body)
		t.Fatalf("Nonce check failed: status code was not 200 was %d: error: %v", resp.StatusCode, string(out))
	}

	time.Sleep(20 * time.Millisecond) // nonce should be expired or already pruned now

	defer resp.Body.Close()

	resp, err = authCheck(server, resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	// resp should not have a 200 status here. Should be 403.
	if resp.StatusCode != http.StatusForbidden {
		out, _ := io.ReadAll(resp.Body)
		t.Fatalf("Auth check did not fail: status code was not 403, was %d: error: %v", resp.StatusCode, string(out))
	}

	resp.Body.Close()
}

func TestConfigUpdate(t *testing.T) {
	c := makeConfig(t)
	const jenny = ":5309"

	c.Listen.Control = jenny

	server := testHandler(
		t,
		c,
		api.PathConfigUpdate,
		"update configuration",
		&api.ConfigUpdateRequest{Config: c},
	)

	if server.config.Listen.Control != jenny {
		t.Fatal("configuration was not updated")
	}
}

func TestConfigReload(t *testing.T) {
	c := makeConfig(t)

	if err := c.Save(); err != nil {
		t.Fatal(err)
	}

	server := testHandler(
		t,
		c,
		api.PathConfigReload,
		"reload configuration",
		&api.ConfigReloadRequest{},
	)

	select {
	case <-time.After(time.Second):
		t.Fatal("Reload was never triggered")
	case <-server.config.ReloadChan():
	}
}

func TestPeerRegistration(t *testing.T) {
	c := makeConfig(t)

	jwk, err := josekit.MakeKey("foo")
	if err != nil {
		t.Fatal(err)
	}

	peer := &config.Peer{
		IPs: []net.IP{net.ParseIP("127.0.0.1")},
		Key: jwk,
	}

	server := testHandler(
		t,
		c,
		api.PathPeerRegistration,
		"peer registration",
		&api.PeerRegistrationRequest{Peer: peer},
	)

	var ok bool

	for _, newPeer := range server.config.Peers {
		// reflect.DeepEqual doesn't work well with pointer values, so we marshal
		// the keys out (which are pointers) and compare the resulting strings.
		// last I checked, encoding/json was deterministic, so this should be
		// solid.
		key, err := peer.Key.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}

		newKey, err := newPeer.Key.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}

		if ok = reflect.DeepEqual(peer.IPs, newPeer.IPs) && string(key) == string(newKey); ok {
			break
		}
	}

	if !ok {
		t.Fatal("never found peer")
	}
}
