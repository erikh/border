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
	return controlclient.ACMEWaitForReady(ctx, dns.config, dns.domain, chal)
}

type ACMEALPNSolver struct {
	domain string
	config *config.Config
}

func ALPNSolver(c *config.Config, domain string) *ACMEALPNSolver {
	return &ACMEALPNSolver{domain: domain, config: c}
}

func (dns *ACMEALPNSolver) Present(ctx context.Context, chal acme.Challenge) error {
	return nil
}

func (dns *ACMEALPNSolver) CleanUp(ctx context.Context, chal acme.Challenge) error {
	return nil
}

func (dns *ACMEALPNSolver) Wait(ctx context.Context, chal acme.Challenge) error {
	return controlclient.ACMEWaitForReady(ctx, dns.config, dns.domain, chal)
}

type ACMEHTTPSolver struct {
	domain string
	config *config.Config
}

func HTTPSolver(c *config.Config, domain string) *ACMEHTTPSolver {
	return &ACMEHTTPSolver{domain: domain, config: c}
}

func (dns *ACMEHTTPSolver) Present(ctx context.Context, chal acme.Challenge) error {
	return nil
}

func (dns *ACMEHTTPSolver) CleanUp(ctx context.Context, chal acme.Challenge) error {
	return nil
}

func (dns *ACMEHTTPSolver) Wait(ctx context.Context, chal acme.Challenge) error {
	return controlclient.ACMEWaitForReady(ctx, dns.config, dns.domain, chal)
}
