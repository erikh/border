package healthcheck

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/erikh/ping"
)

const (
	TypePing = "ping"
	// FIXME add tcp and http types
)

type HealthCheck struct {
	Name     string        `record:"name,optional"`
	Type     string        `record:"type,optional"`
	Timeout  time.Duration `record:"timeout"`
	Failures int           `record:"failures"`

	target string
}

func (hc *HealthCheck) SetTarget(target string) {
	hc.target = target
}

type HealthCheckAction struct {
	Check  *HealthCheck
	Action func() error
}

type HealthChecker struct {
	HealthChecks []HealthCheckAction
	Failures     []int

	timer *time.Ticker
	mutex sync.RWMutex
}

func (hc *HealthCheckAction) runCheck() error {
	switch hc.Check.Type {
	case TypePing:
		ip := net.ParseIP(hc.Check.target)
		if ip == nil {
			return fmt.Errorf("Ping types must be an IP, received %q, which is not an IP", hc.Check.target)
		}

		if !ping.Ping(&net.IPAddr{IP: ip}, hc.Check.Timeout) {
			return fmt.Errorf("Failed to ping address %q", hc.Check.target)
		}
	default:
		return fmt.Errorf("Invalid health check type %q: please adjust your configuration", hc.Check.Type)
	}

	return nil
}

func Init(checks []HealthCheckAction, minDuration time.Duration) *HealthChecker {
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
		go func(i int) {
			if err := check.runCheck(); err != nil {
				log.Print(err)
				hcr.mutex.Lock()
				hcr.Failures[i]++
				hcr.mutex.Unlock()
			} else {
				// should have some back-off code here to detect flapping things
				hcr.Failures[i] = 0
			}
			finished.Done()
		}(i)
	}
	hcr.mutex.RUnlock()

	finished.Wait()
}

func (hcr *HealthChecker) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			hcr.timer.Stop()
			return
		case <-hcr.timer.C:
		}

		hcr.runChecks()

		hcr.mutex.RLock()
		for i, failures := range hcr.Failures {
			if failures >= hcr.HealthChecks[i].Check.Failures {
				if err := hcr.HealthChecks[i].Action(); err != nil {
					log.Printf("Triggered action on failed health check for %q also failed: %v", hcr.HealthChecks[i].Check.Name, err)
				}
			}
		}
		hcr.mutex.RUnlock()
	}
}
