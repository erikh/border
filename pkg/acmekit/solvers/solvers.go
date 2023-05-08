package solvers

import (
	"context"

	"github.com/erikh/border/pkg/acmekit"
	"github.com/erikh/border/pkg/config"
	"github.com/erikh/border/pkg/controlclient"
	"github.com/erikh/border/pkg/dnsconfig"
	"github.com/mholt/acmez/acme"
)

type ACMEDNSSolver struct {
	domain string
	config *config.Config
}

func CachedSolver(ctx context.Context, c *config.Config, domain string, solver acmekit.ClusterSolver) error {
	if err := controlclient.ACMEWaitForReady(ctx, c, domain, solver); err != nil {
		return err
	}

	return controlclient.ACMEFollowerCaptureCert(ctx, c, domain, solver)
}

func DNSSolver(c *config.Config, domain string) *ACMEDNSSolver {
	return &ACMEDNSSolver{domain: domain, config: c}
}

func (dns *ACMEDNSSolver) Present(ctx context.Context, chal acme.Challenge) error {
	domainZone := dns.config.Zones[dns.domain]

	domainZone.Records = append(domainZone.Records, &config.Record{
		Type: dnsconfig.TypeTXT,
		Name: chal.DNS01TXTRecordName(),
		Value: &dnsconfig.TXT{
			Value: []string{chal.DNS01KeyAuthorization()},
		},
	})

	return nil
}

func (dns *ACMEDNSSolver) CleanUp(ctx context.Context, chal acme.Challenge) error {
	records := dns.config.Zones[dns.domain].Records
	newRecords := []*config.Record{}

	for _, rec := range records {
		if rec.Name != chal.DNS01TXTRecordName() {
			newRecords = append(newRecords, rec)
		}
	}

	dns.config.Zones[dns.domain].Records = newRecords

	return nil
}

func (dns *ACMEDNSSolver) Wait(ctx context.Context, chal acme.Challenge) error {
	dns.config.ACMESetChallenge(dns.domain, chal)
	return controlclient.ACMEWaitForReady(ctx, dns.config, dns.domain, dns)
}

func (dns *ACMEDNSSolver) PresentCached(ctx context.Context) error {
	return CachedSolver(ctx, dns.config, dns.domain, dns)
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
	return controlclient.ACMEWaitForReady(ctx, alpn.config, alpn.domain, alpn)
}

func (alpn *ACMEALPNSolver) PresentCached(ctx context.Context) error {
	return CachedSolver(ctx, alpn.config, alpn.domain, alpn)
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
	return controlclient.ACMEWaitForReady(ctx, hs.config, hs.domain, hs)
}

func (hs *ACMEHTTPSolver) PresentCached(ctx context.Context) error {
	return CachedSolver(ctx, hs.config, hs.domain, hs)
}
