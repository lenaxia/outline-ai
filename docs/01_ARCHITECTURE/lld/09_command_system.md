# Low-Level Design: Command System

**Domain:** Command Detection and Routing
**Status:** Design
**Last Updated:** 2026-01-19
**Target Deployment:** Homelab/SOHO

## Purpose

Detect command markers in documents, parse command arguments, and route to appropriate handlers for execution.

## Design Principles

1. **Extensible**: Easy to add new commands
2. **Type-Safe**: Strongly-typed command parsing
3. **Command Cleanup**: Remove markers after execution
4. **Error Feedback**: Provide clear feedback via comments
5. **SOHO Optimized**: Simple pattern matching, no complex parsing

## Command Structure

### Command Models

```go
package command

import (
    "github.com/yourusername/outline-ai/internal/outline"
)

type Command struct {
    Type      CommandType
    Arguments string
    LineRange *LineRange // For comment placement
    RawText   string
}

type CommandType string

const (
    CommandAI           CommandType = "/ai"           // Question answering
    CommandAIFile       CommandType = "/ai-file"      // Document filing
    CommandAIFileUncertain CommandType = "?ai-file"   // Uncertain filing marker
    CommandSummarize    CommandType = "/summarize"    // Generate summary
    CommandEnhanceTitle CommandType = "/enhance-title" // Improve title
    CommandRelated      CommandType = "/related"      // Find related docs
)

type LineRange struct {
    Start int
    End   int
}
```

## Detector Interface

### Command Detection

```go
package command

import (
    "context"

    "github.com/yourusername/outline-ai/internal/outline"
)

type Detector interface {
    // Detect all commands in document
    DetectCommands(ctx context.Context, doc *outline.Document) ([]*Command, error)

    // Check if document has specific command
    HasCommand(ctx context.Context, doc *outline.Document, cmdType CommandType) bool
}

type Processor interface {
    // Process all commands in document
    ProcessDocument(ctx context.Context, documentID string) error

    // Process specific command
    ProcessCommand(ctx context.Context, doc *outline.Document, cmd *Command) error
}
```

## Detector Implementation

### Pattern-Based Detector

```go
package command

import (
    "context"
    "fmt"
    "regexp"
    "strings"

    "github.com/rs/zerolog/log"
)

type RegexDetector struct {
    patterns map[CommandType]*regexp.Regexp
}

func NewRegexDetector() *RegexDetector {
    return &RegexDetector{
        patterns: map[CommandType]*regexp.Regexp{
            CommandAI:              regexp.MustCompile(`(?m)^/ai\s+(.+)$`),
            CommandAIFile:          regexp.MustCompile(`(?m)^/ai-file(?:\s+(.*))?$`),
            CommandAIFileUncertain: regexp.MustCompile(`(?m)^\?ai-file(?:\s+(.*))?$`),
            CommandSummarize:       regexp.MustCompile(`(?m)^/summarize\s*$`),
            CommandEnhanceTitle:    regexp.MustCompile(`(?m)^/enhance-title\s*$`),
            CommandRelated:         regexp.MustCompile(`(?m)^/related\s*$`),
        },
    }
}

func (d *RegexDetector) DetectCommands(ctx context.Context, doc *outline.Document) ([]*Command, error) {
    var commands []*Command

    for cmdType, pattern := range d.patterns {
        matches := pattern.FindAllStringSubmatchIndex(doc.Text, -1)

        for _, match := range matches {
            cmd := &Command{
                Type:      cmdType,
                RawText:   doc.Text[match[0]:match[1]],
                LineRange: d.calculateLineRange(doc.Text, match[0], match[1]),
            }

            // Extract arguments if present
            if len(match) > 2 && match[2] != -1 {
                cmd.Arguments = strings.TrimSpace(doc.Text[match[2]:match[3]])
            }

            commands = append(commands, cmd)

            log.Debug().
                Str("command", string(cmdType)).
                Str("arguments", cmd.Arguments).
                Str("document_id", doc.ID).
                Msg("Command detected")
        }
    }

    return commands, nil
}

func (d *RegexDetector) HasCommand(ctx context.Context, doc *outline.Document, cmdType CommandType) bool {
    pattern, exists := d.patterns[cmdType]
    if !exists {
        return false
    }

    return pattern.MatchString(doc.Text)
}

func (d *RegexDetector) calculateLineRange(text string, start, end int) *LineRange {
    // Count newlines before start position
    startLine := strings.Count(text[:start], "\n")
    endLine := strings.Count(text[:end], "\n")

    return &LineRange{
        Start: startLine,
        End:   endLine,
    }
}
```

## Command Router

### Handler Registry

```go
package command

import (
    "context"
    "fmt"
    "sync"

    "github.com/yourusername/outline-ai/internal/outline"
)

type Handler interface {
    Handle(ctx context.Context, doc *outline.Document, cmd *Command) error
    GetCommandType() CommandType
}

type Router struct {
    handlers map[CommandType]Handler
    mu       sync.RWMutex
}

func NewRouter() *Router {
    return &Router{
        handlers: make(map[CommandType]Handler),
    }
}

func (r *Router) RegisterHandler(handler Handler) {
    r.mu.Lock()
    defer r.mu.Unlock()

    cmdType := handler.GetCommandType()
    r.handlers[cmdType] = handler

    log.Info().
        Str("command_type", string(cmdType)).
        Msg("Command handler registered")
}

func (r *Router) Route(ctx context.Context, doc *outline.Document, cmd *Command) error {
    r.mu.RLock()
    handler, exists := r.handlers[cmd.Type]
    r.mu.RUnlock()

    if !exists {
        return fmt.Errorf("no handler registered for command: %s", cmd.Type)
    }

    log.Info().
        Str("command_type", string(cmd.Type)).
        Str("document_id", doc.ID).
        Msg("Routing command to handler")

    return handler.Handle(ctx, doc, cmd)
}
```

## Command Processor

### Main Processor

```go
package command

import (
    "context"
    "fmt"

    "github.com/yourusername/outline-ai/internal/outline"
    "github.com/rs/zerolog/log"
)

type DefaultProcessor struct {
    detector      Detector
    router        *Router
    outlineClient outline.Client
}

func NewDefaultProcessor(detector Detector, router *Router, client outline.Client) *DefaultProcessor {
    return &DefaultProcessor{
        detector:      detector,
        router:        router,
        outlineClient: client,
    }
}

func (p *DefaultProcessor) ProcessDocument(ctx context.Context, documentID string) error {
    // Fetch document
    doc, err := p.outlineClient.GetDocument(ctx, documentID)
    if err != nil {
        return fmt.Errorf("failed to fetch document: %w", err)
    }

    // Detect commands
    commands, err := p.detector.DetectCommands(ctx, doc)
    if err != nil {
        return fmt.Errorf("failed to detect commands: %w", err)
    }

    if len(commands) == 0 {
        log.Debug().
            Str("document_id", documentID).
            Msg("No commands found in document")
        return nil
    }

    log.Info().
        Str("document_id", documentID).
        Int("commands_found", len(commands)).
        Msg("Processing commands in document")

    // Process each command
    for _, cmd := range commands {
        if err := p.ProcessCommand(ctx, doc, cmd); err != nil {
            log.Error().
                Err(err).
                Str("command_type", string(cmd.Type)).
                Str("document_id", documentID).
                Msg("Command processing failed")

            // Continue processing other commands
            continue
        }
    }

    return nil
}

func (p *DefaultProcessor) ProcessCommand(ctx context.Context, doc *outline.Document, cmd *Command) error {
    // Route to handler
    if err := p.router.Route(ctx, doc, cmd); err != nil {
        return fmt.Errorf("command routing failed: %w", err)
    }

    // Remove command marker from document (cleanup)
    if err := p.removeCommandMarker(ctx, doc, cmd); err != nil {
        log.Warn().
            Err(err).
            Str("command_type", string(cmd.Type)).
            Str("document_id", doc.ID).
            Msg("Failed to remove command marker")
        // Don't fail the command if marker removal fails
    }

    return nil
}

func (p *DefaultProcessor) removeCommandMarker(ctx context.Context, doc *outline.Document, cmd *Command) error {
    // Replace command marker with empty string
    updatedText := strings.Replace(doc.Text, cmd.RawText, "", 1)

    // Also remove ?ai-file if processing /ai-file (dual marker cleanup)
    if cmd.Type == CommandAIFile {
        uncertainPattern := regexp.MustCompile(`(?m)^\?ai-file(?:\s+.*)?$`)
        updatedText = uncertainPattern.ReplaceAllString(updatedText, "")
    }

    // Update document
    updateReq := &outline.UpdateDocumentRequest{
        Text: updatedText,
        Done: true,
    }

    _, err := p.outlineClient.UpdateDocument(ctx, doc.ID, updateReq)
    if err != nil {
        return fmt.Errorf("failed to update document: %w", err)
    }

    log.Debug().
        Str("command_type", string(cmd.Type)).
        Str("document_id", doc.ID).
        Msg("Command marker removed")

    return nil
}
```

## Command Handlers

### AI Question Handler

```go
package command

import (
    "context"
    "fmt"

    "github.com/yourusername/outline-ai/internal/ai"
    "github.com/yourusername/outline-ai/internal/outline"
)

type AIQuestionHandler struct {
    aiClient      ai.Client
    outlineClient outline.Client
    searcher      DocumentSearcher
}

type DocumentSearcher interface {
    SearchRelevant(ctx context.Context, query string, limit int) ([]*outline.Document, error)
}

func NewAIQuestionHandler(aiClient ai.Client, outlineClient outline.Client, searcher DocumentSearcher) *AIQuestionHandler {
    return &AIQuestionHandler{
        aiClient:      aiClient,
        outlineClient: outlineClient,
        searcher:      searcher,
    }
}

func (h *AIQuestionHandler) GetCommandType() CommandType {
    return CommandAI
}

func (h *AIQuestionHandler) Handle(ctx context.Context, doc *outline.Document, cmd *Command) error {
    if cmd.Arguments == "" {
        return fmt.Errorf("question text is required for /ai command")
    }

    question := cmd.Arguments

    log.Info().
        Str("question", question).
        Str("document_id", doc.ID).
        Msg("Processing question")

    // Search for relevant documents
    relevantDocs, err := h.searcher.SearchRelevant(ctx, question, 5)
    if err != nil {
        return fmt.Errorf("failed to search relevant documents: %w", err)
    }

    // Build context for AI
    contextDocs := make([]ai.ContextDocument, len(relevantDocs))
    for i, d := range relevantDocs {
        excerpt := d.Text
        if len(excerpt) > 500 {
            excerpt = excerpt[:500] + "..."
        }

        contextDocs[i] = ai.ContextDocument{
            Title:   d.Title,
            Excerpt: excerpt,
            URL:     fmt.Sprintf("outline://doc/%s", d.ID),
        }
    }

    // Ask AI
    questionReq := &ai.QuestionRequest{
        Question:    question,
        ContextDocs: contextDocs,
    }

    answer, err := h.aiClient.AnswerQuestion(ctx, questionReq)
    if err != nil {
        return fmt.Errorf("failed to get answer from AI: %w", err)
    }

    // Post answer as comment
    commentReq := &outline.CreateCommentRequest{
        DocumentID: doc.ID,
        Data:       outline.NewCommentContent(answer.Answer),
    }

    _, err = h.outlineClient.CreateComment(ctx, commentReq)
    if err != nil {
        return fmt.Errorf("failed to create comment: %w", err)
    }

    log.Info().
        Str("question", question).
        Str("document_id", doc.ID).
        Float64("confidence", answer.Confidence).
        Msg("Question answered successfully")

    return nil
}
```

### Filing Handler

```go
package command

type AIFileHandler struct {
    aiClient        ai.Client
    outlineClient   outline.Client
    taxonomyBuilder TaxonomyBuilder
    confidenceThreshold float64
}

type TaxonomyBuilder interface {
    GetTaxonomy(ctx context.Context) (*taxonomy.Taxonomy, error)
}

func NewAIFileHandler(
    aiClient ai.Client,
    outlineClient outline.Client,
    taxonomyBuilder TaxonomyBuilder,
    threshold float64,
) *AIFileHandler {
    return &AIFileHandler{
        aiClient:        aiClient,
        outlineClient:   outlineClient,
        taxonomyBuilder: taxonomyBuilder,
        confidenceThreshold: threshold,
    }
}

func (h *AIFileHandler) GetCommandType() CommandType {
    return CommandAIFile
}

func (h *AIFileHandler) Handle(ctx context.Context, doc *outline.Document, cmd *Command) error {
    // Get taxonomy
    tax, err := h.taxonomyBuilder.GetTaxonomy(ctx)
    if err != nil {
        return fmt.Errorf("failed to get taxonomy: %w", err)
    }

    // Classify document
    classReq := &ai.ClassificationRequest{
        DocumentTitle:   doc.Title,
        DocumentContent: doc.Text,
        UserGuidance:    cmd.Arguments,
        Taxonomy:        tax.ToAIContext(),
    }

    classResp, err := h.aiClient.ClassifyDocument(ctx, classReq)
    if err != nil {
        return fmt.Errorf("classification failed: %w", err)
    }

    // Check confidence
    if classResp.Confidence < h.confidenceThreshold {
        return h.handleLowConfidence(ctx, doc, classResp)
    }

    // High confidence - proceed with filing
    return h.handleHighConfidence(ctx, doc, classResp)
}

func (h *AIFileHandler) handleHighConfidence(ctx context.Context, doc *outline.Document, classResp *ai.ClassificationResponse) error {
    // Move document
    err := h.outlineClient.MoveDocument(ctx, doc.ID, classResp.CollectionID)
    if err != nil {
        return fmt.Errorf("failed to move document: %w", err)
    }

    // Add search terms
    if len(classResp.SearchTerms) > 0 {
        searchTermsText := fmt.Sprintf("\n\n---\n<!-- AI-SEARCH-TERMS-START -->\n**Search Terms**: %s\n<!-- AI-SEARCH-TERMS-END -->",
            strings.Join(classResp.SearchTerms, ", "))

        updatedText := doc.Text + searchTermsText
        updateReq := &outline.UpdateDocumentRequest{
            Text: updatedText,
            Done: true,
        }

        _, err = h.outlineClient.UpdateDocument(ctx, doc.ID, updateReq)
        if err != nil {
            log.Warn().Err(err).Msg("Failed to add search terms")
        }
    }

    // Add success comment
    comment := fmt.Sprintf("✓ Filed to collection (confidence: %.0f%%)\n\nReasoning: %s",
        classResp.Confidence*100, classResp.Reasoning)

    commentReq := &outline.CreateCommentRequest{
        DocumentID: doc.ID,
        Data:       outline.NewCommentContent(comment),
    }

    h.outlineClient.CreateComment(ctx, commentReq)

    return nil
}

func (h *AIFileHandler) handleLowConfidence(ctx context.Context, doc *outline.Document, classResp *ai.ClassificationResponse) error {
    // Convert /ai-file to ?ai-file
    updatedText := strings.Replace(doc.Text, "/ai-file", "?ai-file", 1)

    updateReq := &outline.UpdateDocumentRequest{
        Text: updatedText,
        Done: true,
    }

    _, err := h.outlineClient.UpdateDocument(ctx, doc.ID, updateReq)
    if err != nil {
        return fmt.Errorf("failed to update marker: %w", err)
    }

    // Add uncertainty comment
    var alternativesText string
    for _, alt := range classResp.Alternatives {
        alternativesText += fmt.Sprintf("\n- %s (%.0f%%) - %s",
            alt.CollectionID, alt.Confidence*100, alt.Reasoning)
    }

    comment := fmt.Sprintf("⚠️ Unable to file with confidence (%.0f%%)\n\nUncertain between:%s\n\nTo help me decide, update the ?ai-file line with guidance.",
        classResp.Confidence*100, alternativesText)

    commentReq := &outline.CreateCommentRequest{
        DocumentID: doc.ID,
        Data:       outline.NewCommentContent(comment),
    }

    h.outlineClient.CreateComment(ctx, commentReq)

    return nil
}
```

### Summarize Handler

```go
package command

type SummarizeHandler struct {
    aiClient      ai.Client
    outlineClient outline.Client
}

func NewSummarizeHandler(aiClient ai.Client, outlineClient outline.Client) *SummarizeHandler {
    return &SummarizeHandler{
        aiClient:      aiClient,
        outlineClient: outlineClient,
    }
}

func (h *SummarizeHandler) GetCommandType() CommandType {
    return CommandSummarize
}

func (h *SummarizeHandler) Handle(ctx context.Context, doc *outline.Document, cmd *Command) error {
    // Generate summary
    summaryReq := &ai.SummaryRequest{
        DocumentTitle:   doc.Title,
        DocumentContent: doc.Text,
    }

    summaryResp, err := h.aiClient.GenerateSummary(ctx, summaryReq)
    if err != nil {
        return fmt.Errorf("failed to generate summary: %w", err)
    }

    // Insert or replace summary
    updatedText := h.insertSummary(doc.Text, summaryResp.Summary)

    updateReq := &outline.UpdateDocumentRequest{
        Text: updatedText,
        Done: true,
    }

    _, err = h.outlineClient.UpdateDocument(ctx, doc.ID, updateReq)
    if err != nil {
        return fmt.Errorf("failed to update document: %w", err)
    }

    log.Info().
        Str("document_id", doc.ID).
        Msg("Summary added successfully")

    return nil
}

func (h *SummarizeHandler) insertSummary(text, summary string) string {
    markerStart := "<!-- AI-SUMMARY-START -->"
    markerEnd := "<!-- AI-SUMMARY-END -->"

    summaryBlock := fmt.Sprintf("%s\n> **Summary**: %s\n%s\n\n", markerStart, summary, markerEnd)

    // Check if markers exist
    if strings.Contains(text, markerStart) {
        // Replace existing summary
        startIdx := strings.Index(text, markerStart)
        endIdx := strings.Index(text, markerEnd)
        if endIdx != -1 {
            endIdx += len(markerEnd)
            return text[:startIdx] + summaryBlock + text[endIdx:]
        }
    }

    // No markers - add at beginning
    return summaryBlock + text
}
```

## Command Failure Recovery

### Overview

The command system implements comprehensive failure recovery to handle various error scenarios while maintaining document integrity and providing clear feedback to users.

### Command Marker Cleanup on Failure

When command processing fails, the system must decide whether to leave, modify, or remove command markers.

```go
package command

type CleanupStrategy int

const (
    CleanupLeaveMarker    CleanupStrategy = iota  // Leave marker for retry
    CleanupConvertMarker                          // Convert to question marker (?ai-file)
    CleanupRemoveMarker                           // Remove marker completely
    CleanupAddFailureNote                         // Add failure note, keep marker
)

type FailureHandler struct {
    outlineClient outline.Client
    storage       persistence.Storage
}

func (fh *FailureHandler) HandleCommandFailure(
    ctx context.Context,
    doc *outline.Document,
    cmd *Command,
    err error,
    attemptCount int,
) error {
    strategy := fh.determineCleanupStrategy(cmd, err, attemptCount)

    log.Warn().
        Str("command_type", string(cmd.Type)).
        Str("document_id", doc.ID).
        Err(err).
        Int("attempt", attemptCount).
        Str("strategy", fmt.Sprintf("%v", strategy)).
        Msg("Handling command failure")

    switch strategy {
    case CleanupLeaveMarker:
        return fh.leaveMarkerForRetry(ctx, doc, cmd, err)

    case CleanupConvertMarker:
        return fh.convertToUncertainMarker(ctx, doc, cmd, err)

    case CleanupRemoveMarker:
        return fh.removeMarker(ctx, doc, cmd, err)

    case CleanupAddFailureNote:
        return fh.addFailureNote(ctx, doc, cmd, err)
    }

    return nil
}

func (fh *FailureHandler) determineCleanupStrategy(
    cmd *Command,
    err error,
    attemptCount int,
) CleanupStrategy {
    // Permanent errors - remove marker to prevent infinite retries
    if worker.IsPermanentError(err) {
        return CleanupRemoveMarker
    }

    // For /ai-file command with AI low confidence
    if cmd.Type == CommandAIFile {
        if aiErr, ok := err.(*ai.LowConfidenceError); ok {
            return CleanupConvertMarker
        }
    }

    // Transient errors - leave marker for automatic retry
    if worker.IsTransientError(err) && attemptCount < 3 {
        return CleanupLeaveMarker
    }

    // Max retries exceeded - add failure note
    if attemptCount >= 3 {
        return CleanupAddFailureNote
    }

    // Default: leave marker
    return CleanupLeaveMarker
}

func (fh *FailureHandler) leaveMarkerForRetry(
    ctx context.Context,
    doc *outline.Document,
    cmd *Command,
    err error,
) error {
    // Post comment explaining temporary failure
    comment := fmt.Sprintf("⏳ Command temporarily failed (will retry automatically)\n\n"+
        "Command: %s\nError: %s\n\n"+
        "The system will retry this command automatically. If this persists, please contact support.",
        cmd.Type, err.Error())

    commentReq := &outline.CreateCommentRequest{
        DocumentID: doc.ID,
        Data:       outline.NewCommentContent(comment),
    }

    if _, err := fh.outlineClient.CreateComment(ctx, commentReq); err != nil {
        log.Warn().Err(err).Msg("Failed to post retry comment")
    }

    return nil
}

func (fh *FailureHandler) convertToUncertainMarker(
    ctx context.Context,
    doc *outline.Document,
    cmd *Command,
    err error,
) error {
    // Convert /ai-file to ?ai-file
    updatedText := strings.Replace(doc.Text, "/ai-file", "?ai-file", 1)

    updateReq := &outline.UpdateDocumentRequest{
        Text: updatedText,
        Done: true,
    }

    if _, err := fh.outlineClient.UpdateDocument(ctx, doc.ID, updateReq); err != nil {
        return fmt.Errorf("failed to convert marker: %w", err)
    }

    // Add uncertainty comment
    if aiErr, ok := err.(*ai.LowConfidenceError); ok {
        comment := fh.formatUncertaintyComment(aiErr)

        commentReq := &outline.CreateCommentRequest{
            DocumentID: doc.ID,
            Data:       outline.NewCommentContent(comment),
        }

        fh.outlineClient.CreateComment(ctx, commentReq)
    }

    return nil
}

func (fh *FailureHandler) removeMarker(
    ctx context.Context,
    doc *outline.Document,
    cmd *Command,
    err error,
) error {
    // Remove command marker
    updatedText := strings.Replace(doc.Text, cmd.RawText, "", 1)

    updateReq := &outline.UpdateDocumentRequest{
        Text: updatedText,
        Done: true,
    }

    if _, err := fh.outlineClient.UpdateDocument(ctx, doc.ID, updateReq); err != nil {
        return fmt.Errorf("failed to remove marker: %w", err)
    }

    // Post failure comment
    comment := fmt.Sprintf("❌ Command failed permanently\n\n"+
        "Command: %s\nError: %s\n\n"+
        "The command marker has been removed to prevent retries. "+
        "Please review the error and try again manually if needed.",
        cmd.Type, err.Error())

    commentReq := &outline.CreateCommentRequest{
        DocumentID: doc.ID,
        Data:       outline.NewCommentContent(comment),
    }

    fh.outlineClient.CreateComment(ctx, commentReq)

    return nil
}

func (fh *FailureHandler) addFailureNote(
    ctx context.Context,
    doc *outline.Document,
    cmd *Command,
    err error,
) error {
    comment := fmt.Sprintf("❌ Command failed after multiple attempts\n\n"+
        "Command: %s\nError: %s\n\n"+
        "The command marker is still in the document. "+
        "To retry: Remove and re-add the command, or contact support.",
        cmd.Type, err.Error())

    commentReq := &outline.CreateCommentRequest{
        DocumentID: doc.ID,
        Data:       outline.NewCommentContent(comment),
    }

    if _, err := fh.outlineClient.CreateComment(ctx, commentReq); err != nil {
        log.Warn().Err(err).Msg("Failed to post failure comment")
    }

    return nil
}

func (fh *FailureHandler) formatUncertaintyComment(aiErr *ai.LowConfidenceError) string {
    var alternatives string
    for _, alt := range aiErr.Alternatives {
        alternatives += fmt.Sprintf("\n- %s (%.0f%%) - %s",
            alt.CollectionName, alt.Confidence*100, alt.Reasoning)
    }

    return fmt.Sprintf("⚠️ Unable to file with confidence (%.0f%%)\n\n"+
        "Uncertain between:%s\n\n"+
        "To help me decide, update the ?ai-file line with guidance:\n"+
        "Example: ?ai-file engineering documentation\n"+
        "Example: ?ai-file customer-facing content",
        aiErr.Confidence*100, alternatives)
}
```

### Dual Marker Edge Cases

Handle scenarios where both `/ai-file` and `?ai-file` exist simultaneously.

```go
package command

type DualMarkerHandler struct {
    detector      Detector
    outlineClient outline.Client
}

func (h *DualMarkerHandler) HandleDualMarkers(
    ctx context.Context,
    doc *outline.Document,
) error {
    hasActiveMarker := h.detector.HasCommand(ctx, doc, CommandAIFile)
    hasUncertainMarker := h.detector.HasCommand(ctx, doc, CommandAIFileUncertain)

    if !hasActiveMarker || !hasUncertainMarker {
        // No dual marker situation
        return nil
    }

    log.Info().
        Str("document_id", doc.ID).
        Msg("Detected dual markers (/ai-file and ?ai-file)")

    // Strategy: Process the /ai-file (user has added guidance)
    // On success, remove BOTH markers
    // On failure, keep ?ai-file, remove /ai-file

    return nil // Handled by command processor
}

// Enhanced removeCommandMarker with dual marker cleanup
func (p *DefaultProcessor) removeCommandMarkerEnhanced(
    ctx context.Context,
    doc *outline.Document,
    cmd *Command,
) error {
    updatedText := doc.Text

    // Remove primary command marker
    updatedText = strings.Replace(updatedText, cmd.RawText, "", 1)

    // For /ai-file command, also remove ?ai-file if it exists
    if cmd.Type == CommandAIFile {
        uncertainPattern := regexp.MustCompile(`(?m)^\?ai-file(?:\s+.*)?$`)
        updatedText = uncertainPattern.ReplaceAllString(updatedText, "")

        log.Debug().
            Str("document_id", doc.ID).
            Msg("Removed both /ai-file and ?ai-file markers")
    }

    // Clean up multiple consecutive blank lines
    updatedText = regexp.MustCompile(`\n{3,}`).ReplaceAllString(updatedText, "\n\n")

    updateReq := &outline.UpdateDocumentRequest{
        Text: updatedText,
        Done: true,
    }

    _, err := p.outlineClient.UpdateDocument(ctx, doc.ID, updateReq)
    if err != nil {
        return fmt.Errorf("failed to update document: %w", err)
    }

    return nil
}
```

### Comment Posting Failures

Handle scenarios where comment creation fails.

```go
package command

type CommentPoster struct {
    outlineClient outline.Client
    maxRetries    int
    retryBackoff  time.Duration
}

func (cp *CommentPoster) PostCommentWithRetry(
    ctx context.Context,
    documentID string,
    content string,
) error {
    var lastErr error

    for attempt := 0; attempt < cp.maxRetries; attempt++ {
        if attempt > 0 {
            backoff := time.Duration(attempt) * cp.retryBackoff

            log.Info().
                Str("document_id", documentID).
                Int("attempt", attempt).
                Dur("backoff", backoff).
                Msg("Retrying comment creation")

            select {
            case <-time.After(backoff):
            case <-ctx.Done():
                return ctx.Err()
            }
        }

        commentReq := &outline.CreateCommentRequest{
            DocumentID: documentID,
            Data:       outline.NewCommentContent(content),
        }

        _, err := cp.outlineClient.CreateComment(ctx, commentReq)
        if err == nil {
            return nil
        }

        lastErr = err

        // Don't retry on permanent errors
        if worker.IsPermanentError(err) {
            log.Error().
                Err(err).
                Str("document_id", documentID).
                Msg("Comment posting failed with permanent error")
            return err
        }
    }

    log.Warn().
        Err(lastErr).
        Str("document_id", documentID).
        Msg("Comment posting failed after retries - continuing without comment")

    // Don't fail the entire command if comment posting fails
    return nil
}

// Alternative: Store failed comments for later retry
type FailedComment struct {
    DocumentID string
    Content    string
    Timestamp  time.Time
    Retries    int
}

func (cp *CommentPoster) StoreFailedComment(
    ctx context.Context,
    documentID, content string,
) error {
    // Store in database for background retry
    failedComment := &FailedComment{
        DocumentID: documentID,
        Content:    content,
        Timestamp:  time.Now(),
        Retries:    0,
    }

    // Implementation: Store in persistence layer
    log.Info().
        Str("document_id", documentID).
        Msg("Stored failed comment for background retry")

    return nil
}
```

### Rollback Strategies

Implement rollback for multi-step command operations.

```go
package command

type RollbackableCommand struct {
    steps      []CommandStep
    executed   []CommandStep
    doc        *outline.Document
    client     outline.Client
}

type CommandStep interface {
    Execute(ctx context.Context) error
    Rollback(ctx context.Context) error
    GetDescription() string
}

func (rc *RollbackableCommand) Execute(ctx context.Context) error {
    for _, step := range rc.steps {
        log.Info().
            Str("step", step.GetDescription()).
            Str("document_id", rc.doc.ID).
            Msg("Executing command step")

        if err := step.Execute(ctx); err != nil {
            log.Error().
                Err(err).
                Str("step", step.GetDescription()).
                Msg("Step failed - initiating rollback")

            // Rollback all previously executed steps
            rc.rollback(ctx)
            return fmt.Errorf("step failed: %w", err)
        }

        rc.executed = append(rc.executed, step)
    }

    return nil
}

func (rc *RollbackableCommand) rollback(ctx context.Context) {
    // Rollback in reverse order
    for i := len(rc.executed) - 1; i >= 0; i-- {
        step := rc.executed[i]

        log.Info().
            Str("step", step.GetDescription()).
            Str("document_id", rc.doc.ID).
            Msg("Rolling back command step")

        if err := step.Rollback(ctx); err != nil {
            log.Error().
                Err(err).
                Str("step", step.GetDescription()).
                Msg("Rollback failed - manual cleanup may be needed")
        }
    }
}

// Example: Filing command with rollback
type AddSearchTermsStep struct {
    doc           *outline.Document
    searchTerms   []string
    originalText  string
    client        outline.Client
}

func (s *AddSearchTermsStep) Execute(ctx context.Context) error {
    s.originalText = s.doc.Text

    searchTermsText := fmt.Sprintf("\n\n---\n<!-- AI-SEARCH-TERMS-START -->\n"+
        "**Search Terms**: %s\n<!-- AI-SEARCH-TERMS-END -->",
        strings.Join(s.searchTerms, ", "))

    updatedText := s.doc.Text + searchTermsText

    updateReq := &outline.UpdateDocumentRequest{
        Text: updatedText,
        Done: true,
    }

    _, err := s.client.UpdateDocument(ctx, s.doc.ID, updateReq)
    return err
}

func (s *AddSearchTermsStep) Rollback(ctx context.Context) error {
    // Restore original text
    updateReq := &outline.UpdateDocumentRequest{
        Text: s.originalText,
        Done: true,
    }

    _, err := s.client.UpdateDocument(ctx, s.doc.ID, updateReq)
    return err
}

func (s *AddSearchTermsStep) GetDescription() string {
    return "add_search_terms"
}

type MoveDocumentStep struct {
    doc               *outline.Document
    targetCollectionID string
    sourceCollectionID string
    client            outline.Client
}

func (s *MoveDocumentStep) Execute(ctx context.Context) error {
    s.sourceCollectionID = s.doc.CollectionID

    err := s.client.MoveDocument(ctx, s.doc.ID, s.targetCollectionID)
    return err
}

func (s *MoveDocumentStep) Rollback(ctx context.Context) error {
    // Move back to original collection
    err := s.client.MoveDocument(ctx, s.doc.ID, s.sourceCollectionID)
    return err
}

func (s *MoveDocumentStep) GetDescription() string {
    return "move_document"
}
```

### User Notification on Failures

Provide clear, actionable feedback to users when commands fail.

```go
package command

type UserNotifier struct {
    outlineClient outline.Client
}

type NotificationLevel int

const (
    NotificationInfo NotificationLevel = iota
    NotificationWarning
    NotificationError
)

type UserNotification struct {
    Level        NotificationLevel
    Title        string
    Message      string
    ActionItems  []string
    TechnicalDetails string
}

func (un *UserNotifier) NotifyUser(
    ctx context.Context,
    documentID string,
    notification *UserNotification,
) error {
    emoji := "ℹ️"
    switch notification.Level {
    case NotificationWarning:
        emoji = "⚠️"
    case NotificationError:
        emoji = "❌"
    }

    var actionText string
    if len(notification.ActionItems) > 0 {
        actionText = "\n\n**What you can do:**\n"
        for _, action := range notification.ActionItems {
            actionText += fmt.Sprintf("- %s\n", action)
        }
    }

    comment := fmt.Sprintf("%s **%s**\n\n%s%s",
        emoji, notification.Title, notification.Message, actionText)

    // Add technical details in collapsed section
    if notification.TechnicalDetails != "" {
        comment += fmt.Sprintf("\n\n<details>\n<summary>Technical Details</summary>\n\n```\n%s\n```\n</details>",
            notification.TechnicalDetails)
    }

    commentReq := &outline.CreateCommentRequest{
        DocumentID: documentID,
        Data:       outline.NewCommentContent(comment),
    }

    _, err := un.outlineClient.CreateComment(ctx, commentReq)
    return err
}

// Example usage
func (h *AIFileHandler) notifyFilingFailure(
    ctx context.Context,
    doc *outline.Document,
    err error,
) {
    notification := &UserNotification{
        Level:   NotificationError,
        Title:   "Filing Failed",
        Message: "Unable to automatically file this document.",
        ActionItems: []string{
            "Review the document content and ensure it's ready for filing",
            "Add guidance to help the AI: `/ai-file [your guidance here]`",
            "Manually move the document to the desired collection",
            "Contact support if this error persists",
        },
        TechnicalDetails: fmt.Sprintf("Error: %v\nDocument ID: %s\nTimestamp: %s",
            err, doc.ID, time.Now().Format(time.RFC3339)),
    }

    notifier := &UserNotifier{outlineClient: h.outlineClient}
    notifier.NotifyUser(ctx, doc.ID, notification)
}
```

### Command State Persistence

Track command execution state for recovery.

```go
package command

// Add to persistence layer schema
const commandStateSchema = `
CREATE TABLE IF NOT EXISTS command_state (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    command_id TEXT NOT NULL UNIQUE,
    document_id TEXT NOT NULL,
    command_type TEXT NOT NULL,
    command_args TEXT,
    status TEXT NOT NULL,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    last_attempt TIMESTAMP,
    last_error TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_command_state_document ON command_state(document_id);
CREATE INDEX idx_command_state_status ON command_state(status);
CREATE INDEX idx_command_state_last_attempt ON command_state(last_attempt);
`

type CommandState struct {
    ID           int64
    CommandID    string    // Hash of document_id + command_type + args
    DocumentID   string
    CommandType  string
    CommandArgs  string
    Status       string    // pending, processing, completed, failed
    AttemptCount int
    LastAttempt  time.Time
    LastError    string
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

type CommandStateTracker interface {
    RecordCommandAttempt(ctx context.Context, state *CommandState) error
    GetCommandState(ctx context.Context, commandID string) (*CommandState, error)
    MarkCommandCompleted(ctx context.Context, commandID string) error
    MarkCommandFailed(ctx context.Context, commandID string, err error) error
    GetPendingCommands(ctx context.Context, olderThan time.Duration) ([]*CommandState, error)
}

// Clean up old command state
func CleanupOldCommandState(ctx context.Context, storage CommandStateTracker, olderThan time.Duration) error {
    cutoff := time.Now().Add(-olderThan)

    // This would be implemented in persistence layer
    log.Info().
        Time("cutoff", cutoff).
        Msg("Cleaning up old command state")

    return nil
}
```

## Testing Strategy

### Unit Tests

```go
func TestRegexDetector_DetectCommands(t *testing.T)
func TestRegexDetector_HasCommand(t *testing.T)
func TestRouter_RegisterHandler(t *testing.T)
func TestRouter_Route(t *testing.T)
func TestDefaultProcessor_ProcessDocument(t *testing.T)
func TestDefaultProcessor_ProcessCommand(t *testing.T)
func TestAIQuestionHandler(t *testing.T)
func TestAIFileHandler_HighConfidence(t *testing.T)
func TestAIFileHandler_LowConfidence(t *testing.T)
func TestSummarizeHandler(t *testing.T)

// Error recovery tests
func TestFailureHandler_DetermineStrategy(t *testing.T)
func TestFailureHandler_ConvertMarker(t *testing.T)
func TestDualMarkerHandler_Cleanup(t *testing.T)
func TestCommentPoster_Retry(t *testing.T)
func TestRollbackableCommand_Execute(t *testing.T)
func TestRollbackableCommand_Rollback(t *testing.T)
func TestUserNotifier_FormatMessages(t *testing.T)
```

## SOHO Deployment Considerations

### Simplifications for Homelab

1. **Simple regex patterns**: No complex parsing
2. **Sequential processing**: Process commands one at a time
3. **No command queue**: Immediate execution
4. **Fixed command set**: No dynamic command registration
5. **Basic error handling**: Continue on failure

## Package Structure

```
internal/command/
├── detector.go         # Command detection
├── processor.go        # Command processing
├── router.go           # Handler routing
├── handlers/
│   ├── ai_question.go  # /ai handler
│   ├── ai_file.go      # /ai-file handler
│   ├── summarize.go    # /summarize handler
│   ├── enhance_title.go # /enhance-title handler
│   └── related.go      # /related handler
└── command_test.go     # Test suite
```

## Dependencies

- `github.com/yourusername/outline-ai/internal/ai` - AI client
- `github.com/yourusername/outline-ai/internal/outline` - Outline client
- `github.com/yourusername/outline-ai/internal/taxonomy` - Taxonomy builder
- `github.com/rs/zerolog` - Logging

---

**Status:** Ready for implementation
**Complexity:** Medium
**Priority:** High (core command functionality)
