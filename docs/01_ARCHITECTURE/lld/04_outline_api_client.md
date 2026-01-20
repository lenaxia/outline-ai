# Low-Level Design: Outline API Client

**Domain:** Outline API Integration
**Status:** Design
**Last Updated:** 2026-01-19
**Target Deployment:** Homelab/SOHO

## Purpose

All interactions with Outline workspace including document management, collections, search, and comments.

## Design Principles

1. **Rate Limited**: Respect API limits via rate limiter
2. **Retry Logic**: Automatic retries for transient failures
3. **Type-Safe**: Strongly-typed request/response structs
4. **Error Classification**: Permanent vs transient errors
5. **Context-Aware**: Support timeouts and cancellation

## API Client Structure

### Client Interface

```go
package outline

import (
    "context"
)

type Client interface {
    // Collections
    ListCollections(ctx context.Context) ([]*Collection, error)
    GetCollection(ctx context.Context, id string) (*Collection, error)

    // Documents
    GetDocument(ctx context.Context, id string) (*Document, error)
    ListDocuments(ctx context.Context, collectionID string) ([]*Document, error)
    CreateDocument(ctx context.Context, req *CreateDocumentRequest) (*Document, error)
    UpdateDocument(ctx context.Context, id string, req *UpdateDocumentRequest) (*Document, error)
    MoveDocument(ctx context.Context, id string, collectionID string) error
    SearchDocuments(ctx context.Context, query string, opts *SearchOptions) (*SearchResult, error)

    // Comments
    CreateComment(ctx context.Context, req *CreateCommentRequest) (*Comment, error)
    ListComments(ctx context.Context, documentID string) ([]*Comment, error)

    // Health
    Ping(ctx context.Context) error
}
```

### Domain Models

```go
package outline

import "time"

type Collection struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Description string    `json:"description"`
    CreatedAt   time.Time `json:"createdAt"`
    UpdatedAt   time.Time `json:"updatedAt"`
}

type Document struct {
    ID           string    `json:"id"`
    CollectionID string    `json:"collectionId"`
    Title        string    `json:"title"`
    Text         string    `json:"text"`
    CreatedAt    time.Time `json:"createdAt"`
    UpdatedAt    time.Time `json:"updatedAt"`
    PublishedAt  *time.Time `json:"publishedAt,omitempty"`
}

type CreateDocumentRequest struct {
    CollectionID string  `json:"collectionId"`
    Title        string  `json:"title"`
    Text         string  `json:"text"`
    Publish      bool    `json:"publish"`
    ParentDocumentID *string `json:"parentDocumentId,omitempty"`
}

type UpdateDocumentRequest struct {
    Title string  `json:"title,omitempty"`
    Text  string  `json:"text,omitempty"`
    Done  bool    `json:"done,omitempty"`
}

type Comment struct {
    ID         string    `json:"id"`
    DocumentID string    `json:"documentId"`
    Data       string    `json:"data"`
    CreatedAt  time.Time `json:"createdAt"`
}

type CommentContent struct {
    Type    string          `json:"type"`
    Content []ContentNode   `json:"content,omitempty"`
}

type ContentNode struct {
    Type    string          `json:"type"`
    Text    string          `json:"text,omitempty"`
    Content []ContentNode   `json:"content,omitempty"`
}

type CreateCommentRequest struct {
    DocumentID string         `json:"documentId"`
    Data       CommentContent `json:"data"`
}

func NewCommentContent(text string) CommentContent {
    return CommentContent{
        Type: "doc",
        Content: []ContentNode{
            {
                Type: "paragraph",
                Content: []ContentNode{
                    {
                        Type: "text",
                        Text: text,
                    },
                },
            },
        },
    }
}

type SearchOptions struct {
    Limit         int    `json:"limit,omitempty"`
    Offset        int    `json:"offset,omitempty"`
    CollectionID  string `json:"collectionId,omitempty"`
}

type SearchResult struct {
    Documents  []*Document `json:"data"`
    TotalCount int         `json:"total"`
}
```

## HTTP Client Implementation

### Main Client

```go
package outline

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"

    "github.com/yourusername/outline-ai/internal/ratelimit"
    "github.com/rs/zerolog/log"
)

type HTTPClient struct {
    httpClient *http.Client
    limiter    ratelimit.Limiter
    baseURL    string
    apiKey     string
    maxRetries int
    retryBackoff time.Duration
}

func NewHTTPClient(baseURL, apiKey string, limiter ratelimit.Limiter) *HTTPClient {
    return &HTTPClient{
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
        },
        limiter:      limiter,
        baseURL:      baseURL,
        apiKey:       apiKey,
        maxRetries:   3,
        retryBackoff: 1 * time.Second,
    }
}

func (c *HTTPClient) doRequest(ctx context.Context, method, endpoint string, body interface{}) ([]byte, error) {
    // Prepare request body
    var reqBody io.Reader
    if body != nil {
        jsonBody, err := json.Marshal(body)
        if err != nil {
            return nil, fmt.Errorf("failed to marshal request: %w", err)
        }
        reqBody = bytes.NewReader(jsonBody)
    }

    // Create request
    url := c.baseURL + endpoint
    req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }

    // Set headers
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+c.apiKey)

    // Retry loop
    var lastErr error
    for attempt := 0; attempt <= c.maxRetries; attempt++ {
        if attempt > 0 {
            backoff := time.Duration(attempt) * c.retryBackoff
            log.Debug().
                Int("attempt", attempt).
                Dur("backoff", backoff).
                Str("endpoint", endpoint).
                Msg("Retrying request")

            select {
            case <-time.After(backoff):
            case <-ctx.Done():
                return nil, ctx.Err()
            }
        }

        // Wait for rate limiter
        if err := c.limiter.Wait(ctx); err != nil {
            return nil, fmt.Errorf("rate limiter failed: %w", err)
        }

        // Make request
        resp, err := c.httpClient.Do(req)
        if err != nil {
            lastErr = err
            if !isRetriableError(err) {
                return nil, fmt.Errorf("request failed: %w", err)
            }
            continue
        }

        // Read response
        respBody, err := io.ReadAll(resp.Body)
        resp.Body.Close()
        if err != nil {
            lastErr = err
            continue
        }

        // Check status code
        if resp.StatusCode >= 200 && resp.StatusCode < 300 {
            return respBody, nil
        }

        // Handle error responses
        lastErr = classifyHTTPError(resp.StatusCode, respBody)
        if !isRetriableHTTPError(resp.StatusCode) {
            return nil, lastErr
        }
    }

    return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}
```

### Error Classification

```go
package outline

import (
    "errors"
    "fmt"
    "net/http"
)

// Package-specific errors for Outline API client
// These are distinct from ai.ErrRateLimited and other package errors
var (
    ErrUnauthorized    = errors.New("outline: unauthorized")
    ErrNotFound        = errors.New("outline: not found")
    ErrRateLimited     = errors.New("outline: rate limited")
    ErrServerError     = errors.New("outline: server error")
    ErrInvalidRequest  = errors.New("outline: invalid request")
)

func classifyHTTPError(statusCode int, body []byte) error {
    switch statusCode {
    case http.StatusUnauthorized:
        return ErrUnauthorized
    case http.StatusNotFound:
        return ErrNotFound
    case http.StatusTooManyRequests:
        return ErrRateLimited
    case http.StatusBadRequest:
        return fmt.Errorf("%w: %s", ErrInvalidRequest, string(body))
    case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
        return ErrServerError
    default:
        return fmt.Errorf("HTTP %d: %s", statusCode, string(body))
    }
}

func isRetriableHTTPError(statusCode int) bool {
    switch statusCode {
    case http.StatusTooManyRequests,
         http.StatusInternalServerError,
         http.StatusBadGateway,
         http.StatusServiceUnavailable,
         http.StatusGatewayTimeout:
        return true
    default:
        return false
    }
}

func isRetriableError(err error) bool {
    // Network errors, timeouts, etc. are retriable
    return true
}
```

## API Method Implementations

### Collections

```go
func (c *HTTPClient) ListCollections(ctx context.Context) ([]*Collection, error) {
    respBody, err := c.doRequest(ctx, "POST", "/collections.list", nil)
    if err != nil {
        return nil, err
    }

    var response struct {
        Data []*Collection `json:"data"`
    }
    if err := json.Unmarshal(respBody, &response); err != nil {
        return nil, fmt.Errorf("failed to parse response: %w", err)
    }

    return response.Data, nil
}

func (c *HTTPClient) GetCollection(ctx context.Context, id string) (*Collection, error) {
    req := map[string]string{"id": id}
    respBody, err := c.doRequest(ctx, "POST", "/collections.info", req)
    if err != nil {
        return nil, err
    }

    var response struct {
        Data *Collection `json:"data"`
    }
    if err := json.Unmarshal(respBody, &response); err != nil {
        return nil, fmt.Errorf("failed to parse response: %w", err)
    }

    return response.Data, nil
}
```

### Documents

```go
func (c *HTTPClient) GetDocument(ctx context.Context, id string) (*Document, error) {
    req := map[string]string{"id": id}
    respBody, err := c.doRequest(ctx, "POST", "/documents.info", req)
    if err != nil {
        return nil, err
    }

    var response struct {
        Data *Document `json:"data"`
    }
    if err := json.Unmarshal(respBody, &response); err != nil {
        return nil, fmt.Errorf("failed to parse response: %w", err)
    }

    return response.Data, nil
}

func (c *HTTPClient) UpdateDocument(ctx context.Context, id string, req *UpdateDocumentRequest) (*Document, error) {
    payload := map[string]interface{}{
        "id": id,
    }
    if req.Title != "" {
        payload["title"] = req.Title
    }
    if req.Text != "" {
        payload["text"] = req.Text
    }
    payload["done"] = req.Done

    respBody, err := c.doRequest(ctx, "POST", "/documents.update", payload)
    if err != nil {
        return nil, err
    }

    var response struct {
        Data *Document `json:"data"`
    }
    if err := json.Unmarshal(respBody, &response); err != nil {
        return nil, fmt.Errorf("failed to parse response: %w", err)
    }

    return response.Data, nil
}

func (c *HTTPClient) MoveDocument(ctx context.Context, id string, collectionID string) error {
    req := map[string]string{
        "id":           id,
        "collectionId": collectionID,
    }

    _, err := c.doRequest(ctx, "POST", "/documents.move", req)
    return err
}

func (c *HTTPClient) SearchDocuments(ctx context.Context, query string, opts *SearchOptions) (*SearchResult, error) {
    req := map[string]interface{}{
        "query": query,
    }
    if opts != nil {
        if opts.Limit > 0 {
            req["limit"] = opts.Limit
        }
        if opts.Offset > 0 {
            req["offset"] = opts.Offset
        }
        if opts.CollectionID != "" {
            req["collectionId"] = opts.CollectionID
        }
    }

    respBody, err := c.doRequest(ctx, "POST", "/documents.search", req)
    if err != nil {
        return nil, err
    }

    var result SearchResult
    if err := json.Unmarshal(respBody, &result); err != nil {
        return nil, fmt.Errorf("failed to parse response: %w", err)
    }

    return &result, nil
}
```

### Comments

```go
func (c *HTTPClient) CreateComment(ctx context.Context, req *CreateCommentRequest) (*Comment, error) {
    respBody, err := c.doRequest(ctx, "POST", "/comments.create", req)
    if err != nil {
        return nil, err
    }

    var response struct {
        Data *Comment `json:"data"`
    }
    if err := json.Unmarshal(respBody, &response); err != nil {
        return nil, fmt.Errorf("failed to parse response: %w", err)
    }

    return response.Data, nil
}

func (c *HTTPClient) ListComments(ctx context.Context, documentID string) ([]*Comment, error) {
    req := map[string]string{"documentId": documentID}
    respBody, err := c.doRequest(ctx, "POST", "/comments.list", req)
    if err != nil {
        return nil, err
    }

    var response struct {
        Data []*Comment `json:"data"`
    }
    if err := json.Unmarshal(respBody, &response); err != nil {
        return nil, fmt.Errorf("failed to parse response: %w", err)
    }

    return response.Data, nil
}
```

### Health Check

```go
func (c *HTTPClient) Ping(ctx context.Context) error {
    _, err := c.doRequest(ctx, "POST", "/auth.info", nil)
    return err
}
```

## Testing Strategy

### Unit Tests

```go
func TestHTTPClient_GetDocument(t *testing.T)
func TestHTTPClient_UpdateDocument(t *testing.T)
func TestHTTPClient_MoveDocument(t *testing.T)
func TestHTTPClient_SearchDocuments(t *testing.T)
func TestHTTPClient_CreateComment(t *testing.T)
func TestHTTPClient_RetryLogic(t *testing.T)
func TestHTTPClient_RateLimiting(t *testing.T)
func TestErrorClassification(t *testing.T)
```

### Mock Server for Tests

```go
func setupMockServer(t *testing.T) (*httptest.Server, *HTTPClient) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Mock responses based on endpoint
        switch r.URL.Path {
        case "/documents.info":
            json.NewEncoder(w).Encode(map[string]interface{}{
                "data": map[string]interface{}{
                    "id":    "doc123",
                    "title": "Test Document",
                    "text":  "Content here",
                },
            })
        default:
            http.NotFound(w, r)
        }
    }))

    limiter := ratelimit.NewTokenBucketLimiter(1000)
    client := NewHTTPClient(server.URL, "test-key", limiter)

    t.Cleanup(func() {
        server.Close()
    })

    return server, client
}
```

## Performance Considerations

### For SOHO Deployment

- **Concurrent requests**: 3-5 simultaneous
- **Timeout**: 30 seconds per request
- **Retry attempts**: 3 maximum
- **Rate limit**: 60 requests/minute

### Caching Strategy

```go
type CachedClient struct {
    client Client
    cache  map[string]cacheEntry
    ttl    time.Duration
}

type cacheEntry struct {
    data      interface{}
    expiresAt time.Time
}

func (c *CachedClient) GetDocument(ctx context.Context, id string) (*Document, error) {
    // Check cache
    if entry, ok := c.cache[id]; ok && time.Now().Before(entry.expiresAt) {
        return entry.data.(*Document), nil
    }

    // Fetch from API
    doc, err := c.client.GetDocument(ctx, id)
    if err != nil {
        return nil, err
    }

    // Update cache
    c.cache[id] = cacheEntry{
        data:      doc,
        expiresAt: time.Now().Add(c.ttl),
    }

    return doc, nil
}
```

## Package Structure

```
internal/outline/
├── client.go          # Main HTTP client
├── models.go          # Domain models
├── errors.go          # Error types
├── retry.go           # Retry logic
├── cache.go           # Optional caching
└── client_test.go     # Test suite
```

## Dependencies

- Standard library `net/http`, `encoding/json`
- `github.com/yourusername/outline-ai/internal/ratelimit` - Rate limiting
- `github.com/rs/zerolog` - Logging

---

**Status:** Ready for implementation
**Complexity:** Medium
**Priority:** High (core integration)
