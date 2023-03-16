package lb

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

func (b *Balancer) dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	b.mutex.Lock()
	b.backendConns[addr]++
	b.mutex.Unlock()

	go b.decrementCount(ctx, addr)

	return (&net.Dialer{}).DialContext(ctx, network, addr)
}

func (b *Balancer) BalanceHTTP(ctx context.Context, notifyFunc func(error)) {
	httpCtx, cancel := context.WithCancel(ctx)

	client := &http.Client{
		Transport: &http.Transport{
			DialContext:         b.dialContext,
			MaxIdleConnsPerHost: b.maxConns,
			IdleConnTimeout:     b.timeout, // XXX should we still manually time out the connection with our TCP functions?
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", b.serveHTTP(httpCtx, client))

	server := &http.Server{
		Handler: mux,
	}

	errChan := make(chan error, 1)

	go func() {
		defer client.Transport.(*http.Transport).CloseIdleConnections()
		defer cancel()
		if err := server.Serve(b.listener); err != nil {
			errChan <- err
		}

	}()

	select {
	case <-time.After(100 * time.Millisecond): // XXX make this adjustable?
		notifyFunc(nil)
	case err := <-errChan:
		notifyFunc(err)
	}
}

func (b *Balancer) serveHTTP(ctx context.Context, client *http.Client) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		headers := http.Header{}

		for header, values := range r.Header {
			switch http.CanonicalHeaderKey(header) {
			case http.CanonicalHeaderKey("x-forwarded-for"):
				if len(values) > 1 {
					log.Println("Multiple X-Forwarded-For headers; failing this connection")
					http.Error(w, "Multiple X-Forwarded-For headers; failing this connection", http.StatusInternalServerError)
					return
				}

				if len(values) == 0 {
					values = []string{b.listenerIP}
				} else {
					ips := strings.Split(values[0], ",")
					ips = append(ips, b.listenerIP)
					values[0] = strings.Join(ips, ",")
				}
			}

			headers[header] = values
		}

		// FIXME X-Real-IP support?

		var (
			connCtx context.Context
			cancel  context.CancelFunc
		)

		if b.timeout != 0 {
			connCtx, cancel = context.WithTimeout(ctx, b.timeout)
		} else {
			connCtx, cancel = context.WithCancel(ctx)
		}

		defer cancel()

		select {
		case <-ctx.Done(): // balancer context, not the conn
		default:
		retry:
			if lowestAddr := b.getLowestBalancer(); lowestAddr != "" {
				url := r.URL
				url.Scheme = "http"
				url.Host = lowestAddr

				req, err := http.NewRequestWithContext(connCtx, r.Method, url.String(), r.Body)
				if err != nil {
					http.Error(w, fmt.Sprintf("Proxy error: %v", err), http.StatusInternalServerError)
					return
				}

				req.Header = headers

				resp, err := client.Do(req)
				if err != nil {
					http.Error(w, fmt.Sprintf("Proxy error: %v", err), http.StatusInternalServerError)
					return
				}

				r.Body.Close()
				defer resp.Body.Close()

				if _, err := io.Copy(w, resp.Body); err != nil && !errors.Is(err, net.ErrClosed) {
					http.Error(w, fmt.Sprintf("Early close in body copy: %v", err), http.StatusInternalServerError)
					return
				}

				w.WriteHeader(resp.StatusCode)
			} else {
				goto retry
			}
		}
	}
}
