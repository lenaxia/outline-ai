# Low-Level Design: AI Client (OpenAI-Compatible)

**Domain:** AI Integration
**Status:** Design
**Last Updated:** 2026-01-19
**Target Deployment:** Homelab/SOHO

## Purpose

Interact with any OpenAI-compatible AI service for document classification, question answering, content enhancement, and related document discovery.

## Design Principles

1. **Provider Agnostic**: Works with OpenAI, Claude, local models, Ollama
2. **Structured Responses**: JSON-mode responses with type-safe parsing
3. **Circuit Breaker**: Pause after consecutive failures
4. **Token Management**: Handle token limits gracefully
5. **SOHO Optimized**: Single endpoint, simple configuration

## Client Interface

### Main Interface

```go
package ai

import (
    "context"
)

type Client interface {
    // Document classification for filing
    ClassifyDocument(ctx context.Context, req *ClassificationRequest) (*ClassificationResponse, error)

    // Question answering with context
    AnswerQuestion(ctx context.Context, req *QuestionRequest) (*QuestionResponse, error)

    // Content enhancement
    GenerateSummary(ctx context.Context, req *SummaryRequest) (*SummaryResponse, error)
    EnhanceTitle(ctx context.Context, req *TitleRequest) (*TitleResponse, error)
    GenerateSearchTerms(ctx context.Context, req *SearchTermsRequest) (*SearchTermsResponse, error)

    // Related documents
    FindRelatedDocuments(ctx context.Context, req *RelatedDocsRequest) (*RelatedDocsResponse, error)

    // Health
    Ping(ctx context.Context) error
}
```

## Domain Models

### Classification Models

```go
package ai

type ClassificationRequest struct {
    DocumentTitle   string             `json:"document_title"`
    DocumentContent string             `json:"document_content"`
    UserGuidance    string             `json:"user_guidance,omitempty"`
    Taxonomy        *TaxonomyContext   `json:"taxonomy"`
}

// TaxonomyContext wraps taxonomy information for AI classification
// The canonical taxonomy models are defined in the taxonomy package (LLD-06)
type TaxonomyContext struct {
    Collections []TaxonomyCollection `json:"collections"`
}

// TaxonomyCollection represents a collection with sample documents for AI context
// This is a simplified view of taxonomy.CollectionTaxonomy for AI consumption
type TaxonomyCollection struct {
    ID              string   `json:"id"`
    Name            string   `json:"name"`
    Description     string   `json:"description"`
    SampleDocuments []string `json:"sample_documents,omitempty"`
}

type ClassificationResponse struct {
    CollectionID string                    `json:"collection_id"`
    Confidence   float64                   `json:"confidence"`
    Reasoning    string                    `json:"reasoning"`
    Alternatives []AlternativeClassification `json:"alternatives,omitempty"`
    SearchTerms  []string                  `json:"search_terms"`
}

type AlternativeClassification struct {
    CollectionID string  `json:"collection_id"`
    Confidence   float64 `json:"confidence"`
    Reasoning    string  `json:"reasoning"`
}
```

### Question Answering Models

```go
package ai

type QuestionRequest struct {
    Question      string            `json:"question"`
    ContextDocs   []ContextDocument `json:"context_documents"`
}

type ContextDocument struct {
    Title   string `json:"title"`
    Excerpt string `json:"excerpt"`
    URL     string `json:"url"`
}

type QuestionResponse struct {
    Answer    string         `json:"answer"`
    Citations []CitationInfo `json:"citations"`
    Confidence float64       `json:"confidence"`
}

type CitationInfo struct {
    DocumentTitle string `json:"document_title"`
    DocumentURL   string `json:"document_url"`
}
```

### Content Enhancement Models

```go
package ai

type SummaryRequest struct {
    DocumentTitle   string `json:"document_title"`
    DocumentContent string `json:"document_content"`
}

type SummaryResponse struct {
    Summary string `json:"summary"`
}

type TitleRequest struct {
    CurrentTitle    string `json:"current_title"`
    DocumentContent string `json:"document_content"`
}

type TitleResponse struct {
    SuggestedTitle string  `json:"suggested_title"`
    Confidence     float64 `json:"confidence"`
}

type SearchTermsRequest struct {
    DocumentTitle   string `json:"document_title"`
    DocumentContent string `json:"document_content"`
}

type SearchTermsResponse struct {
    SearchTerms []string `json:"search_terms"`
}

type RelatedDocsRequest struct {
    DocumentTitle   string   `json:"document_title"`
    DocumentContent string   `json:"document_content"`
    AvailableDocs   []string `json:"available_documents"`
}

type RelatedDocsResponse struct {
    RelatedDocuments []RelatedDocument `json:"related_documents"`
}

type RelatedDocument struct {
    Title      string  `json:"title"`
    Relevance  float64 `json:"relevance"`
    Reason     string  `json:"reason"`
}
```

## OpenAI Client Implementation

### HTTP Client

```go
package ai

import (
    "context"
    "fmt"
    "time"

    "github.com/sashabaranov/go-openai"
    "github.com/rs/zerolog/log"
)

type OpenAIClient struct {
    client          *openai.Client
    model           string
    maxTokens       int
    timeout         time.Duration
    circuitBreaker  *CircuitBreaker
}

func NewOpenAIClient(endpoint, apiKey, model string, maxTokens int, timeout time.Duration) (*OpenAIClient, error) {
    config := openai.DefaultConfig(apiKey)
    config.BaseURL = endpoint

    client := openai.NewClientWithConfig(config)

    return &OpenAIClient{
        client:         client,
        model:          model,
        maxTokens:      maxTokens,
        timeout:        timeout,
        circuitBreaker: NewCircuitBreaker(5, 5*time.Minute),
    }, nil
}

func (c *OpenAIClient) makeCompletionRequest(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
    // Check circuit breaker
    if !c.circuitBreaker.Allow() {
        return "", fmt.Errorf("circuit breaker open: too many recent failures")
    }

    // Create context with timeout
    ctx, cancel := context.WithTimeout(ctx, c.timeout)
    defer cancel()

    // Build request
    req := openai.ChatCompletionRequest{
        Model: c.model,
        Messages: []openai.ChatCompletionMessage{
            {
                Role:    openai.ChatMessageRoleSystem,
                Content: systemPrompt,
            },
            {
                Role:    openai.ChatMessageRoleUser,
                Content: userPrompt,
            },
        },
        MaxTokens:   c.maxTokens,
        Temperature: 0.3, // Lower temperature for more consistent responses
    }

    // Make request
    resp, err := c.client.CreateChatCompletion(ctx, req)
    if err != nil {
        c.circuitBreaker.RecordFailure()
        return "", fmt.Errorf("AI request failed: %w", err)
    }

    if len(resp.Choices) == 0 {
        c.circuitBreaker.RecordFailure()
        return "", fmt.Errorf("no response choices returned")
    }

    c.circuitBreaker.RecordSuccess()
    return resp.Choices[0].Message.Content, nil
}
```

### Classification Implementation

```go
package ai

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"
)

func (c *OpenAIClient) ClassifyDocument(ctx context.Context, req *ClassificationRequest) (*ClassificationResponse, error) {
    systemPrompt := buildClassificationSystemPrompt()
    userPrompt := buildClassificationUserPrompt(req)

    responseText, err := c.makeCompletionRequest(ctx, systemPrompt, userPrompt)
    if err != nil {
        return nil, err
    }

    // Parse JSON response
    var response ClassificationResponse
    if err := json.Unmarshal([]byte(responseText), &response); err != nil {
        return nil, fmt.Errorf("failed to parse classification response: %w", err)
    }

    // Validate response
    if err := validateClassificationResponse(&response, req.Taxonomy); err != nil {
        return nil, fmt.Errorf("invalid classification response: %w", err)
    }

    log.Info().
        Str("collection_id", response.CollectionID).
        Float64("confidence", response.Confidence).
        Str("reasoning", response.Reasoning).
        Msg("Document classified")

    return &response, nil
}

func buildClassificationSystemPrompt() string {
    return `You are a document classification assistant. Your task is to analyze documents and determine which collection they belong to.

RESPONSE FORMAT:
You must respond with a valid JSON object with this exact structure:
{
  "collection_id": "the ID of the best matching collection",
  "confidence": 0.85,
  "reasoning": "brief explanation of why this collection fits",
  "alternatives": [
    {
      "collection_id": "alternative_id",
      "confidence": 0.65,
      "reasoning": "why this is also relevant"
    }
  ],
  "search_terms": ["term1", "term2", "term3"]
}

INSTRUCTIONS:
- Analyze document title, content, and user guidance (if provided)
- User guidance should heavily influence your decision
- Return confidence between 0.0 and 1.0
- Include up to 3 alternatives if confidence < 0.9
- Generate 5-10 relevant search terms
- Be concise but clear in reasoning`
}

func buildClassificationUserPrompt(req *ClassificationRequest) string {
    var sb strings.Builder

    sb.WriteString("DOCUMENT TO CLASSIFY:\n")
    sb.WriteString(fmt.Sprintf("Title: %s\n\n", req.DocumentTitle))

    // Truncate content if too long
    content := req.DocumentContent
    if len(content) > 5000 {
        content = content[:5000] + "... (truncated)"
    }
    sb.WriteString(fmt.Sprintf("Content:\n%s\n\n", content))

    if req.UserGuidance != "" {
        sb.WriteString(fmt.Sprintf("USER GUIDANCE: %s\n\n", req.UserGuidance))
    }

    sb.WriteString("AVAILABLE COLLECTIONS:\n")
    for _, col := range req.Taxonomy.Collections {
        sb.WriteString(fmt.Sprintf("\nID: %s\n", col.ID))
        sb.WriteString(fmt.Sprintf("Name: %s\n", col.Name))
        sb.WriteString(fmt.Sprintf("Description: %s\n", col.Description))

        if len(col.SampleDocuments) > 0 {
            sb.WriteString("Sample documents: ")
            sb.WriteString(strings.Join(col.SampleDocuments, ", "))
            sb.WriteString("\n")
        }
    }

    return sb.String()
}

func validateClassificationResponse(resp *ClassificationResponse, taxonomy *TaxonomyContext) error {
    if resp.CollectionID == "" {
        return fmt.Errorf("collection_id is required")
    }

    if resp.Confidence < 0 || resp.Confidence > 1 {
        return fmt.Errorf("confidence must be between 0 and 1")
    }

    // Verify collection exists in taxonomy
    found := false
    for _, col := range taxonomy.Collections {
        if col.ID == resp.CollectionID {
            found = true
            break
        }
    }
    if !found {
        return fmt.Errorf("collection_id %s not found in taxonomy", resp.CollectionID)
    }

    return nil
}
```

### Question Answering Implementation

```go
package ai

func (c *OpenAIClient) AnswerQuestion(ctx context.Context, req *QuestionRequest) (*QuestionResponse, error) {
    systemPrompt := buildQuestionSystemPrompt()
    userPrompt := buildQuestionUserPrompt(req)

    responseText, err := c.makeCompletionRequest(ctx, systemPrompt, userPrompt)
    if err != nil {
        return nil, err
    }

    // Parse JSON response
    var response QuestionResponse
    if err := json.Unmarshal([]byte(responseText), &response); err != nil {
        return nil, fmt.Errorf("failed to parse question response: %w", err)
    }

    log.Info().
        Str("question", req.Question).
        Float64("confidence", response.Confidence).
        Int("citations", len(response.Citations)).
        Msg("Question answered")

    return &response, nil
}

func buildQuestionSystemPrompt() string {
    return `You are a helpful assistant that answers questions based on provided context documents.

RESPONSE FORMAT:
{
  "answer": "your detailed answer here",
  "citations": [
    {
      "document_title": "title of source document",
      "document_url": "URL from context"
    }
  ],
  "confidence": 0.9
}

INSTRUCTIONS:
- Only use information from the provided context documents
- Cite sources by including them in the citations array
- If you cannot answer from context, say so and set confidence low
- Be clear, concise, and accurate
- Format answer in markdown`
}

func buildQuestionUserPrompt(req *QuestionRequest) string {
    var sb strings.Builder

    sb.WriteString(fmt.Sprintf("QUESTION: %s\n\n", req.Question))
    sb.WriteString("CONTEXT DOCUMENTS:\n\n")

    for i, doc := range req.ContextDocs {
        sb.WriteString(fmt.Sprintf("Document %d:\n", i+1))
        sb.WriteString(fmt.Sprintf("Title: %s\n", doc.Title))
        sb.WriteString(fmt.Sprintf("URL: %s\n", doc.URL))
        sb.WriteString(fmt.Sprintf("Excerpt:\n%s\n\n", doc.Excerpt))
    }

    return sb.String()
}
```

### Content Enhancement Implementation

```go
package ai

func (c *OpenAIClient) GenerateSummary(ctx context.Context, req *SummaryRequest) (*SummaryResponse, error) {
    systemPrompt := "You are a document summarization assistant. Create concise 2-3 sentence summaries. Respond with JSON: {\"summary\": \"your summary here\"}"

    content := req.DocumentContent
    if len(content) > 5000 {
        content = content[:5000] + "... (truncated)"
    }

    userPrompt := fmt.Sprintf("Title: %s\n\nContent:\n%s", req.DocumentTitle, content)

    responseText, err := c.makeCompletionRequest(ctx, systemPrompt, userPrompt)
    if err != nil {
        return nil, err
    }

    var response SummaryResponse
    if err := json.Unmarshal([]byte(responseText), &response); err != nil {
        return nil, fmt.Errorf("failed to parse summary response: %w", err)
    }

    return &response, nil
}

func (c *OpenAIClient) EnhanceTitle(ctx context.Context, req *TitleRequest) (*TitleResponse, error) {
    systemPrompt := "You are a title improvement assistant. Create clear, descriptive titles. Respond with JSON: {\"suggested_title\": \"improved title\", \"confidence\": 0.9}"

    content := req.DocumentContent
    if len(content) > 3000 {
        content = content[:3000] + "... (truncated)"
    }

    userPrompt := fmt.Sprintf("Current title: %s\n\nContent:\n%s", req.CurrentTitle, content)

    responseText, err := c.makeCompletionRequest(ctx, systemPrompt, userPrompt)
    if err != nil {
        return nil, err
    }

    var response TitleResponse
    if err := json.Unmarshal([]byte(responseText), &response); err != nil {
        return nil, fmt.Errorf("failed to parse title response: %w", err)
    }

    return &response, nil
}

func (c *OpenAIClient) GenerateSearchTerms(ctx context.Context, req *SearchTermsRequest) (*SearchTermsResponse, error) {
    systemPrompt := "You are a search term extraction assistant. Extract 5-10 relevant keywords and phrases. Respond with JSON: {\"search_terms\": [\"term1\", \"term2\"]}"

    content := req.DocumentContent
    if len(content) > 4000 {
        content = content[:4000] + "... (truncated)"
    }

    userPrompt := fmt.Sprintf("Title: %s\n\nContent:\n%s", req.DocumentTitle, content)

    responseText, err := c.makeCompletionRequest(ctx, systemPrompt, userPrompt)
    if err != nil {
        return nil, err
    }

    var response SearchTermsResponse
    if err := json.Unmarshal([]byte(responseText), &response); err != nil {
        return nil, fmt.Errorf("failed to parse search terms response: %w", err)
    }

    return &response, nil
}
```

## Circuit Breaker

### Circuit Breaker Implementation

```go
package ai

import (
    "sync"
    "time"
)

type CircuitBreaker struct {
    mu              sync.RWMutex
    failureCount    int
    lastFailureTime time.Time
    threshold       int
    resetTimeout    time.Duration
    isOpen          bool
}

func NewCircuitBreaker(threshold int, resetTimeout time.Duration) *CircuitBreaker {
    return &CircuitBreaker{
        threshold:    threshold,
        resetTimeout: resetTimeout,
    }
}

func (cb *CircuitBreaker) Allow() bool {
    cb.mu.RLock()
    defer cb.mu.RUnlock()

    if !cb.isOpen {
        return true
    }

    // Check if we should reset
    if time.Since(cb.lastFailureTime) > cb.resetTimeout {
        return true
    }

    return false
}

func (cb *CircuitBreaker) RecordSuccess() {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    cb.failureCount = 0
    cb.isOpen = false
}

func (cb *CircuitBreaker) RecordFailure() {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    cb.failureCount++
    cb.lastFailureTime = time.Now()

    if cb.failureCount >= cb.threshold {
        cb.isOpen = true
        log.Warn().
            Int("failures", cb.failureCount).
            Msg("Circuit breaker opened due to consecutive failures")
    }
}

func (cb *CircuitBreaker) Reset() {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    cb.failureCount = 0
    cb.isOpen = false
}
```

## Testing Strategy

### Unit Tests

```go
func TestOpenAIClient_ClassifyDocument(t *testing.T)
func TestOpenAIClient_AnswerQuestion(t *testing.T)
func TestOpenAIClient_GenerateSummary(t *testing.T)
func TestOpenAIClient_EnhanceTitle(t *testing.T)
func TestOpenAIClient_GenerateSearchTerms(t *testing.T)
func TestCircuitBreaker_OpenClose(t *testing.T)
func TestCircuitBreaker_Reset(t *testing.T)
func TestValidateClassificationResponse(t *testing.T)
```

### Mock Client for Tests

```go
type MockAIClient struct {
    ClassifyDocumentFunc        func(ctx context.Context, req *ClassificationRequest) (*ClassificationResponse, error)
    AnswerQuestionFunc          func(ctx context.Context, req *QuestionRequest) (*QuestionResponse, error)
    GenerateSummaryFunc         func(ctx context.Context, req *SummaryRequest) (*SummaryResponse, error)
    EnhanceTitleFunc            func(ctx context.Context, req *TitleRequest) (*TitleResponse, error)
    GenerateSearchTermsFunc     func(ctx context.Context, req *SearchTermsRequest) (*SearchTermsResponse, error)
}

func (m *MockAIClient) ClassifyDocument(ctx context.Context, req *ClassificationRequest) (*ClassificationResponse, error) {
    if m.ClassifyDocumentFunc != nil {
        return m.ClassifyDocumentFunc(ctx, req)
    }
    return &ClassificationResponse{
        CollectionID: "default-collection",
        Confidence:   0.9,
        Reasoning:    "Mock classification",
        SearchTerms:  []string{"test", "mock"},
    }, nil
}
```

## Error Handling

### Error Types

```go
package ai

import "errors"

// Package-specific errors for AI client
// These are distinct from outline.ErrRateLimited and other package errors
var (
    ErrCircuitBreakerOpen = errors.New("ai: circuit breaker open")
    ErrInvalidResponse    = errors.New("ai: invalid response")
    ErrTimeout            = errors.New("ai: request timeout")
    ErrTokenLimitExceeded = errors.New("ai: token limit exceeded")
    ErrRateLimited        = errors.New("ai: rate limited by provider")
)
```

## SOHO Deployment Considerations

### Simplifications for Homelab

1. **Single endpoint**: No load balancing across multiple AI providers
2. **No embedding cache**: Generate embeddings on-demand
3. **Simple circuit breaker**: No distributed circuit breaker state
4. **No request queuing**: Direct synchronous calls
5. **Provider flexibility**: Easy to switch between OpenAI, Claude, local models

### Cost Management

```go
type UsageTracker struct {
    mu              sync.Mutex
    totalRequests   int
    totalTokens     int64
    estimatedCost   float64
    resetInterval   time.Duration
}

func (t *UsageTracker) RecordRequest(tokensUsed int) {
    t.mu.Lock()
    defer t.mu.Unlock()

    t.totalRequests++
    t.totalTokens += int64(tokensUsed)

    // Rough cost estimate ($0.03 per 1K tokens for GPT-4)
    t.estimatedCost = float64(t.totalTokens) / 1000.0 * 0.03
}

func (t *UsageTracker) GetStats() (int, int64, float64) {
    t.mu.Lock()
    defer t.mu.Unlock()
    return t.totalRequests, t.totalTokens, t.estimatedCost
}
```

## Package Structure

```
internal/ai/
├── client.go           # Interface definition
├── openai.go           # OpenAI implementation
├── models.go           # Request/response models
├── prompts.go          # System prompt builders
├── circuit_breaker.go  # Circuit breaker
├── usage_tracker.go    # Usage tracking
├── errors.go           # Error types
└── ai_test.go          # Test suite
```

## Dependencies

- `github.com/sashabaranov/go-openai` - OpenAI SDK (works with compatible APIs)
- `github.com/rs/zerolog` - Logging
- Standard library for JSON parsing

---

**Status:** Ready for implementation
**Complexity:** Medium
**Priority:** High (core AI functionality)
