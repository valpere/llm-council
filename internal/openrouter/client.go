package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"syscall"
	"time"

	"github.com/valpere/llm-council/internal/council"
)

const (
	defaultURL    = "https://openrouter.ai/api/v1/chat/completions"
	maxBodyBytes  = 4 * 1024 * 1024  // 4 MiB cap on response bodies
	maxRetryAfter = 30 * time.Second // ceiling applied to Retry-After header values
)

// retryBaseDelay is the first attempt's nominal backoff. Exposed as a package
// variable so tests can shrink it; production uses 500 ms.
//
// maxCumulativeBackoffDuration caps the total time spent sleeping across all
// retry attempts in a single Complete call. Also a variable for testability.
var (
	retryBaseDelay               = 500 * time.Millisecond
	maxCumulativeBackoffDuration = 60 * time.Second
)

// APIError is returned when OpenRouter responds with a non-200 status code.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("openrouter: API error %d: %s", e.StatusCode, e.Body)
}

// Client sends completion requests to the OpenRouter API.
type Client struct {
	apiKey     string
	baseURL    string // overridable in tests; defaults to defaultURL
	http       *http.Client
	maxRetries int          // total retries (1 initial attempt + maxRetries retries)
	logger     *slog.Logger // never nil — NewClient substitutes slog.Default()
}

// NewClient creates a Client with the given API key, base URL, HTTP timeout,
// retry budget, and logger. baseURL overrides the default OpenRouter endpoint;
// pass "" to use the default. maxRetries of 0 means a single attempt (no
// retries). A nil logger falls back to slog.Default().
func NewClient(apiKey, baseURL string, timeout time.Duration, maxRetries int, logger *slog.Logger) *Client {
	if baseURL == "" {
		baseURL = defaultURL
	}
	if logger == nil {
		logger = slog.Default()
	}
	if maxRetries < 0 {
		maxRetries = 0
	}
	return &Client{
		apiKey:     apiKey,
		baseURL:    baseURL,
		http:       &http.Client{Timeout: timeout},
		maxRetries: maxRetries,
		logger:     logger,
	}
}

// Compile-time assertion: Client implements council.LLMClient.
var _ council.LLMClient = (*Client)(nil)

// Complete POSTs a chat completion request to OpenRouter and returns the response.
// On transient failures (5xx, 429, network blips), it retries with exponential
// backoff and ±25% jitter, honoring Retry-After headers and a cumulative
// 60 s sleep budget. Returns *APIError on non-200 responses after retries are
// exhausted.
func (c *Client) Complete(ctx context.Context, req council.CompletionRequest) (council.CompletionResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return council.CompletionResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	var (
		cumulativeBackoff time.Duration
		lastErr           error
	)
	for attempt := 0; ; attempt++ {
		// Honour a context cancellation observed before issuing the next request.
		if cerr := ctx.Err(); cerr != nil {
			return council.CompletionResponse{}, cerr
		}

		resp, attemptErr := c.doAttempt(ctx, body)
		shouldRetry, retryAfter, finalErr := c.classifyAttempt(resp, attemptErr)

		if !shouldRetry {
			// Either we have a successful response, an unmarshal error, or a
			// non-retryable failure. classifyAttempt returns the right value
			// in finalErr (and resp on success) for us to surface.
			if finalErr != nil {
				if attempt > 0 {
					c.logger.Info("openrouter: retries exhausted",
						"attempts", attempt+1, "final_error", finalErr)
				}
				return council.CompletionResponse{}, finalErr
			}
			// Successful 200 + decoded body.
			return c.decodeBody(resp)
		}

		// Retryable. lastErr always tracks the most recent reason for retry so
		// we can surface it if the cap or maxRetries forces a final return.
		lastErr = finalErr

		if attempt >= c.maxRetries {
			c.logger.Info("openrouter: retries exhausted",
				"attempts", attempt+1, "final_error", lastErr)
			return council.CompletionResponse{}, lastErr
		}

		delay := backoffDelay(attempt, retryAfter)
		if cumulativeBackoff+delay > maxCumulativeBackoffDuration {
			c.logger.Info("openrouter: cumulative backoff cap reached",
				"cumulative_ms", cumulativeBackoff.Milliseconds(),
				"final_error", lastErr)
			return council.CompletionResponse{}, lastErr
		}
		cumulativeBackoff += delay

		c.logger.Debug("openrouter: retrying",
			"attempt", attempt+1, "delay_ms", delay.Milliseconds(),
			"cause", lastErr)

		select {
		case <-ctx.Done():
			return council.CompletionResponse{}, ctx.Err()
		case <-time.After(delay):
		}
	}
}

// doAttempt performs a single HTTP request. On HTTP-level responses, it leaves
// resp.Body open so the caller can decide whether to drain (retry path) or
// read (non-retry path). On network errors it returns err only.
func (c *Client) doAttempt(ctx context.Context, body []byte) (*http.Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("HTTP-Referer", "https://github.com/valpere/llm-council")
	httpReq.Header.Set("X-Title", "LLM Council")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	return resp, nil
}

// classifyAttempt inspects the result of doAttempt and decides whether to
// retry. On a retry decision, it drains and closes the response body so the
// connection returns to the keep-alive pool. On a non-retry decision, it
// either returns a final error (with the body read for a non-retryable status)
// or leaves resp populated with the body still readable (for the success path,
// where the caller will decode).
func (c *Client) classifyAttempt(resp *http.Response, err error) (shouldRetry bool, retryAfter time.Duration, finalErr error) {
	// Network-level error path.
	if err != nil {
		if isRetriableNetErr(err) {
			return true, 0, err
		}
		return false, 0, err
	}

	// HTTP-level path.
	if isRetriableStatus(resp.StatusCode) {
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		// Read the body up to maxBodyBytes so it can be surfaced if retries
		// are exhausted (or maxRetries=0). Then drain + close any tail so the
		// connection returns to the keep-alive pool — for typical OpenRouter
		// error JSON the tail will be empty.
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		return true, retryAfter, &APIError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	// Non-retryable status (4xx other than 429, or 200). Read body so we can
	// either return the *APIError with body, or hand off to decodeBody on 200.
	respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	_ = resp.Body.Close()
	if readErr != nil {
		// A failed body read is itself a candidate for retry (ErrUnexpectedEOF, EOF).
		if isRetriableNetErr(readErr) {
			return true, 0, fmt.Errorf("read response: %w", readErr)
		}
		return false, 0, fmt.Errorf("read response: %w", readErr)
	}

	if resp.StatusCode != http.StatusOK {
		return false, 0, &APIError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	// Success — stash the body on resp so decodeBody can read it without re-reading.
	resp.Body = io.NopCloser(bytes.NewReader(respBody))
	return false, 0, nil
}

// decodeBody parses a successful response body into a CompletionResponse.
func (c *Client) decodeBody(resp *http.Response) (council.CompletionResponse, error) {
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return council.CompletionResponse{}, fmt.Errorf("read response: %w", err)
	}
	var completionResp council.CompletionResponse
	if err := json.Unmarshal(respBody, &completionResp); err != nil {
		return council.CompletionResponse{}, fmt.Errorf("unmarshal response: %w", err)
	}
	return completionResp, nil
}

// isRetriableStatus returns true for HTTP status codes worth retrying:
// 429 (rate limit), 502, 503, 504 (transient upstream errors).
func isRetriableStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	}
	return false
}

// isRetriableNetErr returns true for network-level errors worth retrying:
// timeouts (including http.Client.Timeout), connection resets, broken pipes,
// EOFs from closed keep-alive connections.
//
// User-context cancellation is NOT classified here — Complete checks ctx.Err()
// at the top of each iteration before retrying, so any user-cancelled context
// short-circuits before this function would observe it. We deliberately treat
// net.Error.Timeout() as retriable even when its underlying cause unwraps to
// context.DeadlineExceeded, because http.Client.Timeout fires that exact shape
// and we want it to retry.
func isRetriableNetErr(err error) bool {
	if err == nil {
		return false
	}
	// Explicit user cancellation is never retried — no point continuing if the
	// caller has given up. (DeadlineExceeded is intentionally not rejected here:
	// see the doc comment above.)
	if errors.Is(err, context.Canceled) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) {
		return true
	}
	return false
}

// parseRetryAfter parses an HTTP Retry-After header. Supports both
// delta-seconds (integer) and HTTP-date forms. Returns 0 on parse failure or
// out-of-range values; caller falls back to the schedule. Caps at maxRetryAfter
// (30 s) to prevent the gateway from forcing arbitrarily long client waits.
func parseRetryAfter(h string) time.Duration {
	if h == "" {
		return 0
	}
	if secs, err := strconv.Atoi(h); err == nil {
		if secs <= 0 {
			return 0
		}
		d := time.Duration(secs) * time.Second
		if d > maxRetryAfter {
			return maxRetryAfter
		}
		return d
	}
	if t, err := http.ParseTime(h); err == nil {
		d := time.Until(t)
		if d <= 0 {
			return 0
		}
		if d > maxRetryAfter {
			return maxRetryAfter
		}
		return d
	}
	return 0
}

// backoffDelay returns the next sleep duration. If retryAfter > 0 (already
// capped), it's used directly. Otherwise: retryBaseDelay * 3^attempt with
// ±25% jitter via math/rand/v2 (Go 1.22+ — no global lock).
func backoffDelay(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		return retryAfter
	}
	base := retryBaseDelay
	for range attempt {
		base *= 3
	}
	if base <= 0 {
		return retryBaseDelay
	}
	// Jitter range: [-base/4, +base/4].
	jitter := time.Duration(rand.Int64N(int64(base)/2)) - base/4
	return base + jitter
}
