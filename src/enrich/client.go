// client.go provides a goroutine-pooled HTTP client with rate limiting
// and exponential backoff for fetching enrichment provider endpoints.
package enrich

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	// MaxProviderPayloadBytes bounds one provider response retained as inline evidence.
	MaxProviderPayloadBytes   = 10 << 20
	maxProviderRatePerSecond  = int(time.Second)
	maxProviderConcurrency    = 64
	maxProviderTimeoutSeconds = 300
	maxProviderRetries        = 10
	maxProviderBatchSize      = 1000
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

// NewClient creates a Client for the given validated source config.
func NewClient(cfg SourceConfig) (*Client, error) {
	if err := ValidateSourceConfig(cfg); err != nil {
		return nil, err
	}
	interval := time.Second / time.Duration(cfg.RatePerSecond)

	return &Client{
		cfg:    cfg,
		ticker: time.NewTicker(interval),
		client: &http.Client{
			Timeout: time.Duration(cfg.TimeoutSecs) * time.Second,
		},
	}, nil
}

// ValidateSourceConfig verifies provider endpoints and request bounds before enrichment begins.
func ValidateSourceConfig(cfg SourceConfig) error {
	if strings.TrimSpace(cfg.Name) == "" {
		return fmt.Errorf("provider name is required")
	}
	if err := validProviderURL("base URL", cfg.BaseURL); err != nil {
		return err
	}
	for name, value := range cfg.ExtraURLs {
		if err := validProviderURL("extra URL "+name, value); err != nil {
			return err
		}
	}
	if cfg.RatePerSecond < 1 || cfg.RatePerSecond > maxProviderRatePerSecond {
		return fmt.Errorf("provider rate_per_second must be between 1 and %d", maxProviderRatePerSecond)
	}
	if cfg.Concurrency < 1 || cfg.Concurrency > maxProviderConcurrency {
		return fmt.Errorf("provider concurrency must be between 1 and %d", maxProviderConcurrency)
	}
	if cfg.TimeoutSecs < 1 || cfg.TimeoutSecs > maxProviderTimeoutSeconds {
		return fmt.Errorf("provider timeout_seconds must be between 1 and %d", maxProviderTimeoutSeconds)
	}
	if cfg.MaxRetries < 1 || cfg.MaxRetries > maxProviderRetries {
		return fmt.Errorf("provider max_retries must be between 1 and %d", maxProviderRetries)
	}
	if cfg.BatchSize < 1 || cfg.BatchSize > maxProviderBatchSize {
		return fmt.Errorf("provider batch_size must be between 1 and %d", maxProviderBatchSize)
	}
	return nil
}

// validProviderURL verifies an HTTP(S) provider endpoint has a host.
func validProviderURL(label, value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("provider %s must be an absolute HTTP(S) URL", label)
	}
	return nil
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
	log.Info(
		"http batch started",
		"source", source,
		"total", len(urls),
		"concurrency", poolSize,
		"rate_per_second", c.cfg.RatePerSecond,
		"request_timeout", time.Duration(c.cfg.TimeoutSecs)*time.Second,
		"max_attempts", c.cfg.MaxRetries+1,
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
	contact := strings.TrimSpace(c.cfg.ContactEmail)
	host, path := requestLogTarget(url)
	maxAttempts := c.cfg.MaxRetries + 1

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		currentAttempt := attempt
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return &FetchResult{Err: fmt.Errorf("create request: %w", err)}
		}
		req.Header.Set("From", contact)
		req.Header.Set("User-Agent", c.cfg.UserAgent)
		req.Header.Set("Accept", "application/json")
		attemptStarted := time.Now()
		log.Debug(
			"http request started",
			"source", c.sourceName(),
			"attempt", currentAttempt,
			"max_attempts", maxAttempts,
			"host", host,
			"path", path,
		)
		slowTimer := time.AfterFunc(slowRequestLogAfter, func() {
			log.Info(
				"http request still in progress",
				"source", c.sourceName(),
				"attempt", currentAttempt,
				"elapsed", time.Since(attemptStarted).Round(time.Second),
				"timeout", time.Duration(c.cfg.TimeoutSecs)*time.Second,
				"host", host,
				"path", path,
			)
		})
		resp, err := c.client.Do(req)
		if err != nil {
			slowTimer.Stop()
			log.Warn(
				"http request failed",
				"source", c.sourceName(), "attempt", currentAttempt, "host", host, "path", path,
			)
			return &FetchResult{Err: fmt.Errorf("request failed: %w", err)}
		}

		var body []byte
		if resp.StatusCode == http.StatusTooManyRequests {
			err = discardProviderBody(resp.Body)
		} else {
			body, err = readProviderBody(resp.Body)
		}
		closeErr := resp.Body.Close()
		slowTimer.Stop()
		if err == nil {
			err = closeErr
		}
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
				"max_attempts", maxAttempts,
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

	err := fmt.Errorf("max retries (%d) exceeded", c.cfg.MaxRetries)
	log.Warn("http request exhausted retries", "source", c.sourceName(), "error", err)
	return &FetchResult{Err: err}
}

// readProviderBody reads one retained provider payload and rejects oversized responses.
func readProviderBody(body io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, MaxProviderPayloadBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxProviderPayloadBytes {
		return nil, fmt.Errorf("provider response exceeds %d bytes", MaxProviderPayloadBytes)
	}
	return data, nil
}

// discardProviderBody consumes a bounded non-retained response body before the connection is reused.
func discardProviderBody(body io.Reader) error {
	read, err := io.Copy(io.Discard, io.LimitReader(body, MaxProviderPayloadBytes+1))
	if err != nil {
		return err
	}
	if read > MaxProviderPayloadBytes {
		return fmt.Errorf("provider response exceeds %d bytes", MaxProviderPayloadBytes)
	}
	return nil
}

// requestLogTarget returns query-free request fields suitable for operational logs.
func requestLogTarget(value string) (string, string) {
	parsed, err := url.Parse(value)
	if err != nil {
		return "invalid", ""
	}
	return parsed.Host, parsed.EscapedPath()
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
