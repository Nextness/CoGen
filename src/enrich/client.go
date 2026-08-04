// client.go provides a goroutine-pooled HTTP client with rate limiting
// and exponential backoff for fetching enrichment provider endpoints.
package enrich

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Client fetches URLs concurrently with configurable rate limiting and
// exponential backoff on 429 responses.
type Client struct {
	cfg    SourceConfig
	ticker *time.Ticker
	client *http.Client
}

var (
	fetchHeartbeatInterval = 5 * time.Second
	slowRequestLogAfter    = 5 * time.Second
)

// NewClient creates a Client for the given source config.
func NewClient(cfg SourceConfig) *Client {
	interval := time.Second
	if cfg.RatePerSecond > 0 {
		interval = time.Second / time.Duration(cfg.RatePerSecond)
	}

	return &Client{
		cfg:    cfg,
		ticker: time.NewTicker(interval),
		client: &http.Client{
			Timeout: time.Duration(cfg.TimeoutSecs) * time.Second,
		},
	}
}

// Close stops the rate-limit ticker.
func (c *Client) Close() {
	c.ticker.Stop()
}

// FetchResult carries the response body (or error) for one URL fetch.
type FetchResult struct {
	Body       []byte
	StatusCode int
	Err        error
}

// FetchAll fetches all URLs concurrently using a goroutine pool. It respects
// rate limiting across all workers and applies exponential backoff on 429.
// Returns a map of original URL -> FetchResult in the same order as input.
func (c *Client) FetchAll(ctx context.Context, urls []string) map[string]*FetchResult {
	if len(urls) == 0 {
		return nil
	}

	type task struct {
		url string
		idx int
	}

	type response struct {
		idx int
		res *FetchResult
	}

	tasks := make(chan task, len(urls))
	results := make(chan response, len(urls))
	started := time.Now()
	source := c.sourceName()

	// Start worker goroutines
	var wg sync.WaitGroup
	poolSize := c.cfg.Concurrency
	if poolSize < 1 {
		poolSize = 1
	}
	log.Info(
		"http batch started",
		"source", source,
		"total", len(urls),
		"concurrency", poolSize,
		"rate_per_second", c.cfg.RatePerSecond,
		"request_timeout", time.Duration(c.cfg.TimeoutSecs)*time.Second,
		"max_attempts", c.cfg.MaxRetries,
	)
	for i := 0; i < poolSize; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range tasks {
				// Rate limit: wait for ticker or context cancellation
				select {
				case <-c.ticker.C:
				case <-ctx.Done():
					results <- response{t.idx, &FetchResult{Err: ctx.Err()}}
					continue
				}

				fr := c.fetchOne(ctx, t.url)
				results <- response{t.idx, fr}
			}
		}()
	}

	// Send tasks
	for i, u := range urls {
		tasks <- task{u, i}
	}
	close(tasks)

	// Wait for all workers and close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect in order while periodically reporting that outstanding requests
	// are still active. Gather functions only inspect results after this method
	// returns, so progress must be emitted here to avoid a silent batch wait.
	out := make(map[string]*FetchResult, len(urls))
	heartbeat := time.NewTicker(fetchHeartbeatInterval)
	defer heartbeat.Stop()
	completed := 0
	succeeded := 0
	notFound := 0
	failed := 0
	progressEvery := (len(urls) + 9) / 10 // approximately every 10%
	if progressEvery < 1 {
		progressEvery = 1
	}

	for completed < len(urls) {
		select {
		case r, ok := <-results:
			if !ok {
				if completed < len(urls) {
					log.Warn(
						"http batch ended before every request completed",
						"source", source, "completed", completed, "total", len(urls),
					)
				}
				return out
			}
			out[urls[r.idx]] = r.res
			completed++
			switch {
			case r.res != nil && r.res.StatusCode == http.StatusOK && r.res.Err == nil:
				succeeded++
			case r.res != nil && r.res.StatusCode == http.StatusNotFound:
				notFound++
			default:
				failed++
			}
			if completed == 1 || completed == len(urls) || completed%progressEvery == 0 {
				logFetchProgress(
					"http fetch progress",
					source, started, completed, len(urls), succeeded, notFound, failed,
				)
			}
		case <-heartbeat.C:
			logFetchProgress(
				"http fetch heartbeat",
				source, started, completed, len(urls), succeeded, notFound, failed,
			)
		}
	}
	return out
}

// Fetch retrieves one URL through the same rate-limited path as FetchAll.
// Workspace cache policy uses it so individual cache misses do not bypass the
// provider's configured request limit.
func (c *Client) Fetch(ctx context.Context, url string) *FetchResult {
	return c.FetchAll(ctx, []string{url})[url]
}

// fetchOne performs a single HTTP GET with retries and exponential backoff.
func (c *Client) fetchOne(ctx context.Context, url string) *FetchResult {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return &FetchResult{Err: fmt.Errorf("create request: %w", err)}
	}
	contact := strings.TrimSpace(c.cfg.ContactEmail)
	userAgent := c.cfg.UserAgent
	req.Header.Set("From", contact)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	for attempt := 1; attempt <= c.cfg.MaxRetries; attempt++ {
		currentAttempt := attempt
		attemptStarted := time.Now()
		log.Debug(
			"http request started",
			"source", c.sourceName(),
			"attempt", currentAttempt,
			"max_attempts", c.cfg.MaxRetries,
			"url", truncateStr(url, 160),
		)
		slowTimer := time.AfterFunc(slowRequestLogAfter, func() {
			log.Info(
				"http request still in progress",
				"source", c.sourceName(),
				"attempt", currentAttempt,
				"elapsed", time.Since(attemptStarted).Round(time.Second),
				"timeout", time.Duration(c.cfg.TimeoutSecs)*time.Second,
				"url", truncateStr(url, 160),
			)
		})
		resp, err := c.client.Do(req)
		if err != nil {
			slowTimer.Stop()
			log.Warn(
				"http request failed",
				"source", c.sourceName(), "attempt", currentAttempt, "error", err,
			)
			return &FetchResult{Err: fmt.Errorf("request failed: %w", err)}
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		slowTimer.Stop()
		if err != nil {
			log.Warn(
				"http response read failed",
				"source", c.sourceName(), "attempt", currentAttempt, "error", err,
			)
			return &FetchResult{Err: fmt.Errorf("read body: %w", err)}
		}

		switch {
		case resp.StatusCode == 200:
			return &FetchResult{Body: body, StatusCode: 200}

		case resp.StatusCode == 404:
			return &FetchResult{StatusCode: 404}

		case resp.StatusCode == 429:
			// Exponential backoff: 2^attempt seconds
			wait := time.Duration(1<<attempt) * time.Second
			log.Warn(
				"http request rate limited; backing off",
				"source", c.sourceName(),
				"attempt", currentAttempt,
				"max_attempts", c.cfg.MaxRetries,
				"backoff", wait,
			)

			select {
			case <-time.After(wait):
				continue
			case <-ctx.Done():
				return &FetchResult{Err: ctx.Err()}
			}

		default:
			log.Warn(
				"http request returned unexpected status",
				"source", c.sourceName(), "attempt", currentAttempt, "status", resp.StatusCode,
			)
			return &FetchResult{
				Body:       body,
				StatusCode: resp.StatusCode,
				Err:        fmt.Errorf("unexpected status %d", resp.StatusCode),
			}
		}
	}

	err = fmt.Errorf("max retries (%d) exceeded", c.cfg.MaxRetries)
	log.Warn("http request exhausted retries", "source", c.sourceName(), "error", err)
	return &FetchResult{Err: err}
}

// truncateStr truncates str to the requested limit.
func truncateStr(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "..."
}

// sourceName returns the configured provider name used in logging and evidence.
func (c *Client) sourceName() string {
	if c.cfg.Name != "" {
		return c.cfg.Name
	}
	return "unconfigured"
}

// logFetchProgress emits structured provider progress and elapsed-time fields.
func logFetchProgress(message, source string, started time.Time, completed, total, succeeded, notFound, failed int) {
	log.Info(
		message,
		"source", source,
		"completed", completed,
		"total", total,
		"remaining", total-completed,
		"succeeded", succeeded,
		"not_found", notFound,
		"failed", failed,
		"elapsed", time.Since(started).Round(time.Second),
	)
}
