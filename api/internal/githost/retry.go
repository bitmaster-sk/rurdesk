package githost

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// doWithRetry executes an HTTP request with up to 3 attempts and exponential backoff (1s, 3s, 9s).
// On 429: reads Retry-After header, sleeps that duration, counts as a retry.
// On 5xx: retries. On 4xx (except 429): returns immediately.
func doWithRetry(ctx context.Context, client *http.Client, req *http.Request) (*http.Response, error) {
	backoff := []time.Duration{1 * time.Second, 3 * time.Second, 9 * time.Second}
	var lastErr error

	for attempt := 0; attempt < 3; attempt++ {
		// Clone request for retry (body was already drained on previous attempt if needed).
		cloned := req.Clone(ctx)

		resp, err := client.Do(cloned)
		if err != nil {
			lastErr = fmt.Errorf("attempt %d: %w", attempt+1, err)
			if attempt < 2 {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(backoff[attempt]):
				}
			}
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			delay := backoff[attempt]
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if secs, parseErr := strconv.Atoi(ra); parseErr == nil {
					delay = time.Duration(secs) * time.Second
				}
			}
			lastErr = fmt.Errorf("attempt %d: 429 too many requests", attempt+1)
			if attempt < 2 {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(delay):
				}
			}
			continue
		}

		if resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("attempt %d: server error %d", attempt+1, resp.StatusCode)
			if attempt < 2 {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(backoff[attempt]):
				}
			}
			continue
		}

		// 4xx (except 429): no retry
		return resp, nil
	}

	return nil, fmt.Errorf("all retries exhausted: %w", lastErr)
}
