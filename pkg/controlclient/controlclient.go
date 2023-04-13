package controlclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/erikh/border/pkg/acmekit"
	"github.com/erikh/border/pkg/api"
	"github.com/erikh/border/pkg/config"
	"github.com/erikh/border/pkg/josekit"
	"github.com/ghodss/yaml"
	"github.com/go-jose/go-jose/v3"
	"github.com/sirupsen/logrus"
)

var (
	ErrAcquireNonce = errors.New("Could not acquire nonce")
	ErrMarshal      = errors.New("Could not marshal payload")
	ErrBadResponse  = errors.New("Invalid response from API server")
	ErrDecrypt      = errors.New("Could not decrypt response")
	ErrEncrypt      = errors.New("Could not encrypt payload")
)

type Client struct {
	AuthKey *jose.JSONWebKey `json:"auth_key"`
	BaseURL string           `json:"base_url"`
}

func Load(filename string) (*Client, error) {
	byt, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("Could not read client configuration: %w", err)
	}

	var c Client

	if err := yaml.Unmarshal(byt, &c); err != nil {
		return nil, fmt.Errorf("Could not unmarshal client configuration: %w", err)
	}

	return &c, nil
}

func FromPeer(peer *config.Peer) *Client {
	return &Client{
		AuthKey: peer.Key,
		// FIXME should be https
		BaseURL: fmt.Sprintf("http://%s", peer.ControlServer),
	}
}

func (c *Client) GetNonce(peer bool) ([]byte, error) {
	baseurl, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("Base URL %q is invalid: %w", c.BaseURL, err)
	}

	path := api.PathNonce
	if peer {
		path = api.PathPeerNonce
	}

	u := baseurl.JoinPath("/" + path)
	resp, err := http.Get(u.String())
	if err != nil {
		return nil, errors.Join(ErrAcquireNonce, err)
	}

	if resp.StatusCode != http.StatusOK {
		byt, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.Join(ErrAcquireNonce, err)
		}

		return nil, fmt.Errorf("Status was not OK after nonce call, status was %v: %v", resp.StatusCode, string(byt))
	}

	nonce, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Join(ErrAcquireNonce, err)
	}

	enc, err := jose.ParseEncrypted(string(nonce))
	if err != nil {
		return nil, errors.Join(ErrDecrypt, err)
	}

	nonce, err = enc.Decrypt(c.AuthKey)
	if err != nil {
		return nil, errors.Join(ErrDecrypt, err)
	}

	return nonce, nil
}

func (c *Client) PrepareRequest(msg api.Request, peer bool) ([]byte, error) {
	nonce, err := c.GetNonce(peer)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve nonce: %w", err)
	}

	if err := msg.SetNonce(nonce); err != nil {
		return nil, fmt.Errorf("Could not set nonce: %w", err)
	}

	encrypter, err := josekit.GetEncrypter(c.AuthKey)
	if err != nil {
		return nil, errors.Join(ErrEncrypt, err)
	}

	byt, err := msg.Marshal()
	if err != nil {
		return nil, errors.Join(ErrMarshal, err)
	}

	cipherText, err := encrypter.Encrypt(byt)
	if err != nil {
		return nil, errors.Join(ErrEncrypt, err)
	}

	out, err := cipherText.CompactSerialize()
	if err != nil {
		return nil, errors.Join(ErrEncrypt, err)
	}

	return []byte(out), nil
}

func (c *Client) SendRequest(msg api.Request, peer bool) (*http.Response, error) {
	baseurl, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("Base URL %q is invalid: %w", c.BaseURL, err)
	}

	u := baseurl.JoinPath("/" + msg.Endpoint())

	out, err := c.PrepareRequest(msg, peer)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PUT", u.String(), bytes.NewBuffer(out))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, errors.Join(ErrBadResponse, err)
	}

	if resp.StatusCode != http.StatusOK {
		byt, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.Join(ErrBadResponse, err)
		}

		return nil, fmt.Errorf("Status was not OK after %T call, status was %v: %v", msg, resp.StatusCode, string(byt))
	}

	return resp, nil
}

func (c *Client) Exchange(msg api.Request, peer bool) (api.Message, error) {
	resp, err := c.SendRequest(msg, peer)
	if err != nil {
		return nil, fmt.Errorf("Failed to deliver request: %w", err)
	}

	byt, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Join(ErrBadResponse, err)
	}

	enc, err := jose.ParseEncrypted(string(byt))
	if err != nil {
		return nil, errors.Join(ErrDecrypt, err)
	}

	byt, err = enc.Decrypt(c.AuthKey)
	if err != nil {
		return nil, errors.Join(ErrDecrypt, err)
	}

	res := msg.Response()

	if err := json.Unmarshal(byt, res); err != nil {
		return nil, errors.Join(ErrBadResponse, err)
	}

	return res, nil
}

// ACMEPublisherWaitForReady is the publisher function to wait to serve the
// challenge. It caches the challenge, and then waits for all peers to report
// that they're ready before serving the challenge, lest it be captured
// prematurely.
func ACMEPublisherWaitForReady(ctx context.Context, conf *config.Config, domain string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		config.EditMutex.RLock()
		peers, ok := conf.ACMEReady[domain]
		if ok {
			for _, peer := range conf.Peers {
				var found bool

				for _, p := range peers {
					if p.Name() == peer.Name() {
						found = true
						break
					}
				}

				if found {
					return nil
				} else {
					break
				}
			}
		}
		config.EditMutex.RUnlock()
	}
}

// ACMEFollowerWaitForReady is the follower (aka, everybody but the publisher)
// method for waiting for a challenge. It requests the challenge data, reports
// that it's ready to serve, and waits for the signal to serve the data.
func ACMEFollowerWaitForReady(ctx context.Context, conf *config.Config, domain string) error {
requestRetry:
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	client := FromPeer(conf.GetPublisher())

	resp, err := client.Exchange(&api.ACMEChallengeRequest{Domain: domain}, true)
	if err != nil {
		logrus.Errorf("Could not request ACME challenge from publisher: %v", err)
		time.Sleep(time.Second)
		goto requestRetry
	}

	conf.ACMECacheChallenge(domain, resp.(*api.ACMEChallengeResponse).Challenge)

	if _, err := client.Exchange(&api.ACMEReadyRequest{Peer: conf.GetMe().Name(), Domain: domain}, true); err != nil {
		logrus.Errorf("Unable to report to publisher that we are ready: %v", err)
		time.Sleep(time.Second)
		goto requestRetry
	}

	for {
		resp, err = client.Exchange(&api.ACMEServeRequest{Domain: domain}, true)
		if err != nil {
			logrus.Errorf("Unable to get ready state from publisher: %v", err)
			time.Sleep(time.Second)
			goto requestRetry // do not try this loop again, start over completely, in case the publisher changed
		}

		if resp.(*api.ACMEServeResponse).Ok {
			return nil
		}

		time.Sleep(time.Second)
	}
}

// ACMEFollowerCaptureCert gets the cert from the publisher, and then indicates
// that the challenge endpoint can be torn down.
func ACMEFollowerCaptureCert(ctx context.Context, conf *config.Config, domain string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		client := FromPeer(conf.GetPublisher())

		resp, err := client.Exchange(&api.ACMEChallengeCompleteRequest{Domain: domain}, true)
		if err != nil {
			logrus.Errorf("Unable to get completed state from publisher: %v", err)
			time.Sleep(time.Second)
			continue
		}

		complete := resp.(*api.ACMEChallengeCompleteResponse)

		if !complete.Ok {
			logrus.Warn("Publisher has not completed ACME challenge, retrying in a second")
			time.Sleep(time.Second)
			continue
		}

		config.EditMutex.Lock()
		defer config.EditMutex.Unlock()

		conf.ACME.Account.Certificates[domain] = &acmekit.Certificate{
			Chain:      complete.Chain,
			PrivateKey: complete.PrivateKey,
		}

		return nil
	}
}

// ACMEWaitForReady encapsulates the waiting loop solvers will use to determine
// when the entire cluster is ready to serve a challenge.
//
// It is not pretty.
func ACMEWaitForReady(ctx context.Context, conf *config.Config, domain string) error {
retry:
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if conf.GetPublisher() == nil {
		time.Sleep(time.Second)
		goto retry
	}

	if conf.GetPublisher().Name() == conf.GetMe().Name() {
		return ACMEPublisherWaitForReady(ctx, conf, domain)
	} else {
		return ACMEFollowerWaitForReady(ctx, conf, domain)
	}
}
