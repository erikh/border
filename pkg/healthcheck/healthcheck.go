package healthcheck

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/erikh/ping"
)

const (
	TypePing = "ping"
	TypeHTTP = "http"
	// FIXME add tcp type
)

type HealthCheck struct {
	Name     string        `record:"name,optional"`
	Type     string        `record:"type,optional"`
	Timeout  time.Duration `record:"timeout"`
	Failures int           `record:"failures"`

	// for HTTP, this is *http.Request in, *http.Request out.
	// for Ping, this is not used.
	requestTransformer func(interface{}) interface{}
	target             string
	failed             bool
}

// Copies the health check without duplicating the target.
func (hc *HealthCheck) Copy() *HealthCheck {
	return &HealthCheck{
		Name:     hc.Name,
		Type:     hc.Type,
		Timeout:  hc.Timeout,
		Failures: hc.Failures,
	}
}

func (hc *HealthCheck) SetTarget(target string) {
	hc.target = target
}

func (hc *HealthCheck) Target() string {
	return hc.target
}

func (hc *HealthCheck) SetRequestTransformer(transformer func(interface{}) interface{}) {
	hc.requestTransformer = transformer
}

type HealthCheckAction struct {
	Check        *HealthCheck
	FailedAction func(*HealthCheck) error
	ReviveAction func(*HealthCheck) error
}

type HealthChecker struct {
	HealthChecks []*HealthCheckAction
	Failures     []int

	timer      *time.Ticker
	mutex      sync.RWMutex
	cancelFunc context.CancelFunc
}

func (hc *HealthCheckAction) runCheck() error {
	switch hc.Check.Type {
	case TypePing:
		ip := net.ParseIP(hc.Check.target)
		if ip == nil {
			return fmt.Errorf("Ping types must be an IP, received %q for check %q, which is not an IP", hc.Check.target, hc.Check.Name)
		}

		if !ping.Ping(&net.IPAddr{IP: ip}, hc.Check.Timeout) {
			return fmt.Errorf("Failed to ping address %q (check: %q)", hc.Check.target, hc.Check.Name)
		}
	case TypeHTTP:
		ctx, cancel := context.WithTimeout(context.Background(), hc.Check.Timeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, hc.Check.target, nil)
		if err != nil {
			return fmt.Errorf("Could not construct initial request: %w", err)
		}

		if hc.Check.requestTransformer != nil {
			req = hc.Check.requestTransformer(req).(*http.Request)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("Failed to contact %q over HTTP (check: %q)", hc.Check.target, hc.Check.Name)
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("Status was not OK (was: %d) on HTTP healthcheck %q for target %q", resp.StatusCode, hc.Check.Name, hc.Check.target)
		}
	default:
		return fmt.Errorf("Invalid health check type %q (check: %q): please adjust your configuration", hc.Check.Type, hc.Check.Name)
	}

	return nil
}

func Init(checks []*HealthCheckAction, minDuration time.Duration) *HealthChecker {
	failures := make([]int, len(checks))

	return &HealthChecker{
		HealthChecks: checks,
		Failures:     failures,
		timer:        time.NewTicker(minDuration),
	}
}

func (hcr *HealthChecker) runChecks() {
	finished := &sync.WaitGroup{}
	finished.Add(len(hcr.HealthChecks))

	hcr.mutex.RLock()
	for i, check := range hcr.HealthChecks {
		go func(check *HealthCheckAction, i int) {
			if err := check.runCheck(); err != nil {
				log.Print(err)
				hcr.mutex.Lock()
				hcr.Failures[i]++
				if hcr.Failures[i] >= hcr.HealthChecks[i].Check.Failures && !hcr.HealthChecks[i].Check.failed {
					if err := hcr.HealthChecks[i].FailedAction(hcr.HealthChecks[i].Check); err != nil {
						log.Printf("Triggered action on failed health check for %q also failed: %v", hcr.HealthChecks[i].Check.Name, err)
					}

					hcr.HealthChecks[i].Check.failed = true
				}
				hcr.mutex.Unlock()
			} else {
				hcr.mutex.RLock()
				if hcr.Failures[i] > 0 {
					log.Printf("%q revived on target %q", check.Check.Name, check.Check.Target())

					if err := check.ReviveAction(check.Check); err != nil {
						log.Printf("Error while reviving record %q (name: %q): %v", check.Check.Target(), check.Check.Name, err)
					}

				}
				hcr.mutex.RUnlock()

				hcr.mutex.Lock()
				// should have some back-off code here to detect flapping things
				hcr.Failures[i] = 0
				hcr.HealthChecks[i].Check.failed = false
				hcr.mutex.Unlock()
			}
			finished.Done()
		}(check, i)
	}
	hcr.mutex.RUnlock()

	finished.Wait()
}

func (hcr *HealthChecker) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	hcr.cancelFunc = cancel

	go hcr.run(ctx)
}

func (hcr *HealthChecker) Shutdown() {
	hcr.cancelFunc()
}

func (hcr *HealthChecker) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			hcr.timer.Stop()
			return
		case <-hcr.timer.C:
		}

		hcr.runChecks()
	}
}
