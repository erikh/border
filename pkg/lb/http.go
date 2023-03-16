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

func (b *Balancer) BalanceHTTP(ctx context.Context, notifyFunc func(error)) {
	httpCtx, cancel := context.WithCancel(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/", b.serveHTTP(httpCtx))

	server := &http.Server{
		Handler: mux,
	}

	errChan := make(chan error, 1)

	go func() {
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

func (b *Balancer) serveHTTP(ctx context.Context) func(http.ResponseWriter, *http.Request) {
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

		for {
			select {
			case <-ctx.Done():
			default:
			retry:
				lowestAddr := b.getLowestBalancer()
				if lowestAddr != "" {
					b.mutex.Lock()
					b.backendConns[lowestAddr]++

					url := r.URL
					url.Host = lowestAddr

					req, err := http.NewRequestWithContext(ctx, r.Method, url.String(), r.Body)
					if err != nil {
						http.Error(w, fmt.Sprintf("Proxy error: %v", err), http.StatusInternalServerError)
						b.mutex.Unlock()
						return
					}

					req.Header = headers

					b.mutex.Unlock()

					// FIXME need to use http.Transport properly to do connection tracking
					// properly. net/http will get in the way.
					defer b.decrementCount(ctx, lowestAddr)
					resp, err := http.DefaultClient.Do(req)
					if err != nil {
						http.Error(w, fmt.Sprintf("Proxy error: %v", err), http.StatusInternalServerError)
						return
					}

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
}
