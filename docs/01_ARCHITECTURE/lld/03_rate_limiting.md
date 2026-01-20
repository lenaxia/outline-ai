# Low-Level Design: Rate Limiting

**Domain:** Rate Limiting Infrastructure
**Status:** Design
**Last Updated:** 2026-01-19
**Target Deployment:** Homelab/SOHO

## Purpose

Respect API rate limits for both Outline API and AI services to prevent throttling and ensure reliable operation.

## Design Principles

1. **Respect Upstream Limits**: Never exceed provider rate limits
2. **Fair Queuing**: FIFO request processing
3. **Non-Blocking**: Use blocking waits when necessary
4. **Per-Service Limits**: Separate limiters for each API
5. **Simple Implementation**: Token bucket algorithm

## Rate Limiting Algorithm

### Token Bucket

**How it works:**
1. Bucket holds tokens (capacity = burst size)
2. Tokens refill at fixed rate
3. Each request consumes one token
4. If no tokens available, wait for refill

**Benefits:**
- Allows bursts up to capacity
- Smooths out traffic over time
- Simple to implement and understand
- Provided by Go standard library extension

## Implementation

### Rate Limiter Interface

```go
package ratelimit

import (
    "context"
    "time"

    "golang.org/x/time/rate"
)

type Limiter interface {
    Wait(ctx context.Context) error
    Allow() bool
    Reserve() *rate.Reservation
}

type TokenBucketLimiter struct {
    limiter *rate.Limiter
    limit   rate.Limit
    burst   int
}

func NewTokenBucketLimiter(requestsPerMinute int) *TokenBucketLimiter {
    r := rate.Limit(float64(requestsPerMinute) / 60.0)
    burst := max(1, requestsPerMinute/10)

    return &TokenBucketLimiter{
        limiter: rate.NewLimiter(r, burst),
        limit:   r,
        burst:   burst,
    }
}

func (l *TokenBucketLimiter) Wait(ctx context.Context) error {
    return l.limiter.Wait(ctx)
}

func (l *TokenBucketLimiter) Allow() bool {
    return l.limiter.Allow()
}

func (l *TokenBucketLimiter) Reserve() *rate.Reservation {
    return l.limiter.Reserve()
}

func max(a, b int) int {
    if a > b {
        return a
    }
    return b
}
```

### Per-Service Rate Limiters

```go
package ratelimit

import "context"

type ServiceLimiters struct {
    outlineLimiter Limiter
    aiLimiter      Limiter
}

func NewServiceLimiters(outlineRPM, aiRPM int) *ServiceLimiters {
    return &ServiceLimiters{
        outlineLimiter: NewTokenBucketLimiter(outlineRPM),
        aiLimiter:      NewTokenBucketLimiter(aiRPM),
    }
}

func (s *ServiceLimiters) WaitForOutline(ctx context.Context) error {
    return s.outlineLimiter.Wait(ctx)
}

func (s *ServiceLimiters) WaitForAI(ctx context.Context) error {
    return s.aiLimiter.Wait(ctx)
}

func (s *ServiceLimiters) CanCallOutline() bool {
    return s.outlineLimiter.Allow()
}

func (s *ServiceLimiters) CanCallAI() bool {
    return s.aiLimiter.Allow()
}
```

## Integration with HTTP Clients

### Outline API Client Integration

```go
package outline

import (
    "context"
    "net/http"

    "github.com/yourusername/outline-ai/internal/ratelimit"
)

type Client struct {
    httpClient *http.Client
    limiter    ratelimit.Limiter
    baseURL    string
    apiKey     string
}

func (c *Client) doRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
    // Wait for rate limiter before making request
    if err := c.limiter.Wait(ctx); err != nil {
        return nil, fmt.Errorf("rate limiter wait failed: %w", err)
    }

    // Add auth header
    req.Header.Set("Authorization", "Bearer "+c.apiKey)

    // Make request
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }

    // Check for rate limit response
    if resp.StatusCode == http.StatusTooManyRequests {
        // Log warning - we got rate limited despite limiter
        log.Warn().Msg("Rate limited by Outline API - may need to lower limit")

        // Check Retry-After header
        if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
            // Wait and retry
            duration, _ := time.ParseDuration(retryAfter + "s")
            time.Sleep(duration)
        }
    }

    return resp, nil
}
```

### AI Client Integration

```go
package ai

import (
    "context"

    "github.com/sashabaranov/go-openai"
    "github.com/yourusername/outline-ai/internal/ratelimit"
)

type Client struct {
    openaiClient *openai.Client
    limiter      ratelimit.Limiter
}

func (c *Client) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
    // Wait for rate limiter
    if err := c.limiter.Wait(ctx); err != nil {
        return openai.ChatCompletionResponse{}, fmt.Errorf("rate limiter wait failed: %w", err)
    }

    // Make AI request
    return c.openaiClient.CreateChatCompletion(ctx, req)
}
```

## Configuration

### Rate Limit Settings

```yaml
outline:
  rate_limit_per_minute: 60  # Conservative for free tier

ai:
  rate_limit_per_minute: 60  # Depends on AI provider tier
```

### Recommended Limits

| Service | Free Tier | Paid Tier | Recommended (SOHO) |
|---------|-----------|-----------|-------------------|
| Outline | 60/min | 120/min | 60/min |
| OpenAI | 60/min | 3500/min | 60/min |
| Claude | 50/min | 1000/min | 50/min |
| Local LLM | Unlimited | Unlimited | 60/min |

## Adaptive Rate Limiting

### 429 Response Handling

```go
package ratelimit

import (
    "net/http"
    "strconv"
    "time"
)

type AdaptiveLimiter struct {
    baseLimiter *TokenBucketLimiter
    backoffUntil time.Time
}

func NewAdaptiveLimiter(requestsPerMinute int) *AdaptiveLimiter {
    return &AdaptiveLimiter{
        baseLimiter: NewTokenBucketLimiter(requestsPerMinute),
    }
}

func (l *AdaptiveLimiter) Wait(ctx context.Context) error {
    // Check if we're in backoff period
    if time.Now().Before(l.backoffUntil) {
        wait := time.Until(l.backoffUntil)
        select {
        case <-time.After(wait):
        case <-ctx.Done():
            return ctx.Err()
        }
    }

    return l.baseLimiter.Wait(ctx)
}

func (l *AdaptiveLimiter) HandleRateLimitResponse(resp *http.Response) {
    if resp.StatusCode != http.StatusTooManyRequests {
        return
    }

    // Parse Retry-After header
    retryAfter := resp.Header.Get("Retry-After")
    if retryAfter == "" {
        // Default backoff: 60 seconds
        l.backoffUntil = time.Now().Add(60 * time.Second)
        return
    }

    // Try parsing as seconds
    if seconds, err := strconv.Atoi(retryAfter); err == nil {
        l.backoffUntil = time.Now().Add(time.Duration(seconds) * time.Second)
        return
    }

    // Try parsing as HTTP date
    if t, err := http.ParseTime(retryAfter); err == nil {
        l.backoffUntil = t
    }
}
```

## Metrics and Monitoring

### Rate Limiter Metrics

```go
package ratelimit

import (
    "sync/atomic"
    "time"
)

type Metrics struct {
    totalWaits      atomic.Int64
    totalAllowed    atomic.Int64
    totalRejected   atomic.Int64
    totalWaitTimeMs atomic.Int64
}

type MeteredLimiter struct {
    limiter Limiter
    metrics *Metrics
}

func NewMeteredLimiter(limiter Limiter) *MeteredLimiter {
    return &MeteredLimiter{
        limiter: limiter,
        metrics: &Metrics{},
    }
}

func (m *MeteredLimiter) Wait(ctx context.Context) error {
    start := time.Now()
    err := m.limiter.Wait(ctx)
    duration := time.Since(start)

    m.metrics.totalWaits.Add(1)
    m.metrics.totalWaitTimeMs.Add(duration.Milliseconds())

    return err
}

func (m *MeteredLimiter) Allow() bool {
    allowed := m.limiter.Allow()
    if allowed {
        m.metrics.totalAllowed.Add(1)
    } else {
        m.metrics.totalRejected.Add(1)
    }
    return allowed
}

func (m *MeteredLimiter) GetMetrics() map[string]int64 {
    return map[string]int64{
        "total_waits":       m.metrics.totalWaits.Load(),
        "total_allowed":     m.metrics.totalAllowed.Load(),
        "total_rejected":    m.metrics.totalRejected.Load(),
        "avg_wait_time_ms":  m.metrics.totalWaitTimeMs.Load() / max(m.metrics.totalWaits.Load(), 1),
    }
}
```

## Testing Strategy

### Unit Tests

```go
func TestTokenBucketLimiter_Allow(t *testing.T)
func TestTokenBucketLimiter_Wait(t *testing.T)
func TestTokenBucketLimiter_Burst(t *testing.T)
func TestAdaptiveLimiter_Backoff(t *testing.T)
func TestAdaptiveLimiter_RetryAfter(t *testing.T)
func TestMeteredLimiter_Metrics(t *testing.T)
```

### Integration Tests

```go
func TestRateLimiting_ConcurrentRequests(t *testing.T)
func TestRateLimiting_BurstHandling(t *testing.T)
func TestRateLimiting_ContextCancellation(t *testing.T)
```

### Test Example

```go
func TestTokenBucketLimiter_RateEnforcement(t *testing.T) {
    limiter := NewTokenBucketLimiter(60) // 60 req/min = 1 req/sec

    // Should allow first request immediately
    allowed := limiter.Allow()
    if !allowed {
        t.Error("Expected first request to be allowed")
    }

    // Should block second immediate request
    allowed = limiter.Allow()
    if allowed {
        t.Error("Expected second immediate request to be blocked")
    }

    // Wait 1 second and try again
    time.Sleep(1 * time.Second)
    allowed = limiter.Allow()
    if !allowed {
        t.Error("Expected request after 1 second to be allowed")
    }
}
```

## Error Handling

### Context Cancellation

```go
func (l *TokenBucketLimiter) Wait(ctx context.Context) error {
    if err := l.limiter.Wait(ctx); err != nil {
        if errors.Is(err, context.Canceled) {
            return fmt.Errorf("rate limiter wait canceled: %w", err)
        }
        if errors.Is(err, context.DeadlineExceeded) {
            return fmt.Errorf("rate limiter wait timeout: %w", err)
        }
        return err
    }
    return nil
}
```

## SOHO Deployment Considerations

### Simplified Approach

For homelab/SOHO:
- **Single instance**: No distributed rate limiting needed
- **Conservative limits**: Use free-tier limits by default
- **No Redis**: In-memory token bucket sufficient
- **Simple monitoring**: Log warnings when limits approached

### Performance

Expected behavior for SOHO load:
- **Normal operation**: Rarely hit rate limits
- **Burst handling**: 3-5 simultaneous operations
- **Wait times**: Usually < 100ms
- **Memory overhead**: < 1 KB per limiter

## Future Enhancements

1. **Dynamic limit adjustment**: Auto-detect provider limits
2. **Per-user limits**: Different limits per API key
3. **Distributed limiting**: Redis-based for multi-instance
4. **Priority queuing**: High-priority requests first

## Package Structure

```
internal/ratelimit/
├── limiter.go         # Token bucket implementation
├── adaptive.go        # Adaptive rate limiting
├── metrics.go         # Metrics collection
└── limiter_test.go    # Test suite
```

## Dependencies

- `golang.org/x/time/rate` - Token bucket rate limiter
- Standard library only

---

**Status:** Ready for implementation
**Complexity:** Low
**Priority:** High (prevents API throttling)
