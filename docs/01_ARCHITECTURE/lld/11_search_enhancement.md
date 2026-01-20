# Low-Level Design: Search Enhancement

**Domain:** Content Enhancement
**Status:** Design
**Last Updated:** 2026-01-19
**Target Deployment:** Homelab/SOHO

## Purpose

Improve document discoverability through AI-generated summaries, enhanced titles, and search terms with idempotent operations.

## Design Principles

1. **Idempotent Operations**: Multiple runs cleanly replace previous enhancements
2. **Hidden Markers**: HTML comments track AI-generated content
3. **User Ownership**: Respect user edits when markers removed
4. **Non-Invasive**: Don't break existing content
5. **SOHO Optimized**: Simple pattern matching, no complex parsing

## Enhancement Types

### Enhancement Categories

```go
package enhancement

type EnhancementType string

const (
    EnhancementSummary      EnhancementType = "summary"
    EnhancementTitle        EnhancementType = "title"
    EnhancementSearchTerms  EnhancementType = "search_terms"
)
```

## Domain Models

### Enhancement Request/Response

```go
package enhancement

import (
    "github.com/yourusername/outline-ai/internal/outline"
)

type EnhancementRequest struct {
    Document        *outline.Document
    EnhancementType EnhancementType
    Force           bool // Force regeneration even if exists
}

type EnhancementResult struct {
    Type            EnhancementType
    Applied         bool
    PreviousContent string
    NewContent      string
    Error           error
}
```

## Service Interface

### Enhancement Service

```go
package enhancement

import (
    "context"

    "github.com/yourusername/outline-ai/internal/outline"
)

type Service interface {
    // Generate and apply summary
    ApplySummary(ctx context.Context, doc *outline.Document, force bool) (*EnhancementResult, error)

    // Enhance document title
    EnhanceTitle(ctx context.Context, doc *outline.Document) (*EnhancementResult, error)

    // Generate and apply search terms
    ApplySearchTerms(ctx context.Context, doc *outline.Document, force bool) (*EnhancementResult, error)

    // Apply all enhancements
    ApplyAll(ctx context.Context, doc *outline.Document) ([]*EnhancementResult, error)
}
```

## Service Implementation

### Main Service

```go
package enhancement

import (
    "context"
    "fmt"

    "github.com/yourusername/outline-ai/internal/ai"
    "github.com/yourusername/outline-ai/internal/outline"
    "github.com/rs/zerolog/log"
)

type DefaultService struct {
    aiClient      ai.Client
    outlineClient outline.Client
    config        Config
}

type Config struct {
    RespectUserOwnership bool
    IdempotentUpdates    bool
}

func NewDefaultService(aiClient ai.Client, outlineClient outline.Client, cfg Config) *DefaultService {
    return &DefaultService{
        aiClient:      aiClient,
        outlineClient: outlineClient,
        config:        cfg,
    }
}

func (s *DefaultService) ApplySummary(ctx context.Context, doc *outline.Document, force bool) (*EnhancementResult, error) {
    result := &EnhancementResult{
        Type: EnhancementSummary,
    }

    // Check if summary exists and should be skipped
    if !force && s.hasSummary(doc.Text) {
        if s.config.RespectUserOwnership && !s.hasSummaryMarkers(doc.Text) {
            log.Info().
                Str("document_id", doc.ID).
                Msg("Summary exists without markers, respecting user ownership")
            return result, nil
        }
    }

    // Generate summary
    summaryReq := &ai.SummaryRequest{
        DocumentTitle:   doc.Title,
        DocumentContent: doc.Text,
    }

    summaryResp, err := s.aiClient.GenerateSummary(ctx, summaryReq)
    if err != nil {
        result.Error = fmt.Errorf("failed to generate summary: %w", err)
        return result, result.Error
    }

    // Apply summary to document
    updatedText, previous := s.applySummaryToText(doc.Text, summaryResp.Summary)

    // Update document
    updateReq := &outline.UpdateDocumentRequest{
        Text: updatedText,
        Done: true,
    }

    _, err = s.outlineClient.UpdateDocument(ctx, doc.ID, updateReq)
    if err != nil {
        result.Error = fmt.Errorf("failed to update document: %w", err)
        return result, result.Error
    }

    result.Applied = true
    result.PreviousContent = previous
    result.NewContent = summaryResp.Summary

    log.Info().
        Str("document_id", doc.ID).
        Bool("replaced_existing", previous != "").
        Msg("Summary applied successfully")

    return result, nil
}

func (s *DefaultService) EnhanceTitle(ctx context.Context, doc *outline.Document) (*EnhancementResult, error) {
    result := &EnhancementResult{
        Type: EnhancementTitle,
    }

    // Check if title needs enhancement
    if !s.needsTitleEnhancement(doc.Title) {
        log.Debug().
            Str("document_id", doc.ID).
            Str("title", doc.Title).
            Msg("Title does not need enhancement")
        return result, nil
    }

    // Generate enhanced title
    titleReq := &ai.TitleRequest{
        CurrentTitle:    doc.Title,
        DocumentContent: doc.Text,
    }

    titleResp, err := s.aiClient.EnhanceTitle(ctx, titleReq)
    if err != nil {
        result.Error = fmt.Errorf("failed to enhance title: %w", err)
        return result, result.Error
    }

    // Check confidence threshold
    if titleResp.Confidence < 0.7 {
        log.Info().
            Str("document_id", doc.ID).
            Float64("confidence", titleResp.Confidence).
            Msg("Title enhancement confidence too low, skipping")
        return result, nil
    }

    // Update document title
    updateReq := &outline.UpdateDocumentRequest{
        Title: titleResp.SuggestedTitle,
        Done:  true,
    }

    _, err = s.outlineClient.UpdateDocument(ctx, doc.ID, updateReq)
    if err != nil {
        result.Error = fmt.Errorf("failed to update title: %w", err)
        return result, result.Error
    }

    result.Applied = true
    result.PreviousContent = doc.Title
    result.NewContent = titleResp.SuggestedTitle

    log.Info().
        Str("document_id", doc.ID).
        Str("old_title", doc.Title).
        Str("new_title", titleResp.SuggestedTitle).
        Msg("Title enhanced successfully")

    return result, nil
}

func (s *DefaultService) ApplySearchTerms(ctx context.Context, doc *outline.Document, force bool) (*EnhancementResult, error) {
    result := &EnhancementResult{
        Type: EnhancementSearchTerms,
    }

    // Check if search terms exist and should be skipped
    if !force && s.hasSearchTerms(doc.Text) {
        if s.config.RespectUserOwnership && !s.hasSearchTermsMarkers(doc.Text) {
            log.Info().
                Str("document_id", doc.ID).
                Msg("Search terms exist without markers, respecting user ownership")
            return result, nil
        }
    }

    // Generate search terms
    searchReq := &ai.SearchTermsRequest{
        DocumentTitle:   doc.Title,
        DocumentContent: doc.Text,
    }

    searchResp, err := s.aiClient.GenerateSearchTerms(ctx, searchReq)
    if err != nil {
        result.Error = fmt.Errorf("failed to generate search terms: %w", err)
        return result, result.Error
    }

    // Apply search terms to document
    updatedText, previous := s.applySearchTermsToText(doc.Text, searchResp.SearchTerms)

    // Update document
    updateReq := &outline.UpdateDocumentRequest{
        Text: updatedText,
        Done: true,
    }

    _, err = s.outlineClient.UpdateDocument(ctx, doc.ID, updateReq)
    if err != nil {
        result.Error = fmt.Errorf("failed to update document: %w", err)
        return result, result.Error
    }

    result.Applied = true
    result.PreviousContent = previous
    result.NewContent = strings.Join(searchResp.SearchTerms, ", ")

    log.Info().
        Str("document_id", doc.ID).
        Int("terms_count", len(searchResp.SearchTerms)).
        Bool("replaced_existing", previous != "").
        Msg("Search terms applied successfully")

    return result, nil
}

func (s *DefaultService) ApplyAll(ctx context.Context, doc *outline.Document) ([]*EnhancementResult, error) {
    var results []*EnhancementResult

    // Apply summary
    summaryResult, err := s.ApplySummary(ctx, doc, false)
    results = append(results, summaryResult)
    if err != nil {
        log.Warn().Err(err).Msg("Failed to apply summary")
    }

    // Enhance title
    titleResult, err := s.EnhanceTitle(ctx, doc)
    results = append(results, titleResult)
    if err != nil {
        log.Warn().Err(err).Msg("Failed to enhance title")
    }

    // Apply search terms
    searchResult, err := s.ApplySearchTerms(ctx, doc, false)
    results = append(results, searchResult)
    if err != nil {
        log.Warn().Err(err).Msg("Failed to apply search terms")
    }

    return results, nil
}
```

## Idempotent Text Operations

### Summary Insertion

```go
package enhancement

import (
    "fmt"
    "regexp"
    "strings"
)

const (
    SummaryMarkerStart = "<!-- AI-SUMMARY-START -->"
    SummaryMarkerEnd   = "<!-- AI-SUMMARY-END -->"
)

func (s *DefaultService) applySummaryToText(text, summary string) (updatedText string, previousSummary string) {
    summaryBlock := fmt.Sprintf("%s\n> **Summary**: %s\n%s\n\n",
        SummaryMarkerStart, summary, SummaryMarkerEnd)

    // Check for existing markers
    if s.hasSummaryMarkers(text) {
        // Extract previous summary
        previousSummary = s.extractSummary(text)

        // Replace content between markers
        startIdx := strings.Index(text, SummaryMarkerStart)
        endIdx := strings.Index(text, SummaryMarkerEnd)
        if endIdx != -1 {
            endIdx += len(SummaryMarkerEnd)
            updatedText = text[:startIdx] + summaryBlock + text[endIdx:]
            return
        }
    }

    // Check for summary without markers (legacy format)
    if s.hasSummary(text) && !s.hasSummaryMarkers(text) {
        if s.config.RespectUserOwnership {
            // Don't replace if no markers (user has taken ownership)
            return text, ""
        }

        // Replace legacy summary
        summaryPattern := regexp.MustCompile(`(?m)^>\s*\*\*Summary\*\*:.*$`)
        updatedText = summaryPattern.ReplaceAllString(text, "")
        updatedText = summaryBlock + strings.TrimSpace(updatedText)
        return
    }

    // No existing summary - add at beginning
    updatedText = summaryBlock + text
    return
}

func (s *DefaultService) hasSummaryMarkers(text string) bool {
    return strings.Contains(text, SummaryMarkerStart) &&
        strings.Contains(text, SummaryMarkerEnd)
}

func (s *DefaultService) hasSummary(text string) bool {
    summaryPattern := regexp.MustCompile(`(?m)^>\s*\*\*Summary\*\*:`)
    return summaryPattern.MatchString(text)
}

func (s *DefaultService) extractSummary(text string) string {
    startIdx := strings.Index(text, SummaryMarkerStart)
    endIdx := strings.Index(text, SummaryMarkerEnd)

    if startIdx == -1 || endIdx == -1 || startIdx >= endIdx {
        return ""
    }

    summaryBlock := text[startIdx+len(SummaryMarkerStart) : endIdx]

    // Extract just the summary text (remove markdown formatting)
    summaryPattern := regexp.MustCompile(`>\s*\*\*Summary\*\*:\s*(.*)`)
    matches := summaryPattern.FindStringSubmatch(summaryBlock)
    if len(matches) > 1 {
        return strings.TrimSpace(matches[1])
    }

    return strings.TrimSpace(summaryBlock)
}
```

### Search Terms Insertion

```go
package enhancement

const (
    SearchTermsMarkerStart = "<!-- AI-SEARCH-TERMS-START -->"
    SearchTermsMarkerEnd   = "<!-- AI-SEARCH-TERMS-END -->"
)

func (s *DefaultService) applySearchTermsToText(text string, searchTerms []string) (updatedText string, previousTerms string) {
    termsText := strings.Join(searchTerms, ", ")
    termsBlock := fmt.Sprintf("\n\n---\n%s\n**Search Terms**: %s\n%s",
        SearchTermsMarkerStart, termsText, SearchTermsMarkerEnd)

    // Check for existing markers
    if s.hasSearchTermsMarkers(text) {
        // Extract previous terms
        previousTerms = s.extractSearchTerms(text)

        // Replace content between markers
        startIdx := strings.Index(text, SearchTermsMarkerStart)
        endIdx := strings.Index(text, SearchTermsMarkerEnd)
        if endIdx != -1 {
            endIdx += len(SearchTermsMarkerEnd)
            updatedText = text[:startIdx] + termsBlock + text[endIdx:]
            return
        }
    }

    // Check for search terms without markers (legacy format)
    if s.hasSearchTerms(text) && !s.hasSearchTermsMarkers(text) {
        if s.config.RespectUserOwnership {
            // Don't replace if no markers
            return text, ""
        }

        // Replace legacy search terms
        termsPattern := regexp.MustCompile(`(?m)^---\s*\n\*\*Search Terms\*\*:.*$`)
        updatedText = termsPattern.ReplaceAllString(text, "")
        updatedText = strings.TrimSpace(updatedText) + termsBlock
        return
    }

    // No existing search terms - append at end
    updatedText = strings.TrimSpace(text) + termsBlock
    return
}

func (s *DefaultService) hasSearchTermsMarkers(text string) bool {
    return strings.Contains(text, SearchTermsMarkerStart) &&
        strings.Contains(text, SearchTermsMarkerEnd)
}

func (s *DefaultService) hasSearchTerms(text string) bool {
    termsPattern := regexp.MustCompile(`(?m)\*\*Search Terms\*\*:`)
    return termsPattern.MatchString(text)
}

func (s *DefaultService) extractSearchTerms(text string) string {
    startIdx := strings.Index(text, SearchTermsMarkerStart)
    endIdx := strings.Index(text, SearchTermsMarkerEnd)

    if startIdx == -1 || endIdx == -1 || startIdx >= endIdx {
        return ""
    }

    termsBlock := text[startIdx+len(SearchTermsMarkerStart) : endIdx]

    // Extract terms
    termsPattern := regexp.MustCompile(`\*\*Search Terms\*\*:\s*(.*)`)
    matches := termsPattern.FindStringSubmatch(termsBlock)
    if len(matches) > 1 {
        return strings.TrimSpace(matches[1])
    }

    return strings.TrimSpace(termsBlock)
}
```

### Title Enhancement Logic

```go
package enhancement

func (s *DefaultService) needsTitleEnhancement(title string) bool {
    // List of vague titles that should be enhanced
    vagueTitles := []string{
        "untitled",
        "draft",
        "notes",
        "new document",
        "document",
        "temp",
        "test",
    }

    lowerTitle := strings.ToLower(strings.TrimSpace(title))

    for _, vague := range vagueTitles {
        if lowerTitle == vague || strings.HasPrefix(lowerTitle, vague+" ") {
            return true
        }
    }

    // Check if title is too short
    if len(title) < 5 {
        return true
    }

    // Check if title is just numbers or very generic
    if regexp.MustCompile(`^[\d\s]+$`).MatchString(title) {
        return true
    }

    return false
}
```

## Batch Enhancement

### Batch Processing

```go
package enhancement

type BatchProcessor struct {
    service       Service
    outlineClient outline.Client
}

func NewBatchProcessor(service Service, client outline.Client) *BatchProcessor {
    return &BatchProcessor{
        service:       service,
        outlineClient: client,
    }
}

func (p *BatchProcessor) EnhanceCollection(ctx context.Context, collectionID string) (*BatchResult, error) {
    result := &BatchResult{
        CollectionID: collectionID,
    }

    // List documents in collection
    docs, err := p.outlineClient.ListDocuments(ctx, collectionID)
    if err != nil {
        return nil, fmt.Errorf("failed to list documents: %w", err)
    }

    log.Info().
        Str("collection_id", collectionID).
        Int("document_count", len(docs)).
        Msg("Starting batch enhancement")

    // Process each document
    for _, doc := range docs {
        // Fetch full document
        fullDoc, err := p.outlineClient.GetDocument(ctx, doc.ID)
        if err != nil {
            log.Warn().
                Err(err).
                Str("document_id", doc.ID).
                Msg("Failed to fetch document")
            result.Failed++
            continue
        }

        // Apply all enhancements
        enhancements, err := p.service.ApplyAll(ctx, fullDoc)
        if err != nil {
            log.Warn().
                Err(err).
                Str("document_id", doc.ID).
                Msg("Failed to enhance document")
            result.Failed++
            continue
        }

        // Count successful enhancements
        enhanced := false
        for _, enh := range enhancements {
            if enh.Applied {
                enhanced = true
                break
            }
        }

        if enhanced {
            result.Enhanced++
        } else {
            result.Skipped++
        }
    }

    log.Info().
        Str("collection_id", collectionID).
        Int("enhanced", result.Enhanced).
        Int("skipped", result.Skipped).
        Int("failed", result.Failed).
        Msg("Batch enhancement completed")

    return result, nil
}

type BatchResult struct {
    CollectionID string
    Enhanced     int
    Skipped      int
    Failed       int
}
```

## Testing Strategy

### Unit Tests

```go
func TestDefaultService_ApplySummary(t *testing.T)
func TestDefaultService_ApplySummary_Idempotent(t *testing.T)
func TestDefaultService_EnhanceTitle(t *testing.T)
func TestDefaultService_ApplySearchTerms(t *testing.T)
func TestDefaultService_ApplySearchTerms_Idempotent(t *testing.T)
func TestApplySummaryToText_NewSummary(t *testing.T)
func TestApplySummaryToText_ReplaceSummary(t *testing.T)
func TestApplySummaryToText_RespectUserOwnership(t *testing.T)
func TestApplySearchTermsToText_NewTerms(t *testing.T)
func TestApplySearchTermsToText_ReplaceTerms(t *testing.T)
func TestNeedsTitleEnhancement(t *testing.T)
```

### Test Cases

```go
func TestSummaryIdempotency(t *testing.T) {
    service := setupTestService(t)

    doc := &outline.Document{
        ID:    "doc1",
        Title: "Test Document",
        Text:  "Original content here",
    }

    // First application
    result1, err := service.ApplySummary(context.Background(), doc, false)
    if err != nil {
        t.Fatalf("First application failed: %v", err)
    }

    // Simulate document fetch with updated text
    doc.Text = result1.NewContent

    // Second application (should replace cleanly)
    result2, err := service.ApplySummary(context.Background(), doc, true)
    if err != nil {
        t.Fatalf("Second application failed: %v", err)
    }

    // Verify only one summary block exists
    summaryCount := strings.Count(result2.NewContent, SummaryMarkerStart)
    if summaryCount != 1 {
        t.Errorf("Expected 1 summary block, got %d", summaryCount)
    }
}
```

## SOHO Deployment Considerations

### Simplifications for Homelab

1. **Simple regex patterns**: No complex parsing
2. **Sequential enhancement**: One document at a time
3. **No versioning**: Don't track enhancement history
4. **Fixed markers**: No customizable marker formats
5. **Basic detection**: Simple pattern matching for vague titles

### Example Configuration

```yaml
enhancement:
  enabled: true
  enhance_titles: true
  add_summaries: true
  idempotent_updates: true
  respect_user_ownership: true

  title_enhancement:
    confidence_threshold: 0.7
    vague_titles: ["untitled", "draft", "notes"]

  summary:
    use_markers: true
    respect_no_markers: true
    detect_existing_format: true

  search_terms:
    use_markers: true
    respect_no_markers: true
    min_terms: 3
    max_terms: 10
```

## Package Structure

```
internal/enhancement/
├── service.go          # Main enhancement service
├── summary.go          # Summary operations
├── title.go            # Title enhancement
├── search_terms.go     # Search terms operations
├── batch.go            # Batch processing
└── enhancement_test.go # Test suite
```

## Dependencies

- `github.com/yourusername/outline-ai/internal/ai` - AI client
- `github.com/yourusername/outline-ai/internal/outline` - Outline client
- `github.com/rs/zerolog` - Logging
- Standard library `regexp`, `strings`

---

**Status:** Ready for implementation
**Complexity:** Medium
**Priority:** Medium (optional enhancement feature)
