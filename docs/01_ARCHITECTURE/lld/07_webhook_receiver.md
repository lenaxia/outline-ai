# Low-Level Design: Webhook Receiver

**Domain:** Event Processing
**Status:** Design
**Last Updated:** 2026-01-19
**Target Deployment:** Homelab/SOHO

## Purpose

Receive real-time webhook events from Outline for document updates and trigger command processing immediately.

## Design Principles

1. **Security First**: HMAC signature validation for all webhooks
2. **Fast Response**: Respond within 5 seconds to prevent auto-disable
3. **Async Processing**: Queue events for background processing
4. **Event Filtering**: Only process relevant event types
5. **Fallback Support**: Graceful degradation to polling if webhooks fail

## Event Processing Flow

### High-Level Flow

```
1. Outline sends webhook → HTTPReceiver.handleWebhook()
2. Validate HMAC signature
3. Parse JSON payload
4. Queue event (non-blocking)
5. Return 200 OK immediately (< 5s)
6. Background: processEvents() picks up event
7. Route to registered EventHandler
8. Handler processes document asynchronously
```

**Critical Requirements:**
- **Fast Response**: Must respond within 5 seconds or Outline disables webhook
- **Signature Validation**: Always validate HMAC before processing
- **Async Processing**: Never block HTTP response on business logic
- **Event Filtering**: Only process configured event types
- **Error Handling**: Log errors but don't fail webhook response

**Processing Guarantees:**
- **At-most-once delivery**: Event queue is in-memory
- **No guaranteed ordering**: Events processed concurrently by worker pool
- **Fire-and-forget**: Failed events are logged but not retried via webhook
- **Fallback available**: Polling can catch missed events

## Webhook Event Structure

### Outline Webhook Format

```go
package webhook

import (
    "time"
)

type OutlineWebhookEvent struct {
    Event     string                 `json:"event"`
    Model     string                 `json:"model"`
    ModelID   string                 `json:"modelId"`
    Payload   map[string]interface{} `json:"payload"`
    ActorID   string                 `json:"actorId"`
    Timestamp time.Time              `json:"timestamp"`
}

type DocumentUpdatePayload struct {
    ID           string    `json:"id"`
    CollectionID string    `json:"collectionId"`
    Title        string    `json:"title"`
    Text         string    `json:"text"`
    UpdatedAt    time.Time `json:"updatedAt"`
}
```

## Receiver Interface

### HTTP Handler Interface

```go
package webhook

import (
    "context"
    "net/http"
)

type Receiver interface {
    // Start HTTP server
    Start(ctx context.Context) error

    // Stop HTTP server
    Stop(ctx context.Context) error

    // Register event handler
    RegisterHandler(eventType string, handler EventHandler)

    // Get statistics
    GetStats() ReceiverStats
}

type EventHandler interface {
    HandleEvent(ctx context.Context, event *OutlineWebhookEvent) error
}

type ReceiverStats struct {
    TotalReceived        int64
    ValidSignatures      int64
    InvalidSignatures    int64
    ProcessedSuccessfully int64
    ProcessingFailed     int64
    LastEventTime        time.Time
}
```

## HTTP Server Implementation

### Main Receiver

```go
package webhook

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "sync"
    "time"

    "github.com/rs/zerolog/log"
)

type HTTPReceiver struct {
    server         *http.Server
    secret         string
    handlers       map[string]EventHandler
    handlersMu     sync.RWMutex
    stats          ReceiverStats
    statsMu        sync.RWMutex
    eventQueue     chan *OutlineWebhookEvent
    queueSize      int
}

func NewHTTPReceiver(port int, secret string, queueSize int) *HTTPReceiver {
    receiver := &HTTPReceiver{
        secret:     secret,
        handlers:   make(map[string]EventHandler),
        eventQueue: make(chan *OutlineWebhookEvent, queueSize),
        queueSize:  queueSize,
    }

    mux := http.NewServeMux()
    mux.HandleFunc("/webhooks", receiver.handleWebhook)
    mux.HandleFunc("/health", receiver.handleHealth)

    receiver.server = &http.Server{
        Addr:         fmt.Sprintf(":%d", port),
        Handler:      mux,
        ReadTimeout:  10 * time.Second,
        WriteTimeout: 10 * time.Second,
    }

    return receiver
}

func (r *HTTPReceiver) Start(ctx context.Context) error {
    // Start event processor
    go r.processEvents(ctx)

    log.Info().
        Str("addr", r.server.Addr).
        Msg("Starting webhook receiver")

    if err := r.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        return fmt.Errorf("webhook server failed: %w", err)
    }

    return nil
}

func (r *HTTPReceiver) Stop(ctx context.Context) error {
    log.Info().Msg("Stopping webhook receiver")

    if err := r.server.Shutdown(ctx); err != nil {
        return fmt.Errorf("failed to shutdown webhook server: %w", err)
    }

    close(r.eventQueue)
    return nil
}

func (r *HTTPReceiver) RegisterHandler(eventType string, handler EventHandler) {
    r.handlersMu.Lock()
    defer r.handlersMu.Unlock()

    r.handlers[eventType] = handler

    log.Info().
        Str("event_type", eventType).
        Msg("Registered webhook handler")
}
```

### Webhook Handler

```go
package webhook

func (r *HTTPReceiver) handleWebhook(w http.ResponseWriter, req *http.Request) {
    // Only accept POST
    if req.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Read body
    body, err := io.ReadAll(req.Body)
    if err != nil {
        log.Error().Err(err).Msg("Failed to read webhook body")
        http.Error(w, "Failed to read body", http.StatusBadRequest)
        return
    }
    defer req.Body.Close()

    // Update stats
    r.updateStats(func(s *ReceiverStats) {
        s.TotalReceived++
    })

    // Validate signature
    signature := req.Header.Get("Outline-Signature")
    if !r.validateSignature(body, signature) {
        log.Warn().
            Str("signature", signature).
            Msg("Invalid webhook signature")

        r.updateStats(func(s *ReceiverStats) {
            s.InvalidSignatures++
        })

        http.Error(w, "Invalid signature", http.StatusUnauthorized)
        return
    }

    r.updateStats(func(s *ReceiverStats) {
        s.ValidSignatures++
    })

    // Parse event
    var event OutlineWebhookEvent
    if err := json.Unmarshal(body, &event); err != nil {
        log.Error().Err(err).Msg("Failed to parse webhook event")
        http.Error(w, "Invalid event format", http.StatusBadRequest)
        return
    }

    log.Info().
        Str("event", event.Event).
        Str("model", event.Model).
        Str("model_id", event.ModelID).
        Msg("Received webhook event")

    // Queue event for processing
    select {
    case r.eventQueue <- &event:
        // Successfully queued
    default:
        log.Warn().
            Str("event", event.Event).
            Msg("Event queue full, dropping event")
        http.Error(w, "Queue full", http.StatusServiceUnavailable)
        return
    }

    // Respond immediately (must respond within 5 seconds)
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"status":"accepted"}`))
}
```

## Signature Validation

### HMAC-SHA256 Validation

```go
package webhook

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
)

func (r *HTTPReceiver) validateSignature(body []byte, signature string) bool {
    if r.secret == "" {
        log.Warn().Msg("Webhook secret not configured, skipping signature validation")
        return true
    }

    if signature == "" {
        return false
    }

    // Calculate expected signature
    mac := hmac.New(sha256.New, []byte(r.secret))
    mac.Write(body)
    expectedSignature := hex.EncodeToString(mac.Sum(nil))

    // Compare signatures (constant time to prevent timing attacks)
    return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

func GenerateSignature(body []byte, secret string) string {
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(body)
    return hex.EncodeToString(mac.Sum(nil))
}
```

## Event Processing

### Async Event Processor

```go
package webhook

func (r *HTTPReceiver) processEvents(ctx context.Context) {
    log.Info().Msg("Starting webhook event processor")

    for {
        select {
        case <-ctx.Done():
            log.Info().Msg("Webhook event processor stopped")
            return

        case event := <-r.eventQueue:
            if event == nil {
                // Channel closed
                return
            }

            if err := r.processEvent(ctx, event); err != nil {
                log.Error().
                    Err(err).
                    Str("event", event.Event).
                    Str("model_id", event.ModelID).
                    Msg("Failed to process webhook event")

                r.updateStats(func(s *ReceiverStats) {
                    s.ProcessingFailed++
                })
            } else {
                r.updateStats(func(s *ReceiverStats) {
                    s.ProcessedSuccessfully++
                    s.LastEventTime = time.Now()
                })
            }
        }
    }
}

func (r *HTTPReceiver) processEvent(ctx context.Context, event *OutlineWebhookEvent) error {
    r.handlersMu.RLock()
    handler, exists := r.handlers[event.Event]
    r.handlersMu.RUnlock()

    if !exists {
        log.Debug().
            Str("event", event.Event).
            Msg("No handler registered for event type")
        return nil
    }

    // Call handler with timeout
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    return handler.HandleEvent(ctx, event)
}
```

## Event Type Filtering

### Document Event Handler

```go
package webhook

import (
    "context"
    "fmt"

    "github.com/yourusername/outline-ai/internal/outline"
)

type DocumentEventHandler struct {
    outlineClient outline.Client
    commandProcessor CommandProcessor
}

type CommandProcessor interface {
    ProcessDocument(ctx context.Context, documentID string) error
}

func NewDocumentEventHandler(client outline.Client, processor CommandProcessor) *DocumentEventHandler {
    return &DocumentEventHandler{
        outlineClient:    client,
        commandProcessor: processor,
    }
}

func (h *DocumentEventHandler) HandleEvent(ctx context.Context, event *OutlineWebhookEvent) error {
    // Filter for document events
    if event.Model != "document" {
        return nil
    }

    // Only process update and create events
    switch event.Event {
    case "documents.update", "documents.create":
        // Process the document
        return h.commandProcessor.ProcessDocument(ctx, event.ModelID)

    default:
        log.Debug().
            Str("event", event.Event).
            Msg("Ignoring document event")
        return nil
    }
}
```

## Health Check

### Health Endpoint

```go
package webhook

func (r *HTTPReceiver) handleHealth(w http.ResponseWriter, req *http.Request) {
    stats := r.GetStats()

    health := map[string]interface{}{
        "status":                  "healthy",
        "total_received":          stats.TotalReceived,
        "valid_signatures":        stats.ValidSignatures,
        "invalid_signatures":      stats.InvalidSignatures,
        "processed_successfully":  stats.ProcessedSuccessfully,
        "processing_failed":       stats.ProcessingFailed,
        "last_event_time":         stats.LastEventTime,
        "queue_size":              len(r.eventQueue),
        "queue_capacity":          r.queueSize,
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(health)
}
```

## Statistics Management

### Thread-Safe Stats

```go
package webhook

func (r *HTTPReceiver) GetStats() ReceiverStats {
    r.statsMu.RLock()
    defer r.statsMu.RUnlock()

    return r.stats
}

func (r *HTTPReceiver) updateStats(fn func(*ReceiverStats)) {
    r.statsMu.Lock()
    defer r.statsMu.Unlock()

    fn(&r.stats)
}

func (r *HTTPReceiver) ResetStats() {
    r.statsMu.Lock()
    defer r.statsMu.Unlock()

    r.stats = ReceiverStats{}
    log.Info().Msg("Webhook receiver stats reset")
}
```

## Webhook Registration

### Register with Outline

```go
package webhook

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

type WebhookRegistration struct {
    Name   string   `json:"name"`
    URL    string   `json:"url"`
    Secret string   `json:"secret"`
    Events []string `json:"events"`
}

func RegisterWebhook(outlineURL, apiKey string, reg *WebhookRegistration) error {
    body, err := json.Marshal(reg)
    if err != nil {
        return fmt.Errorf("failed to marshal registration: %w", err)
    }

    req, err := http.NewRequest("POST", outlineURL+"/webhooks.create", bytes.NewReader(body))
    if err != nil {
        return fmt.Errorf("failed to create request: %w", err)
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+apiKey)

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return fmt.Errorf("registration request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("registration failed with status: %d", resp.StatusCode)
    }

    log.Info().
        Str("url", reg.URL).
        Strs("events", reg.Events).
        Msg("Webhook registered successfully")

    return nil
}
```

## Webhook Failure Recovery

### Overview

The webhook receiver implements comprehensive failure recovery strategies to handle various failure scenarios including prolonged downtime, event queue overflow, and signature validation failures.

### Missed Events During Downtime

When the service is down for extended periods (hours or days), webhooks are lost. Implement a catch-up mechanism.

```go
package webhook

import (
    "context"
    "time"
)

type CatchUpService struct {
    outlineClient    outline.Client
    commandProcessor CommandProcessor
    storage          persistence.Storage
}

type CatchUpState struct {
    LastProcessedTime time.Time
    LastDocumentID    string
}

func (cs *CatchUpService) PerformCatchUp(ctx context.Context) error {
    // Load last processed time from storage
    state, err := cs.storage.GetCatchUpState(ctx)
    if err != nil {
        log.Warn().Err(err).Msg("No catch-up state found, starting from now")
        state = &CatchUpState{
            LastProcessedTime: time.Now().Add(-1 * time.Hour),
        }
    }

    downtime := time.Since(state.LastProcessedTime)

    log.Info().
        Time("last_processed", state.LastProcessedTime).
        Dur("downtime", downtime).
        Msg("Starting catch-up process for missed events")

    // Strategy depends on downtime duration
    if downtime > 24*time.Hour {
        // Long downtime: Full scan of recent documents
        return cs.fullScanCatchUp(ctx, state)
    } else if downtime > 1*time.Hour {
        // Medium downtime: Query recently updated documents
        return cs.incrementalCatchUp(ctx, state)
    } else {
        // Short downtime: Minimal catch-up needed
        return cs.quickCatchUp(ctx, state)
    }
}

func (cs *CatchUpService) fullScanCatchUp(ctx context.Context, state *CatchUpState) error {
    log.Info().Msg("Performing full scan catch-up (downtime > 24h)")

    // Search for documents with command markers
    commandMarkers := []string{"/ai", "/ai-file", "?ai-file", "/summarize", "/enhance-title", "/related"}

    for _, marker := range commandMarkers {
        log.Info().Str("marker", marker).Msg("Searching for command marker")

        // Use Outline search API to find documents with markers
        docs, err := cs.outlineClient.SearchDocuments(ctx, marker)
        if err != nil {
            log.Error().Err(err).Str("marker", marker).Msg("Search failed")
            continue
        }

        log.Info().
            Str("marker", marker).
            Int("found", len(docs)).
            Msg("Found documents with marker")

        // Process each document
        for _, doc := range docs {
            // Check if document was updated during downtime
            if doc.UpdatedAt.After(state.LastProcessedTime) {
                log.Info().
                    Str("document_id", doc.ID).
                    Str("title", doc.Title).
                    Msg("Processing missed document")

                if err := cs.commandProcessor.ProcessDocument(ctx, doc.ID); err != nil {
                    log.Error().
                        Err(err).
                        Str("document_id", doc.ID).
                        Msg("Failed to process document during catch-up")
                }
            }
        }
    }

    // Update catch-up state
    newState := &CatchUpState{
        LastProcessedTime: time.Now(),
    }
    cs.storage.SaveCatchUpState(ctx, newState)

    log.Info().Msg("Full scan catch-up completed")
    return nil
}

func (cs *CatchUpService) incrementalCatchUp(ctx context.Context, state *CatchUpState) error {
    log.Info().Msg("Performing incremental catch-up (downtime 1-24h)")

    // Query documents updated since last processed time
    // This requires using Outline's list documents API with date filtering
    // Note: Outline API may not support date filtering, fallback to search

    return cs.fullScanCatchUp(ctx, state)
}

func (cs *CatchUpService) quickCatchUp(ctx context.Context, state *CatchUpState) error {
    log.Info().Msg("Performing quick catch-up (downtime < 1h)")

    // For short downtimes, check only for high-priority commands
    // or recently updated documents in specific collections

    docs, err := cs.outlineClient.SearchDocuments(ctx, "/ai-file")
    if err != nil {
        return err
    }

    for _, doc := range docs {
        if doc.UpdatedAt.After(state.LastProcessedTime) {
            cs.commandProcessor.ProcessDocument(ctx, doc.ID)
        }
    }

    // Update state
    newState := &CatchUpState{
        LastProcessedTime: time.Now(),
    }
    cs.storage.SaveCatchUpState(ctx, newState)

    return nil
}

// Run catch-up on startup
func (cs *CatchUpService) RunOnStartup(ctx context.Context) error {
    log.Info().Msg("Running catch-up process on startup")

    if err := cs.PerformCatchUp(ctx); err != nil {
        log.Error().Err(err).Msg("Catch-up process failed")
        return err
    }

    log.Info().Msg("Catch-up process completed successfully")
    return nil
}
```

### Catch-Up State Schema

```sql
-- Store catch-up state for recovery after downtime
CREATE TABLE IF NOT EXISTS catchup_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),  -- Only one row
    last_processed_time TIMESTAMP NOT NULL,
    last_document_id TEXT,
    last_catchup_duration_ms INTEGER,
    documents_processed INTEGER,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### Event Queue Overflow

Handle scenarios where the in-memory event queue fills up.

```go
package webhook

type OverflowHandler struct {
    storage       persistence.Storage
    alerter       Alerter
    overflowCount int64
}

type Alerter interface {
    SendAlert(ctx context.Context, alert *Alert) error
}

type Alert struct {
    Level       string
    Title       string
    Message     string
    Details     map[string]interface{}
}

func (r *HTTPReceiver) handleQueueOverflow(event *OutlineWebhookEvent) {
    atomic.AddInt64(&r.overflowHandler.overflowCount, 1)

    log.Error().
        Str("event", event.Event).
        Str("model_id", event.ModelID).
        Int64("overflow_count", atomic.LoadInt64(&r.overflowHandler.overflowCount)).
        Msg("Event queue overflow - storing to database")

    // Store overflow event to database for later processing
    if err := r.overflowHandler.storage.StoreOverflowEvent(context.Background(), event); err != nil {
        log.Error().
            Err(err).
            Str("event", event.Event).
            Msg("Failed to store overflow event")
    }

    // Send alert if overflow count exceeds threshold
    if atomic.LoadInt64(&r.overflowHandler.overflowCount) > 10 {
        r.overflowHandler.alerter.SendAlert(context.Background(), &Alert{
            Level:   "critical",
            Title:   "Webhook Queue Overflow",
            Message: "Event queue is consistently full - events being dropped",
            Details: map[string]interface{}{
                "overflow_count": atomic.LoadInt64(&r.overflowHandler.overflowCount),
                "queue_size":     r.queueSize,
            },
        })
    }
}

// Background processor for overflow events
func (oh *OverflowHandler) ProcessOverflowEvents(ctx context.Context, processor EventHandler) {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := oh.processStoredEvents(ctx, processor); err != nil {
                log.Error().Err(err).Msg("Failed to process overflow events")
            }
        }
    }
}

func (oh *OverflowHandler) processStoredEvents(ctx context.Context, processor EventHandler) error {
    // Retrieve stored overflow events
    events, err := oh.storage.GetOverflowEvents(ctx, 100) // Process in batches
    if err != nil {
        return err
    }

    if len(events) == 0 {
        return nil
    }

    log.Info().Int("count", len(events)).Msg("Processing overflow events from database")

    for _, event := range events {
        if err := processor.HandleEvent(ctx, event); err != nil {
            log.Error().
                Err(err).
                Str("event_id", event.ModelID).
                Msg("Failed to process overflow event")
            continue
        }

        // Remove from overflow storage
        oh.storage.DeleteOverflowEvent(ctx, event.ModelID)
    }

    log.Info().Int("processed", len(events)).Msg("Overflow events processed")
    return nil
}
```

### Overflow Events Schema

```sql
-- Store events that couldn't fit in memory queue
CREATE TABLE IF NOT EXISTS overflow_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_type TEXT NOT NULL,
    model_type TEXT NOT NULL,
    model_id TEXT NOT NULL,
    payload TEXT NOT NULL,  -- JSON payload
    actor_id TEXT,
    event_timestamp TIMESTAMP NOT NULL,
    stored_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_overflow_events_stored ON overflow_events(stored_at);
CREATE INDEX idx_overflow_events_model ON overflow_events(model_id);
```

### Webhook Signature Validation Failure Recovery

Handle signature validation failures and potential secret rotation.

```go
package webhook

type SignatureValidator struct {
    currentSecret  string
    previousSecret string  // For secret rotation
    mu             sync.RWMutex
    failureCount   int64
}

func (sv *SignatureValidator) Validate(body []byte, signature string) bool {
    sv.mu.RLock()
    current := sv.currentSecret
    previous := sv.previousSecret
    sv.mu.RUnlock()

    // Try current secret
    if sv.validateWithSecret(body, signature, current) {
        // Reset failure count on success
        atomic.StoreInt64(&sv.failureCount, 0)
        return true
    }

    // Try previous secret (for rotation grace period)
    if previous != "" && sv.validateWithSecret(body, signature, previous) {
        log.Warn().Msg("Webhook validated with previous secret - rotation in progress")
        atomic.StoreInt64(&sv.failureCount, 0)
        return true
    }

    // Validation failed
    failures := atomic.AddInt64(&sv.failureCount, 1)

    log.Error().
        Int64("consecutive_failures", failures).
        Msg("Webhook signature validation failed")

    // Alert if many consecutive failures
    if failures >= 10 {
        log.Error().
            Int64("failures", failures).
            Msg("CRITICAL: Multiple consecutive signature validation failures - possible secret mismatch")
    }

    return false
}

func (sv *SignatureValidator) validateWithSecret(body []byte, signature, secret string) bool {
    if secret == "" {
        return false
    }

    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(body)
    expectedSignature := hex.EncodeToString(mac.Sum(nil))

    return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

func (sv *SignatureValidator) RotateSecret(newSecret, gracePeriodSecret string) {
    sv.mu.Lock()
    defer sv.mu.Unlock()

    sv.previousSecret = sv.currentSecret
    sv.currentSecret = newSecret

    log.Info().Msg("Webhook secret rotated - previous secret active during grace period")

    // Optional: Schedule removal of previous secret after grace period
    go func() {
        time.Sleep(24 * time.Hour) // Grace period
        sv.mu.Lock()
        sv.previousSecret = ""
        sv.mu.Unlock()
        log.Info().Msg("Previous webhook secret removed after grace period")
    }()
}

// Store failed signature validations for investigation
type SignatureFailure struct {
    Timestamp       time.Time
    ReceivedSignature string
    BodyHash        string
    IPAddress       string
}

func (sv *SignatureValidator) LogFailure(ctx context.Context, storage persistence.Storage, failure *SignatureFailure) {
    if err := storage.StoreSignatureFailure(ctx, failure); err != nil {
        log.Error().Err(err).Msg("Failed to log signature failure")
    }
}
```

### Reprocessing Strategies

Implement intelligent reprocessing for failed events.

```go
package webhook

type ReprocessingStrategy int

const (
    ReprocessImmediate ReprocessingStrategy = iota  // Retry immediately
    ReprocessDelayed                                // Retry after delay
    ReprocessManual                                 // Require manual intervention
    ReprocessSkip                                   // Skip permanently
)

type EventReprocessor struct {
    storage       persistence.Storage
    processor     EventHandler
    maxRetries    int
    retryBackoff  time.Duration
}

type FailedEvent struct {
    ID            int64
    Event         *OutlineWebhookEvent
    FailureReason string
    RetryCount    int
    NextRetry     time.Time
    Strategy      ReprocessingStrategy
}

func (er *EventReprocessor) DetermineStrategy(event *OutlineWebhookEvent, err error, retryCount int) ReprocessingStrategy {
    // Transient errors - retry
    if worker.IsTransientError(err) && retryCount < er.maxRetries {
        return ReprocessDelayed
    }

    // Permanent errors - skip
    if worker.IsPermanentError(err) {
        return ReprocessSkip
    }

    // Document not found - might be deleted, skip
    if errors.Is(err, outline.ErrDocumentNotFound) {
        return ReprocessSkip
    }

    // Max retries exceeded - manual intervention
    if retryCount >= er.maxRetries {
        return ReprocessManual
    }

    // Default: delayed retry
    return ReprocessDelayed
}

func (er *EventReprocessor) RecordFailure(ctx context.Context, event *OutlineWebhookEvent, err error, retryCount int) error {
    strategy := er.DetermineStrategy(event, err, retryCount)

    failedEvent := &FailedEvent{
        Event:         event,
        FailureReason: err.Error(),
        RetryCount:    retryCount,
        Strategy:      strategy,
    }

    // Calculate next retry time based on strategy
    switch strategy {
    case ReprocessImmediate:
        failedEvent.NextRetry = time.Now()
    case ReprocessDelayed:
        backoff := time.Duration(retryCount+1) * er.retryBackoff
        failedEvent.NextRetry = time.Now().Add(backoff)
    case ReprocessManual:
        failedEvent.NextRetry = time.Time{} // No automatic retry
    case ReprocessSkip:
        failedEvent.NextRetry = time.Time{} // Never retry
    }

    log.Warn().
        Str("event", event.Event).
        Str("model_id", event.ModelID).
        Int("retry_count", retryCount).
        Str("strategy", fmt.Sprintf("%v", strategy)).
        Time("next_retry", failedEvent.NextRetry).
        Msg("Recorded event failure for reprocessing")

    return er.storage.StoreFailedEvent(ctx, failedEvent)
}

func (er *EventReprocessor) ProcessFailedEvents(ctx context.Context) error {
    // Get events ready for retry
    events, err := er.storage.GetEventsForRetry(ctx, time.Now())
    if err != nil {
        return err
    }

    if len(events) == 0 {
        return nil
    }

    log.Info().Int("count", len(events)).Msg("Reprocessing failed events")

    for _, failedEvent := range events {
        log.Info().
            Str("event", failedEvent.Event.Event).
            Str("model_id", failedEvent.Event.ModelID).
            Int("retry_count", failedEvent.RetryCount).
            Msg("Retrying failed event")

        err := er.processor.HandleEvent(ctx, failedEvent.Event)
        if err != nil {
            // Record another failure
            er.RecordFailure(ctx, failedEvent.Event, err, failedEvent.RetryCount+1)
        } else {
            // Success - remove from failed events
            er.storage.DeleteFailedEvent(ctx, failedEvent.ID)
            log.Info().
                Str("event", failedEvent.Event.Event).
                Str("model_id", failedEvent.Event.ModelID).
                Msg("Failed event reprocessed successfully")
        }
    }

    return nil
}

// Background worker for reprocessing
func (er *EventReprocessor) StartReprocessingWorker(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := er.ProcessFailedEvents(ctx); err != nil {
                log.Error().Err(err).Msg("Failed to reprocess events")
            }
        }
    }
}
```

### Failed Events Schema

```sql
-- Store failed webhook events for reprocessing
CREATE TABLE IF NOT EXISTS failed_webhook_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_type TEXT NOT NULL,
    model_type TEXT NOT NULL,
    model_id TEXT NOT NULL,
    payload TEXT NOT NULL,  -- JSON payload
    failure_reason TEXT NOT NULL,
    retry_count INTEGER NOT NULL DEFAULT 0,
    next_retry TIMESTAMP,
    strategy TEXT NOT NULL,  -- immediate, delayed, manual, skip
    first_failure TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_failure TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_failed_events_next_retry ON failed_webhook_events(next_retry);
CREATE INDEX idx_failed_events_strategy ON failed_webhook_events(strategy);
CREATE INDEX idx_failed_events_model ON failed_webhook_events(model_id);
```

### Manual Intervention Procedures

Provide tools for manual recovery and intervention.

```go
package webhook

type ManualRecoveryTool struct {
    storage       persistence.Storage
    processor     EventHandler
    outlineClient outline.Client
}

// List failed events requiring manual intervention
func (mrt *ManualRecoveryTool) ListManualInterventionEvents(ctx context.Context) ([]*FailedEvent, error) {
    return mrt.storage.GetEventsByStrategy(ctx, ReprocessManual)
}

// Manually trigger reprocessing of specific event
func (mrt *ManualRecoveryTool) ManuallyReprocess(ctx context.Context, eventID int64) error {
    failedEvent, err := mrt.storage.GetFailedEvent(ctx, eventID)
    if err != nil {
        return fmt.Errorf("failed to get event: %w", err)
    }

    log.Info().
        Int64("event_id", eventID).
        Str("model_id", failedEvent.Event.ModelID).
        Msg("Manually reprocessing event")

    // Attempt reprocessing
    err = mrt.processor.HandleEvent(ctx, failedEvent.Event)
    if err != nil {
        log.Error().
            Err(err).
            Int64("event_id", eventID).
            Msg("Manual reprocessing failed")
        return err
    }

    // Success - remove from failed events
    mrt.storage.DeleteFailedEvent(ctx, eventID)

    log.Info().
        Int64("event_id", eventID).
        Msg("Manual reprocessing successful")

    return nil
}

// Mark event as permanently skipped
func (mrt *ManualRecoveryTool) SkipEvent(ctx context.Context, eventID int64, reason string) error {
    log.Info().
        Int64("event_id", eventID).
        Str("reason", reason).
        Msg("Permanently skipping failed event")

    return mrt.storage.UpdateEventStrategy(ctx, eventID, ReprocessSkip, reason)
}

// Reconstruct and reprocess event from document state
func (mrt *ManualRecoveryTool) ReconstructAndProcess(ctx context.Context, documentID string) error {
    log.Info().
        Str("document_id", documentID).
        Msg("Reconstructing event from document state")

    // Fetch current document
    doc, err := mrt.outlineClient.GetDocument(ctx, documentID)
    if err != nil {
        return fmt.Errorf("failed to fetch document: %w", err)
    }

    // Create synthetic event
    syntheticEvent := &OutlineWebhookEvent{
        Event:     "documents.update",
        Model:     "document",
        ModelID:   documentID,
        Timestamp: time.Now(),
    }

    // Process the event
    return mrt.processor.HandleEvent(ctx, syntheticEvent)
}
```

### Health Monitoring and Alerting

Monitor webhook receiver health and alert on issues.

```go
package webhook

type HealthMonitor struct {
    receiver      *HTTPReceiver
    alerter       Alerter
    checkInterval time.Duration
}

type HealthStatus struct {
    Healthy                bool
    LastEventTime          time.Time
    EventsSinceLastCheck   int64
    FailureRate            float64
    QueueUtilization       float64
    SignatureFailures      int64
    OverflowEvents         int64
}

func (hm *HealthMonitor) CheckHealth(ctx context.Context) *HealthStatus {
    stats := hm.receiver.GetStats()

    status := &HealthStatus{
        Healthy:              true,
        LastEventTime:        stats.LastEventTime,
        EventsSinceLastCheck: stats.TotalReceived,
        QueueUtilization:     float64(len(hm.receiver.eventQueue)) / float64(hm.receiver.queueSize),
        SignatureFailures:    stats.InvalidSignatures,
    }

    // Calculate failure rate
    if stats.TotalReceived > 0 {
        status.FailureRate = float64(stats.ProcessingFailed) / float64(stats.TotalReceived)
    }

    // Check for unhealthy conditions
    if time.Since(stats.LastEventTime) > 10*time.Minute {
        status.Healthy = false
        log.Warn().
            Time("last_event", stats.LastEventTime).
            Msg("No events received recently - webhook may be disabled")
    }

    if status.QueueUtilization > 0.8 {
        status.Healthy = false
        log.Warn().
            Float64("utilization", status.QueueUtilization).
            Msg("Event queue utilization high")
    }

    if status.FailureRate > 0.1 {
        status.Healthy = false
        log.Warn().
            Float64("failure_rate", status.FailureRate).
            Msg("High event processing failure rate")
    }

    return status
}

func (hm *HealthMonitor) StartMonitoring(ctx context.Context) {
    ticker := time.NewTicker(hm.checkInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            status := hm.CheckHealth(ctx)
            if !status.Healthy {
                hm.alerter.SendAlert(ctx, &Alert{
                    Level:   "warning",
                    Title:   "Webhook Receiver Unhealthy",
                    Message: "Webhook receiver health check failed",
                    Details: map[string]interface{}{
                        "last_event":        status.LastEventTime,
                        "failure_rate":      status.FailureRate,
                        "queue_utilization": status.QueueUtilization,
                        "signature_failures": status.SignatureFailures,
                    },
                })
            }
        }
    }
}
```

## Testing Strategy

### Unit Tests

```go
func TestHTTPReceiver_ValidateSignature(t *testing.T)
func TestHTTPReceiver_HandleWebhook(t *testing.T)
func TestHTTPReceiver_InvalidSignature(t *testing.T)
func TestHTTPReceiver_EventProcessing(t *testing.T)
func TestHTTPReceiver_EventFiltering(t *testing.T)
func TestHTTPReceiver_QueueFull(t *testing.T)
func TestHTTPReceiver_Stats(t *testing.T)
func TestDocumentEventHandler(t *testing.T)

// Error recovery tests
func TestCatchUpService_FullScan(t *testing.T)
func TestCatchUpService_Incremental(t *testing.T)
func TestOverflowHandler_StoreAndProcess(t *testing.T)
func TestSignatureValidator_Rotation(t *testing.T)
func TestEventReprocessor_DetermineStrategy(t *testing.T)
func TestEventReprocessor_ProcessFailedEvents(t *testing.T)
func TestManualRecoveryTool_Reprocess(t *testing.T)
func TestHealthMonitor_CheckHealth(t *testing.T)
```

### Test Webhook Generation

```go
func generateTestWebhook(t *testing.T, secret string, event OutlineWebhookEvent) (*http.Request, string) {
    body, err := json.Marshal(event)
    if err != nil {
        t.Fatalf("Failed to marshal event: %v", err)
    }

    signature := GenerateSignature(body, secret)

    req := httptest.NewRequest(http.MethodPost, "/webhooks", bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Outline-Signature", signature)

    return req, signature
}

func TestWebhookValidation(t *testing.T) {
    secret := "test-secret"
    receiver := NewHTTPReceiver(8081, secret, 100)

    event := OutlineWebhookEvent{
        Event:   "documents.update",
        Model:   "document",
        ModelID: "doc123",
    }

    req, _ := generateTestWebhook(t, secret, event)
    rec := httptest.NewRecorder()

    receiver.handleWebhook(rec, req)

    if rec.Code != http.StatusOK {
        t.Errorf("Expected status 200, got %d", rec.Code)
    }
}
```

## Error Handling

### Error Types

```go
package webhook

import "errors"

var (
    ErrInvalidSignature = errors.New("invalid webhook signature")
    ErrQueueFull        = errors.New("event queue full")
    ErrInvalidEvent     = errors.New("invalid event format")
    ErrNoHandler        = errors.New("no handler for event type")
)
```

## SOHO Deployment Considerations

### Simplifications for Homelab

1. **Single endpoint**: No load balancing
2. **In-memory queue**: No Redis/message broker
3. **Simple retry**: No DLQ (dead letter queue)
4. **Fixed queue size**: No dynamic scaling
5. **Synchronous handlers**: No complex async patterns

### Example Configuration

```yaml
webhooks:
  enabled: true
  port: 8081
  events: ["documents.update", "documents.create"]
  signature_validation: true
  queue_size: 1000

  # Fallback if webhooks fail
  fallback_polling:
    enabled: true
    interval: 60s
```

### Reverse Proxy Setup

For homelab deployment with reverse proxy:

```nginx
# nginx configuration
location /outline-ai/webhooks {
    proxy_pass http://localhost:8081/webhooks;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header Outline-Signature $http_outline_signature;
}
```

## Integration Example

### Main Service Integration

```go
package main

func setupWebhookReceiver(cfg *config.Config, processor *CommandProcessor) (*webhook.HTTPReceiver, error) {
    receiver := webhook.NewHTTPReceiver(
        cfg.Webhooks.Port,
        cfg.Outline.WebhookSecret,
        1000, // queue size
    )

    // Register document event handler
    handler := webhook.NewDocumentEventHandler(
        outlineClient,
        processor,
    )

    receiver.RegisterHandler("documents.update", handler)
    receiver.RegisterHandler("documents.create", handler)

    // Start receiver in background
    go func() {
        if err := receiver.Start(context.Background()); err != nil {
            log.Fatal().Err(err).Msg("Webhook receiver failed")
        }
    }()

    return receiver, nil
}
```

## Package Structure

```
internal/webhook/
├── receiver.go         # HTTP receiver implementation
├── models.go           # Event models
├── signature.go        # HMAC validation
├── handler.go          # Event handlers
├── registration.go     # Webhook registration
├── stats.go            # Statistics tracking
└── webhook_test.go     # Test suite
```

## Dependencies

- Standard library `net/http`, `crypto/hmac`
- `github.com/rs/zerolog` - Logging
- `github.com/yourusername/outline-ai/internal/outline` - Outline client

---

**Status:** Ready for implementation
**Complexity:** Medium
**Priority:** High (primary event mechanism)
