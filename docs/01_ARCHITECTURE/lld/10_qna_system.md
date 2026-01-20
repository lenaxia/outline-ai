# Low-Level Design: Q&A System

**Domain:** Question Answering
**Status:** Design
**Last Updated:** 2026-01-19
**Target Deployment:** Homelab/SOHO

## Purpose

Answer user questions using workspace context through document search, AI reasoning, and comment-based responses.

## Design Principles

1. **Context-Aware**: Search workspace for relevant information
2. **Deduplication**: Track answered questions to prevent re-answering
3. **Citation-Based**: Always cite source documents
4. **Non-Invasive**: Use comments instead of inline edits
5. **Works on Free Tier**: Only uses free Outline APIs

## Q&A Flow

### High-Level Process

```
User adds /ai question → Detect question → Check if answered →
Search workspace → Build context → Ask AI → Post comment →
Mark as answered
```

## Domain Models

### Question State

```go
package qna

import (
    "time"
)

type QuestionState struct {
    QuestionHash    string
    DocumentID      string
    QuestionText    string
    ProcessedAt     time.Time
    AnswerDelivered bool
    CommentID       *string
    LastError       *string
    RetryCount      int
}

type QuestionRequest struct {
    DocumentID   string
    QuestionText string
}

type Answer struct {
    Text       string
    Citations  []Citation
    Confidence float64
}

type Citation struct {
    DocumentTitle string
    DocumentURL   string
    Excerpt       string
}
```

## Q&A Service Interface

### Service Definition

```go
package qna

import (
    "context"

    "github.com/yourusername/outline-ai/internal/outline"
)

type Service interface {
    // Process question from document
    ProcessQuestion(ctx context.Context, doc *outline.Document, questionText string) error

    // Check if question already answered
    IsAnswered(ctx context.Context, doc *outline.Document, questionText string) (bool, error)

    // Get answer history for document
    GetAnswerHistory(ctx context.Context, documentID string) ([]*QuestionState, error)
}
```

## Service Implementation

### Main Service

```go
package qna

import (
    "context"
    "fmt"

    "github.com/yourusername/outline-ai/internal/ai"
    "github.com/yourusername/outline-ai/internal/outline"
    "github.com/yourusername/outline-ai/internal/persistence"
    "github.com/rs/zerolog/log"
)

type DefaultService struct {
    aiClient      ai.Client
    outlineClient outline.Client
    storage       persistence.Storage
    searcher      DocumentSearcher
    maxContext    int
}

type DocumentSearcher interface {
    SearchRelevant(ctx context.Context, query string, limit int) ([]*outline.Document, error)
}

func NewDefaultService(
    aiClient ai.Client,
    outlineClient outline.Client,
    storage persistence.Storage,
    searcher DocumentSearcher,
    maxContext int,
) *DefaultService {
    return &DefaultService{
        aiClient:      aiClient,
        outlineClient: outlineClient,
        storage:       storage,
        searcher:      searcher,
        maxContext:    maxContext,
    }
}

func (s *DefaultService) ProcessQuestion(ctx context.Context, doc *outline.Document, questionText string) error {
    // Check if already answered
    isAnswered, err := s.IsAnswered(ctx, doc, questionText)
    if err != nil {
        return fmt.Errorf("failed to check answer status: %w", err)
    }

    if isAnswered {
        log.Info().
            Str("document_id", doc.ID).
            Str("question", questionText).
            Msg("Question already answered, skipping")
        return nil
    }

    log.Info().
        Str("document_id", doc.ID).
        Str("question", questionText).
        Msg("Processing question")

    // Search for relevant documents
    relevantDocs, err := s.searcher.SearchRelevant(ctx, questionText, s.maxContext)
    if err != nil {
        return fmt.Errorf("failed to search documents: %w", err)
    }

    log.Debug().
        Int("relevant_docs", len(relevantDocs)).
        Msg("Found relevant documents")

    // Build context for AI
    contextDocs := s.buildContext(relevantDocs)

    // Get answer from AI
    answer, err := s.getAnswer(ctx, questionText, contextDocs)
    if err != nil {
        return fmt.Errorf("failed to get answer: %w", err)
    }

    // Post answer as comment
    commentID, err := s.postAnswer(ctx, doc, answer)
    if err != nil {
        return fmt.Errorf("failed to post answer: %w", err)
    }

    // Mark as answered
    if err := s.markAnswered(ctx, doc, questionText, commentID); err != nil {
        log.Warn().
            Err(err).
            Str("document_id", doc.ID).
            Msg("Failed to mark question as answered")
    }

    log.Info().
        Str("document_id", doc.ID).
        Str("question", questionText).
        Float64("confidence", answer.Confidence).
        Msg("Question answered successfully")

    return nil
}

func (s *DefaultService) IsAnswered(ctx context.Context, doc *outline.Document, questionText string) (bool, error) {
    questionHash := GenerateQuestionHash(doc.ID, questionText)
    return s.storage.HasAnsweredQuestion(ctx, questionHash)
}

func (s *DefaultService) GetAnswerHistory(ctx context.Context, documentID string) ([]*QuestionState, error) {
    // Implementation would query storage for all questions for this document
    // Simplified for LLD
    return nil, nil
}
```

### Context Building

```go
package qna

func (s *DefaultService) buildContext(docs []*outline.Document) []ai.ContextDocument {
    contextDocs := make([]ai.ContextDocument, 0, len(docs))

    for _, doc := range docs {
        excerpt := s.extractRelevantExcerpt(doc.Text, 500)

        contextDocs = append(contextDocs, ai.ContextDocument{
            Title:   doc.Title,
            Excerpt: excerpt,
            URL:     fmt.Sprintf("outline://doc/%s", doc.ID),
        })
    }

    return contextDocs
}

func (s *DefaultService) extractRelevantExcerpt(text string, maxLength int) string {
    if len(text) <= maxLength {
        return text
    }

    // Simple truncation - could be enhanced with smart extraction
    return text[:maxLength] + "..."
}
```

### Answer Generation

```go
package qna

func (s *DefaultService) getAnswer(ctx context.Context, question string, contextDocs []ai.ContextDocument) (*Answer, error) {
    // Call AI service
    questionReq := &ai.QuestionRequest{
        Question:    question,
        ContextDocs: contextDocs,
    }

    aiResponse, err := s.aiClient.AnswerQuestion(ctx, questionReq)
    if err != nil {
        return nil, fmt.Errorf("AI request failed: %w", err)
    }

    // Convert to internal answer format
    answer := &Answer{
        Text:       aiResponse.Answer,
        Confidence: aiResponse.Confidence,
        Citations:  make([]Citation, len(aiResponse.Citations)),
    }

    for i, cite := range aiResponse.Citations {
        answer.Citations[i] = Citation{
            DocumentTitle: cite.DocumentTitle,
            DocumentURL:   cite.DocumentURL,
        }
    }

    return answer, nil
}
```

### Answer Posting

```go
package qna

func (s *DefaultService) postAnswer(ctx context.Context, doc *outline.Document, answer *Answer) (string, error) {
    // Format answer with citations
    answerText := s.formatAnswer(answer)

    // Create comment
    commentReq := &outline.CreateCommentRequest{
        DocumentID: doc.ID,
        Data: map[string]interface{}{
            "type": "doc",
            "content": []map[string]interface{}{
                {
                    "type": "paragraph",
                    "content": []map[string]interface{}{
                        {
                            "type": "text",
                            "text": answerText,
                        },
                    },
                },
            },
        },
    }

    comment, err := s.outlineClient.CreateComment(ctx, commentReq)
    if err != nil {
        return "", fmt.Errorf("failed to create comment: %w", err)
    }

    return comment.ID, nil
}

func (s *DefaultService) formatAnswer(answer *Answer) string {
    var formatted strings.Builder

    formatted.WriteString("**AI Answer**\n\n")
    formatted.WriteString(answer.Text)
    formatted.WriteString("\n\n")

    if len(answer.Citations) > 0 {
        formatted.WriteString("**Sources:**\n")
        for _, cite := range answer.Citations {
            formatted.WriteString(fmt.Sprintf("- [%s](%s)\n", cite.DocumentTitle, cite.DocumentURL))
        }
    }

    formatted.WriteString(fmt.Sprintf("\n*Confidence: %.0f%%*", answer.Confidence*100))

    return formatted.String()
}
```

### State Tracking

```go
package qna

func (s *DefaultService) markAnswered(ctx context.Context, doc *outline.Document, questionText, commentID string) error {
    questionHash := GenerateQuestionHash(doc.ID, questionText)

    state := &persistence.QuestionState{
        QuestionHash:    questionHash,
        DocumentID:      doc.ID,
        QuestionText:    questionText,
        ProcessedAt:     time.Now(),
        AnswerDelivered: true,
        CommentID:       &commentID,
    }

    return s.storage.MarkQuestionAnswered(ctx, state)
}
```

## Document Search

### Keyword Extraction

```go
package qna

import (
    "strings"
)

type KeywordExtractor struct {
    stopWords map[string]bool
}

func NewKeywordExtractor() *KeywordExtractor {
    stopWords := map[string]bool{
        "what": true, "is": true, "the": true, "a": true, "an": true,
        "how": true, "why": true, "when": true, "where": true, "who": true,
        "are": true, "do": true, "does": true, "can": true, "should": true,
    }

    return &KeywordExtractor{
        stopWords: stopWords,
    }
}

func (e *KeywordExtractor) ExtractKeywords(question string) []string {
    // Convert to lowercase and split
    words := strings.Fields(strings.ToLower(question))

    var keywords []string
    for _, word := range words {
        // Remove punctuation
        word = strings.Trim(word, "?.,!;:")

        // Skip stop words and short words
        if len(word) > 2 && !e.stopWords[word] {
            keywords = append(keywords, word)
        }
    }

    return keywords
}
```

### Relevance Searcher

```go
package qna

type RelevanceSearcher struct {
    outlineClient   outline.Client
    keywordExtractor *KeywordExtractor
}

func NewRelevanceSearcher(client outline.Client) *RelevanceSearcher {
    return &RelevanceSearcher{
        outlineClient:   client,
        keywordExtractor: NewKeywordExtractor(),
    }
}

func (s *RelevanceSearcher) SearchRelevant(ctx context.Context, query string, limit int) ([]*outline.Document, error) {
    // Extract keywords
    keywords := s.keywordExtractor.ExtractKeywords(query)

    if len(keywords) == 0 {
        // Fallback to full query
        keywords = []string{query}
    }

    log.Debug().
        Strs("keywords", keywords).
        Msg("Extracted keywords for search")

    // Search using keywords
    searchQuery := strings.Join(keywords, " ")

    searchOpts := &outline.SearchOptions{
        Limit: limit,
    }

    results, err := s.outlineClient.SearchDocuments(ctx, searchQuery, searchOpts)
    if err != nil {
        return nil, fmt.Errorf("search failed: %w", err)
    }

    log.Debug().
        Int("results_count", len(results.Documents)).
        Msg("Search completed")

    return results.Documents, nil
}
```

## Deduplication

### Question Hashing

```go
package qna

import (
    "crypto/sha256"
    "fmt"
    "strings"
)

func NormalizeQuestion(question string) string {
    // Lowercase
    normalized := strings.ToLower(question)

    // Remove extra whitespace
    normalized = strings.TrimSpace(normalized)
    normalized = strings.Join(strings.Fields(normalized), " ")

    // Remove punctuation
    normalized = strings.Trim(normalized, "?.,!;:")

    return normalized
}

func GenerateQuestionHash(documentID, questionText string) string {
    // Normalize question to handle slight variations
    normalized := NormalizeQuestion(questionText)

    // Combine with document ID
    input := fmt.Sprintf("%s:%s", documentID, normalized)

    // Generate hash
    hash := sha256.Sum256([]byte(input))
    return fmt.Sprintf("%x", hash)
}
```

## Error Handling

### Retry Strategy

```go
package qna

type RetryConfig struct {
    MaxRetries   int
    BackoffBase  time.Duration
    BackoffMax   time.Duration
}

func (s *DefaultService) ProcessQuestionWithRetry(ctx context.Context, doc *outline.Document, questionText string, cfg RetryConfig) error {
    var lastErr error

    for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
        if attempt > 0 {
            backoff := time.Duration(attempt) * cfg.BackoffBase
            if backoff > cfg.BackoffMax {
                backoff = cfg.BackoffMax
            }

            log.Info().
                Int("attempt", attempt).
                Dur("backoff", backoff).
                Msg("Retrying question processing")

            select {
            case <-time.After(backoff):
            case <-ctx.Done():
                return ctx.Err()
            }
        }

        err := s.ProcessQuestion(ctx, doc, questionText)
        if err == nil {
            return nil
        }

        lastErr = err

        // Check if error is retryable
        if !isRetryableError(err) {
            return err
        }
    }

    return fmt.Errorf("max retries exceeded: %w", lastErr)
}

func isRetryableError(err error) bool {
    // Implement error classification
    // Network errors, timeouts, rate limits are retryable
    // Invalid questions, missing documents are not
    return true // Simplified
}
```

## Testing Strategy

### Unit Tests

```go
func TestDefaultService_ProcessQuestion(t *testing.T)
func TestDefaultService_IsAnswered(t *testing.T)
func TestDefaultService_BuildContext(t *testing.T)
func TestDefaultService_FormatAnswer(t *testing.T)
func TestKeywordExtractor_ExtractKeywords(t *testing.T)
func TestRelevanceSearcher_SearchRelevant(t *testing.T)
func TestNormalizeQuestion(t *testing.T)
func TestGenerateQuestionHash(t *testing.T)
```

### Mock Services

```go
type MockSearcher struct {
    SearchRelevantFunc func(ctx context.Context, query string, limit int) ([]*outline.Document, error)
}

func (m *MockSearcher) SearchRelevant(ctx context.Context, query string, limit int) ([]*outline.Document, error) {
    if m.SearchRelevantFunc != nil {
        return m.SearchRelevantFunc(ctx, query, limit)
    }
    return []*outline.Document{
        {ID: "doc1", Title: "Test Doc", Text: "Test content"},
    }, nil
}
```

## Performance Considerations

### For SOHO Deployment

- **Search results**: Limit to 5 documents
- **Excerpt size**: 500 characters per document
- **Total context**: ~2500 characters (fits in most AI contexts)
- **Deduplication**: In-memory hash lookup (< 1ms)
- **Comment posting**: ~200ms per comment

### Optimization Strategies

1. **Cache search results**: Brief cache for repeated questions
2. **Parallel document fetching**: Fetch context documents concurrently
3. **Smart excerpting**: Extract most relevant sections instead of truncation
4. **Question normalization**: Handle slight variations

## SOHO Deployment Considerations

### Simplifications for Homelab

1. **Simple keyword extraction**: No NLP libraries needed
2. **Sequential processing**: One question at a time
3. **SQLite deduplication**: No Redis needed
4. **Comment-based answers**: No inline editing complexity
5. **Basic search**: Uses Outline's built-in search

### Example Configuration

```yaml
qna:
  enabled: true
  max_context_documents: 5
  answer_method: "comment"
  excerpt_size: 500

  deduplication:
    enabled: true
    retention: 30d  # Keep answered questions for 30 days

  retry:
    max_retries: 3
    backoff_base: 30s
    backoff_max: 5m
```

## Integration Example

### Usage in Command Handler

```go
package handlers

func (h *AIQuestionHandler) Handle(ctx context.Context, doc *outline.Document, cmd *Command) error {
    // Extract question from command
    question := cmd.Arguments

    // Process through Q&A service
    if err := h.qnaService.ProcessQuestion(ctx, doc, question); err != nil {
        return fmt.Errorf("failed to process question: %w", err)
    }

    return nil
}
```

## Package Structure

```
internal/qna/
├── service.go          # Main Q&A service
├── searcher.go         # Document search
├── keywords.go         # Keyword extraction
├── deduplication.go    # Question hashing
├── formatting.go       # Answer formatting
└── qna_test.go         # Test suite
```

## Dependencies

- `github.com/yourusername/outline-ai/internal/ai` - AI client
- `github.com/yourusername/outline-ai/internal/outline` - Outline client
- `github.com/yourusername/outline-ai/internal/persistence` - State storage
- `github.com/rs/zerolog` - Logging

---

**Status:** Ready for implementation
**Complexity:** Medium
**Priority:** High (core Q&A functionality)
