package healthcheck

import (
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/erikh/border/integration-tests"
	hc "github.com/erikh/border/pkg/healthcheck"
	"github.com/vishvananda/netlink"
)

func TestHealthCheck(t *testing.T) {
	if !integration.RequireDocker(t) {
		return
	}

	var successful atomic.Bool
	successful.Store(true)

	check := &hc.HealthCheck{
		Name:     "ping test",
		Type:     hc.TypePing,
		Timeout:  100 * time.Millisecond,
		Failures: 3,
	}
	check.SetTarget("11.1.0.1")

	hcr := hc.Init([]hc.HealthCheckAction{
		{
			Check: check,
			Action: func(*hc.HealthCheck) error {
				successful.Store(false)
				return nil
			},
		},
	}, 100*time.Millisecond)

	// create a new interface with an IP address. this should only happen in docker!
	attrs := netlink.NewLinkAttrs()
	attrs.Name = "ping-frontend"
	veth := &netlink.Veth{LinkAttrs: attrs, PeerName: "ping-backend"}
	if err := netlink.LinkAdd(veth); err != nil {
		t.Fatalf("While creating test link: %v", err)
	}

	if err := netlink.AddrAdd(veth, &netlink.Addr{IPNet: netlink.NewIPNet(net.ParseIP("11.1.0.1"))}); err != nil {
		t.Fatalf("While adding address to test link: %v", err)
	}

	if err := netlink.LinkSetUp(veth); err != nil {
		t.Fatalf("While raising test link: %v", err)
	}

	pingCleanup := func() { netlink.LinkDel(veth) }
	t.Cleanup(pingCleanup)
	t.Cleanup(hcr.Shutdown)

	go hcr.Start()

	time.Sleep(time.Second)

	if !successful.Load() {
		t.Fatal("Health check did not succeed")
	}

	pingCleanup()
	time.Sleep(time.Second)

	if successful.Load() {
		t.Fatal("Health check did not fail")
	}
}
