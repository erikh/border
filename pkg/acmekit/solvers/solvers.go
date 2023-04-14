package solvers

import (
	"context"

	"github.com/erikh/border/pkg/config"
	"github.com/erikh/border/pkg/controlclient"
	"github.com/mholt/acmez/acme"
)

type ACMEDNSSolver struct {
	domain string
	config *config.Config
}

func DNSSolver(c *config.Config, domain string) *ACMEDNSSolver {
	return &ACMEDNSSolver{domain: domain, config: c}
}

func (dns *ACMEDNSSolver) Present(ctx context.Context, chal acme.Challenge) error {
	return nil
}

func (dns *ACMEDNSSolver) CleanUp(ctx context.Context, chal acme.Challenge) error {
	return nil
}

func (dns *ACMEDNSSolver) Wait(ctx context.Context, chal acme.Challenge) error {
	dns.config.ACMESetChallenge(dns.domain, chal)
	return controlclient.ACMEWaitForReady(ctx, dns.config, dns.domain)
}

func (dns *ACMEDNSSolver) PresentCached(ctx context.Context) error {
	return controlclient.ACMEWaitForReady(ctx, dns.config, dns.domain)
}

type ACMEALPNSolver struct {
	domain string
	config *config.Config
}

func ALPNSolver(c *config.Config, domain string) *ACMEALPNSolver {
	return &ACMEALPNSolver{domain: domain, config: c}
}

func (alpn *ACMEALPNSolver) Present(ctx context.Context, chal acme.Challenge) error {
	return nil
}

func (alpn *ACMEALPNSolver) CleanUp(ctx context.Context, chal acme.Challenge) error {
	return nil
}

func (alpn *ACMEALPNSolver) Wait(ctx context.Context, chal acme.Challenge) error {
	alpn.config.ACMESetChallenge(alpn.domain, chal)
	return controlclient.ACMEWaitForReady(ctx, alpn.config, alpn.domain)
}

func (alpn *ACMEALPNSolver) PresentCached(ctx context.Context) error {
	return controlclient.ACMEWaitForReady(ctx, alpn.config, alpn.domain)
}

type ACMEHTTPSolver struct {
	domain string
	config *config.Config
}

func HTTPSolver(c *config.Config, domain string) *ACMEHTTPSolver {
	return &ACMEHTTPSolver{domain: domain, config: c}
}

func (hs *ACMEHTTPSolver) Present(ctx context.Context, chal acme.Challenge) error {
	return nil
}

func (hs *ACMEHTTPSolver) CleanUp(ctx context.Context, chal acme.Challenge) error {
	return nil
}

func (hs *ACMEHTTPSolver) Wait(ctx context.Context, chal acme.Challenge) error {
	hs.config.ACMESetChallenge(hs.domain, chal)
	return controlclient.ACMEWaitForReady(ctx, hs.config, hs.domain)
}

func (hs *ACMEHTTPSolver) PresentCached(ctx context.Context) error {
	return controlclient.ACMEWaitForReady(ctx, hs.config, hs.domain)
}
