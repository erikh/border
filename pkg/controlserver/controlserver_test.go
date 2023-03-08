package controlserver

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/erikh/border/pkg/config"
	"github.com/go-jose/go-jose/v3"
)

func makeConfig(t *testing.T) config.Config {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}

	return config.Config{AuthKey: &jose.JSONWebKey{Key: key, KeyID: "test", Algorithm: string(jose.A256KW)}}
}

func getNonce(server *Server) (*http.Response, error) {
	url := fmt.Sprintf("http://%s/nonce", server.listener.Addr())
	return http.Get(url)
}

func authCheck(server *Server, body io.Reader) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	req, err := http.NewRequest("PUT", fmt.Sprintf("http://%s/authCheck", server.listener.Addr()), body)
	if err != nil {
		return nil, err
	}

	return http.DefaultClient.Do(req.WithContext(ctx))
}

func TestStartupShutdown(t *testing.T) {
	server, err := Start(makeConfig(t), ":0", 10*time.Millisecond, 10*time.Millisecond)
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
	server, err := Start(makeConfig(t), ":0", 10*time.Millisecond, 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Shutdown(ctx)
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
	const jenny = 8675309

	server, err := Start(makeConfig(t), ":0", 10*time.Millisecond, 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Shutdown(ctx)
	})

	if server.config.ControlPort == jenny {
		t.Fatal("Regression in test; did you add data to the Config struct?")
	}

	resp, err := getNonce(server)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		out, _ := io.ReadAll(resp.Body)
		t.Fatalf("Nonce check failed: status code was not 200 was %d: error: %v", resp.StatusCode, string(out))
	}

	nonce, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	enc, err := jose.ParseEncrypted(string(nonce))
	if err != nil {
		t.Fatal(err)
	}

	nonce, err = enc.Decrypt(server.config.AuthKey)
	if err != nil {
		t.Fatal(err)
	}

	config := makeConfig(t)
	config.ControlPort = jenny

	encrypter, err := server.getEncrypter()
	if err != nil {
		t.Fatal(err)
	}

	buf, err := json.Marshal(ConfigUpdateRequest{NonceValue: nonce, Config: config})
	if err != nil {
		t.Fatal(err)
	}

	cipherText, err := encrypter.Encrypt(buf)
	if err != nil {
		t.Fatal(err)
	}

	out, err := cipherText.CompactSerialize()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	req, err := http.NewRequest("PUT", fmt.Sprintf("http://%s/configUpdate", server.listener.Addr()), bytes.NewBuffer([]byte(out)))
	if err != nil {
		t.Fatal(err)
	}

	resp, err = http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		byt, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		t.Fatalf("Status was not OK after update call, status was %v: %v", resp.StatusCode, string(byt))
	}

	resp.Body.Close()

	if server.config.ControlPort != jenny {
		t.Fatal("configuration was not updated")
	}
}
