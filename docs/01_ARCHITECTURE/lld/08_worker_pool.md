# Low-Level Design: Worker Pool

**Domain:** Concurrent Processing
**Status:** Design
**Last Updated:** 2026-01-19
**Target Deployment:** Homelab/SOHO

## Purpose

Manage concurrent document processing with controlled parallelism, respecting rate limits and system resources.

## Design Principles

1. **Controlled Concurrency**: Configurable worker count
2. **Graceful Shutdown**: Wait for in-flight work to complete
3. **Context-Based Cancellation**: Support for timeouts and cancellation
4. **Work Queue**: Buffered channel for task distribution
5. **SOHO Optimized**: Simple goroutine pool, no complex scheduling

## Worker Pool Interface

### Interface Definition

```go
package worker

import (
    "context"
)

type Pool interface {
    // Submit work to the pool
    Submit(ctx context.Context, task Task) error

    // Start the worker pool
    Start(ctx context.Context)

    // Stop the worker pool (graceful shutdown)
    Stop(ctx context.Context) error

    // Get pool statistics
    GetStats() PoolStats
}

type Task interface {
    Execute(ctx context.Context) error
    GetID() string
    GetType() string
}

type PoolStats struct {
    WorkersCount    int
    ActiveWorkers   int
    QueuedTasks     int
    CompletedTasks  int64
    FailedTasks     int64
    AverageExecTime float64
}
```

## Pool Implementation

### Main Pool Structure

```go
package worker

import (
    "context"
    "fmt"
    "sync"
    "sync/atomic"
    "time"

    "github.com/rs/zerolog/log"
)

type SimplePool struct {
    workerCount   int
    taskQueue     chan Task
    queueSize     int
    wg            sync.WaitGroup
    ctx           context.Context
    cancel        context.CancelFunc

    // Statistics
    completedTasks int64
    failedTasks    int64
    activeWorkers  int32

    // Execution time tracking
    mu              sync.Mutex
    executionTimes  []time.Duration
    maxExecSamples  int
}

func NewSimplePool(workerCount, queueSize int) *SimplePool {
    return &SimplePool{
        workerCount:    workerCount,
        queueSize:      queueSize,
        taskQueue:      make(chan Task, queueSize),
        maxExecSamples: 100,
        executionTimes: make([]time.Duration, 0, 100),
    }
}

func (p *SimplePool) Start(ctx context.Context) {
    p.ctx, p.cancel = context.WithCancel(ctx)

    log.Info().
        Int("workers", p.workerCount).
        Int("queue_size", p.queueSize).
        Msg("Starting worker pool")

    // Start workers
    for i := 0; i < p.workerCount; i++ {
        p.wg.Add(1)
        go p.worker(i)
    }
}

func (p *SimplePool) Stop(ctx context.Context) error {
    log.Info().Msg("Stopping worker pool")

    // Cancel context to signal workers
    p.cancel()

    // Wait for workers to finish with timeout
    done := make(chan struct{})
    go func() {
        p.wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        log.Info().Msg("Worker pool stopped gracefully")
        return nil
    case <-ctx.Done():
        return fmt.Errorf("worker pool shutdown timeout")
    }
}

func (p *SimplePool) Submit(ctx context.Context, task Task) error {
    select {
    case p.taskQueue <- task:
        log.Debug().
            Str("task_id", task.GetID()).
            Str("task_type", task.GetType()).
            Msg("Task submitted to worker pool")
        return nil
    case <-ctx.Done():
        return ctx.Err()
    default:
        return fmt.Errorf("worker pool queue full")
    }
}
```

### Worker Implementation

```go
package worker

func (p *SimplePool) worker(id int) {
    defer p.wg.Done()

    log.Debug().Int("worker_id", id).Msg("Worker started")

    for {
        select {
        case <-p.ctx.Done():
            log.Debug().Int("worker_id", id).Msg("Worker stopped")
            return

        case task, ok := <-p.taskQueue:
            if !ok {
                log.Debug().Int("worker_id", id).Msg("Task queue closed")
                return
            }

            p.executeTask(id, task)
        }
    }
}

func (p *SimplePool) executeTask(workerID int, task Task) {
    atomic.AddInt32(&p.activeWorkers, 1)
    defer atomic.AddInt32(&p.activeWorkers, -1)

    startTime := time.Now()

    log.Info().
        Int("worker_id", workerID).
        Str("task_id", task.GetID()).
        Str("task_type", task.GetType()).
        Msg("Executing task")

    // Execute task with timeout context
    ctx, cancel := context.WithTimeout(p.ctx, 5*time.Minute)
    defer cancel()

    err := task.Execute(ctx)
    duration := time.Since(startTime)

    // Record execution time
    p.recordExecutionTime(duration)

    if err != nil {
        atomic.AddInt64(&p.failedTasks, 1)

        log.Error().
            Err(err).
            Int("worker_id", workerID).
            Str("task_id", task.GetID()).
            Str("task_type", task.GetType()).
            Dur("duration", duration).
            Msg("Task execution failed")
    } else {
        atomic.AddInt64(&p.completedTasks, 1)

        log.Info().
            Int("worker_id", workerID).
            Str("task_id", task.GetID()).
            Str("task_type", task.GetType()).
            Dur("duration", duration).
            Msg("Task completed successfully")
    }
}

func (p *SimplePool) recordExecutionTime(duration time.Duration) {
    p.mu.Lock()
    defer p.mu.Unlock()

    p.executionTimes = append(p.executionTimes, duration)

    // Keep only recent samples
    if len(p.executionTimes) > p.maxExecSamples {
        p.executionTimes = p.executionTimes[1:]
    }
}
```

### Statistics

```go
package worker

func (p *SimplePool) GetStats() PoolStats {
    completed := atomic.LoadInt64(&p.completedTasks)
    failed := atomic.LoadInt64(&p.failedTasks)
    active := atomic.LoadInt32(&p.activeWorkers)

    avgExecTime := p.calculateAverageExecTime()

    return PoolStats{
        WorkersCount:    p.workerCount,
        ActiveWorkers:   int(active),
        QueuedTasks:     len(p.taskQueue),
        CompletedTasks:  completed,
        FailedTasks:     failed,
        AverageExecTime: avgExecTime,
    }
}

func (p *SimplePool) calculateAverageExecTime() float64 {
    p.mu.Lock()
    defer p.mu.Unlock()

    if len(p.executionTimes) == 0 {
        return 0
    }

    var total time.Duration
    for _, d := range p.executionTimes {
        total += d
    }

    return float64(total) / float64(len(p.executionTimes)) / float64(time.Millisecond)
}
```

## Task Handler Pattern

### Handler Interface Convention

The worker pool uses a consistent handler pattern across the system:

1. **TaskHandler**: Generic document processing
   - Signature: `Handle(ctx context.Context, documentID string) error`
   - Used for: Background document processing tasks

2. **CommandProcessor**: Command-specific processing
   - Signature: `ProcessCommand(ctx context.Context, doc *outline.Document, command, guidance string) error`
   - Used for: User-initiated command processing

**Pattern Guidelines:**
- All handlers accept `context.Context` as first parameter
- Handlers return a single `error` value
- Document data is passed either by ID (for background tasks) or by full object (for commands)
- Additional parameters follow document parameter(s)
- Handlers should be stateless - state managed by the handler implementation, not the task

**Usage Example:**
```go
// Define a handler
type MyHandler struct {
    aiClient ai.Client
    storage  persistence.Storage
}

func (h *MyHandler) Handle(ctx context.Context, documentID string) error {
    // Implementation
    return nil
}

// Create and submit task
task := worker.NewDocumentTask(documentID, "my-task", myHandler)
pool.Submit(ctx, task)
```

## Task Types

### Document Processing Task

```go
package worker

import (
    "context"
    "fmt"

    "github.com/yourusername/outline-ai/internal/outline"
)

type DocumentTask struct {
    ID         string
    DocumentID string
    TaskType   string
    Handler    TaskHandler
}

type TaskHandler interface {
    Handle(ctx context.Context, documentID string) error
}

func NewDocumentTask(documentID string, taskType string, handler TaskHandler) *DocumentTask {
    return &DocumentTask{
        ID:         fmt.Sprintf("%s-%s", taskType, documentID),
        DocumentID: documentID,
        TaskType:   taskType,
        Handler:    handler,
    }
}

func (t *DocumentTask) Execute(ctx context.Context) error {
    return t.Handler.Handle(ctx, t.DocumentID)
}

func (t *DocumentTask) GetID() string {
    return t.ID
}

func (t *DocumentTask) GetType() string {
    return t.TaskType
}
```

### Command Processing Task

```go
package worker

type CommandTask struct {
    ID        string
    Document  *outline.Document
    Command   string
    Guidance  string
    Processor CommandProcessor
}

type CommandProcessor interface {
    ProcessCommand(ctx context.Context, doc *outline.Document, command, guidance string) error
}

func NewCommandTask(doc *outline.Document, command, guidance string, processor CommandProcessor) *CommandTask {
    return &CommandTask{
        ID:        fmt.Sprintf("cmd-%s-%s", command, doc.ID),
        Document:  doc,
        Command:   command,
        Guidance:  guidance,
        Processor: processor,
    }
}

func (t *CommandTask) Execute(ctx context.Context) error {
    return t.Processor.ProcessCommand(ctx, t.Document, t.Command, t.Guidance)
}

func (t *CommandTask) GetID() string {
    return t.ID
}

func (t *CommandTask) GetType() string {
    return t.Command
}
```

## Priority Queue Enhancement

### Priority-Based Pool (Optional)

```go
package worker

import (
    "container/heap"
    "sync"
)

type PriorityTask struct {
    Task     Task
    Priority int
    Index    int
}

type PriorityQueue []*PriorityTask

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
    return pq[i].Priority > pq[j].Priority
}

func (pq PriorityQueue) Swap(i, j int) {
    pq[i], pq[j] = pq[j], pq[i]
    pq[i].Index = i
    pq[j].Index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
    n := len(*pq)
    item := x.(*PriorityTask)
    item.Index = n
    *pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
    old := *pq
    n := len(old)
    item := old[n-1]
    old[n-1] = nil
    item.Index = -1
    *pq = old[0 : n-1]
    return item
}

type PriorityPool struct {
    *SimplePool
    priorityQueue *PriorityQueue
    pqMu          sync.Mutex
}

func (p *PriorityPool) SubmitWithPriority(ctx context.Context, task Task, priority int) error {
    p.pqMu.Lock()
    defer p.pqMu.Unlock()

    heap.Push(p.priorityQueue, &PriorityTask{
        Task:     task,
        Priority: priority,
    })

    return nil
}
```

## Retry Logic

### Retry Task Wrapper

```go
package worker

import (
    "context"
    "time"
)

type RetryableTask struct {
    task         Task
    maxRetries   int
    retryBackoff time.Duration
    attempt      int
}

func NewRetryableTask(task Task, maxRetries int, backoff time.Duration) *RetryableTask {
    return &RetryableTask{
        task:         task,
        maxRetries:   maxRetries,
        retryBackoff: backoff,
        attempt:      0,
    }
}

func (t *RetryableTask) Execute(ctx context.Context) error {
    var lastErr error

    for t.attempt = 0; t.attempt <= t.maxRetries; t.attempt++ {
        if t.attempt > 0 {
            // Wait before retry
            backoff := time.Duration(t.attempt) * t.retryBackoff

            log.Info().
                Str("task_id", t.task.GetID()).
                Int("attempt", t.attempt).
                Dur("backoff", backoff).
                Msg("Retrying task")

            select {
            case <-time.After(backoff):
            case <-ctx.Done():
                return ctx.Err()
            }
        }

        err := t.task.Execute(ctx)
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

func (t *RetryableTask) GetID() string {
    return t.task.GetID()
}

func (t *RetryableTask) GetType() string {
    return t.task.GetType()
}

func isRetryableError(err error) bool {
    // Implement error classification
    // For now, retry all errors
    return true
}
```

## Error Recovery

### Task Failure Handling

The worker pool implements a comprehensive error recovery strategy to handle various failure scenarios while maintaining system stability.

#### Failure Classification

```go
package worker

import (
    "errors"
    "fmt"
)

type ErrorCategory int

const (
    ErrorCategoryTransient ErrorCategory = iota  // Retry with backoff
    ErrorCategoryPermanent                       // Don't retry, log for manual review
    ErrorCategoryPartial                         // Part of task succeeded, retry remainder
)

type TaskError struct {
    Category       ErrorCategory
    OriginalError  error
    RetryAfter     time.Duration
    FailedStep     string
    RecoveryHint   string
}

func (e *TaskError) Error() string {
    return fmt.Sprintf("%s: %v (category: %d)", e.FailedStep, e.OriginalError, e.Category)
}

// IsTransientError checks if error should be retried
func IsTransientError(err error) bool {
    if err == nil {
        return false
    }

    // Network timeouts
    if errors.Is(err, context.DeadlineExceeded) {
        return true
    }

    // Rate limit errors
    if errors.Is(err, ErrRateLimitExceeded) {
        return true
    }

    // Temporary Outline API errors (5xx)
    if outlineErr, ok := err.(*outline.APIError); ok {
        return outlineErr.StatusCode >= 500 && outlineErr.StatusCode < 600
    }

    // Temporary AI errors
    if aiErr, ok := err.(*ai.TemporaryError); ok {
        return true
    }

    return false
}

func IsPermanentError(err error) bool {
    if err == nil {
        return false
    }

    // Auth failures
    if outlineErr, ok := err.(*outline.APIError); ok {
        return outlineErr.StatusCode == 401 || outlineErr.StatusCode == 403
    }

    // Not found
    if errors.Is(err, outline.ErrDocumentNotFound) {
        return true
    }

    // Invalid input
    if errors.Is(err, outline.ErrInvalidCollectionID) {
        return true
    }

    return false
}
```

#### Enhanced Retry Task with Error Classification

```go
package worker

type RetryableTaskV2 struct {
    task         Task
    maxRetries   int
    retryBackoff time.Duration
    attempt      int
    storage      TaskStorage  // For persistence
}

type TaskStorage interface {
    RecordTaskFailure(ctx context.Context, taskID string, err error, attempt int) error
    GetTaskAttempts(ctx context.Context, taskID string) (int, error)
    MarkTaskExhausted(ctx context.Context, taskID string, finalErr error) error
}

func (t *RetryableTaskV2) Execute(ctx context.Context) error {
    var lastErr error

    for t.attempt = 0; t.attempt <= t.maxRetries; t.attempt++ {
        if t.attempt > 0 {
            backoff := time.Duration(t.attempt) * t.retryBackoff

            log.Info().
                Str("task_id", t.task.GetID()).
                Int("attempt", t.attempt).
                Dur("backoff", backoff).
                Msg("Retrying task")

            select {
            case <-time.After(backoff):
            case <-ctx.Done():
                return ctx.Err()
            }
        }

        err := t.task.Execute(ctx)
        if err == nil {
            log.Info().
                Str("task_id", t.task.GetID()).
                Int("attempts", t.attempt+1).
                Msg("Task succeeded after retries")
            return nil
        }

        lastErr = err

        // Record failure
        if t.storage != nil {
            t.storage.RecordTaskFailure(ctx, t.task.GetID(), err, t.attempt+1)
        }

        // Check if error is permanent - stop retrying immediately
        if IsPermanentError(err) {
            log.Error().
                Err(err).
                Str("task_id", t.task.GetID()).
                Msg("Task failed with permanent error - not retrying")

            if t.storage != nil {
                t.storage.MarkTaskExhausted(ctx, t.task.GetID(), err)
            }
            return fmt.Errorf("permanent error: %w", err)
        }

        // Check if error is transient
        if !IsTransientError(err) {
            log.Warn().
                Err(err).
                Str("task_id", t.task.GetID()).
                Msg("Task failed with non-retryable error")
            return err
        }

        log.Warn().
            Err(err).
            Str("task_id", t.task.GetID()).
            Int("attempt", t.attempt+1).
            Msg("Task failed with transient error")
    }

    // Max retries exhausted
    if t.storage != nil {
        t.storage.MarkTaskExhausted(ctx, t.task.GetID(), lastErr)
    }

    return fmt.Errorf("max retries exceeded: %w", lastErr)
}
```

### Partial Failure Recovery

Handle scenarios where part of a task succeeds (e.g., document updated but move failed).

```go
package worker

type PartiallyRecoverableTask struct {
    DocumentID   string
    OutlineClient outline.Client
    Storage      persistence.Storage
}

type TaskCheckpoint struct {
    TaskID           string
    DocumentUpdated  bool
    SearchTermsAdded bool
    DocumentMoved    bool
    MarkersRemoved   bool
    CommentPosted    bool
}

func (t *PartiallyRecoverableTask) Execute(ctx context.Context) error {
    checkpoint := &TaskCheckpoint{TaskID: t.DocumentID}

    // Load previous checkpoint if exists
    if savedCheckpoint, err := t.LoadCheckpoint(ctx); err == nil {
        checkpoint = savedCheckpoint
    }

    // Step 1: Update document content (if not already done)
    if !checkpoint.DocumentUpdated {
        if err := t.updateDocumentContent(ctx); err != nil {
            return &TaskError{
                Category:     ErrorCategoryTransient,
                OriginalError: err,
                FailedStep:   "update_document_content",
                RecoveryHint: "Document update failed - safe to retry from beginning",
            }
        }
        checkpoint.DocumentUpdated = true
        t.SaveCheckpoint(ctx, checkpoint)
    }

    // Step 2: Add search terms (if not already done)
    if !checkpoint.SearchTermsAdded {
        if err := t.addSearchTerms(ctx); err != nil {
            // Document already updated - this is partial failure
            return &TaskError{
                Category:     ErrorCategoryPartial,
                OriginalError: err,
                FailedStep:   "add_search_terms",
                RecoveryHint: "Document updated but search terms failed - retry from this step",
            }
        }
        checkpoint.SearchTermsAdded = true
        t.SaveCheckpoint(ctx, checkpoint)
    }

    // Step 3: Move document (if not already done)
    if !checkpoint.DocumentMoved {
        if err := t.moveDocument(ctx); err != nil {
            return &TaskError{
                Category:     ErrorCategoryPartial,
                OriginalError: err,
                FailedStep:   "move_document",
                RecoveryHint: "Document updated and search terms added, but move failed",
            }
        }
        checkpoint.DocumentMoved = true
        t.SaveCheckpoint(ctx, checkpoint)
    }

    // Step 4: Remove command markers (if not already done)
    if !checkpoint.MarkersRemoved {
        if err := t.removeMarkers(ctx); err != nil {
            // Non-critical failure - log warning but don't fail task
            log.Warn().
                Err(err).
                Str("document_id", t.DocumentID).
                Msg("Failed to remove command markers - manual cleanup required")
            // Continue to next step
        } else {
            checkpoint.MarkersRemoved = true
            t.SaveCheckpoint(ctx, checkpoint)
        }
    }

    // Step 5: Post success comment
    if !checkpoint.CommentPosted {
        if err := t.postComment(ctx); err != nil {
            // Non-critical failure
            log.Warn().
                Err(err).
                Str("document_id", t.DocumentID).
                Msg("Failed to post success comment")
            // Don't fail the task
        } else {
            checkpoint.CommentPosted = true
        }
    }

    // Clean up checkpoint
    t.DeleteCheckpoint(ctx)

    return nil
}

func (t *PartiallyRecoverableTask) LoadCheckpoint(ctx context.Context) (*TaskCheckpoint, error) {
    // Implementation: Load from storage
    return nil, errors.New("not found")
}

func (t *PartiallyRecoverableTask) SaveCheckpoint(ctx context.Context, checkpoint *TaskCheckpoint) error {
    // Implementation: Save to storage
    return nil
}

func (t *PartiallyRecoverableTask) DeleteCheckpoint(ctx context.Context) error {
    // Implementation: Delete from storage
    return nil
}
```

### Retry Exhaustion Scenarios

When a task fails after max retries, implement dead letter queue pattern.

```go
package worker

type DeadLetterQueue struct {
    storage persistence.Storage
    mu      sync.Mutex
}

type DeadLetterEntry struct {
    ID              int64
    TaskID          string
    TaskType        string
    DocumentID      string
    FailureReason   string
    AttemptCount    int
    FirstFailure    time.Time
    LastFailure     time.Time
    ErrorDetails    string
    Checkpoint      string  // JSON checkpoint data
}

func (dlq *DeadLetterQueue) Add(ctx context.Context, entry *DeadLetterEntry) error {
    dlq.mu.Lock()
    defer dlq.mu.Unlock()

    log.Error().
        Str("task_id", entry.TaskID).
        Str("task_type", entry.TaskType).
        Str("document_id", entry.DocumentID).
        Int("attempts", entry.AttemptCount).
        Str("reason", entry.FailureReason).
        Msg("Task moved to dead letter queue")

    // Store in database
    return dlq.storage.AddDeadLetterEntry(ctx, entry)
}

func (dlq *DeadLetterQueue) List(ctx context.Context, limit int) ([]*DeadLetterEntry, error) {
    return dlq.storage.ListDeadLetterEntries(ctx, limit)
}

func (dlq *DeadLetterQueue) Retry(ctx context.Context, entryID int64, pool Pool) error {
    entry, err := dlq.storage.GetDeadLetterEntry(ctx, entryID)
    if err != nil {
        return fmt.Errorf("failed to get DLQ entry: %w", err)
    }

    log.Info().
        Str("task_id", entry.TaskID).
        Int64("dlq_entry_id", entryID).
        Msg("Retrying task from dead letter queue")

    // Reconstruct task (simplified - needs proper task factory)
    task := reconstructTask(entry)

    // Submit to worker pool
    if err := pool.Submit(ctx, task); err != nil {
        return fmt.Errorf("failed to submit task from DLQ: %w", err)
    }

    // Remove from DLQ on successful submission
    return dlq.storage.RemoveDeadLetterEntry(ctx, entryID)
}

func reconstructTask(entry *DeadLetterEntry) Task {
    // Implementation: Reconstruct task from stored metadata
    // This would use a task factory pattern
    return nil
}
```

### Dead Letter Queue Schema

Add to persistence layer:

```sql
-- Dead letter queue for failed tasks
CREATE TABLE IF NOT EXISTS dead_letter_queue (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT NOT NULL,
    task_type TEXT NOT NULL,
    document_id TEXT NOT NULL,
    failure_reason TEXT NOT NULL,
    attempt_count INTEGER NOT NULL,
    first_failure TIMESTAMP NOT NULL,
    last_failure TIMESTAMP NOT NULL,
    error_details TEXT,
    checkpoint TEXT,  -- JSON checkpoint data for partial recovery
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_dlq_task_id ON dead_letter_queue(task_id);
CREATE INDEX idx_dlq_document_id ON dead_letter_queue(document_id);
CREATE INDEX idx_dlq_last_failure ON dead_letter_queue(last_failure);
```

### State Cleanup After Failures

```go
package worker

type FailureCleanup struct {
    outlineClient outline.Client
    storage       persistence.Storage
}

func (fc *FailureCleanup) CleanupFailedTask(ctx context.Context, taskID, documentID string) error {
    log.Info().
        Str("task_id", taskID).
        Str("document_id", documentID).
        Msg("Cleaning up failed task state")

    // Remove task checkpoint
    if err := fc.storage.DeleteTaskCheckpoint(ctx, taskID); err != nil {
        log.Warn().Err(err).Msg("Failed to delete task checkpoint")
    }

    // Optionally add failure marker to document
    doc, err := fc.outlineClient.GetDocument(ctx, documentID)
    if err != nil {
        return fmt.Errorf("failed to get document for cleanup: %w", err)
    }

    // Add comment about failure
    comment := fmt.Sprintf("⚠️ Task failed after multiple retries. Task ID: %s. "+
        "Manual review may be needed. See logs for details.", taskID)

    commentReq := &outline.CreateCommentRequest{
        DocumentID: documentID,
        Data:       outline.NewCommentContent(comment),
    }

    if _, err := fc.outlineClient.CreateComment(ctx, commentReq); err != nil {
        log.Warn().
            Err(err).
            Str("document_id", documentID).
            Msg("Failed to post failure comment")
    }

    return nil
}

// CleanupOrphanedState finds and removes stale task state
func (fc *FailureCleanup) CleanupOrphanedState(ctx context.Context, olderThan time.Duration) error {
    cutoff := time.Now().Add(-olderThan)

    // Find checkpoints older than cutoff
    orphaned, err := fc.storage.FindOrphanedCheckpoints(ctx, cutoff)
    if err != nil {
        return fmt.Errorf("failed to find orphaned checkpoints: %w", err)
    }

    log.Info().
        Int("count", len(orphaned)).
        Msg("Cleaning up orphaned task checkpoints")

    for _, checkpoint := range orphaned {
        if err := fc.storage.DeleteTaskCheckpoint(ctx, checkpoint.TaskID); err != nil {
            log.Warn().
                Err(err).
                Str("task_id", checkpoint.TaskID).
                Msg("Failed to delete orphaned checkpoint")
        }
    }

    return nil
}
```

### Recovery From Worker Crashes

```go
package worker

type WorkerHealthMonitor struct {
    pool          *SimplePool
    lastHeartbeat map[int]time.Time
    mu            sync.Mutex
    timeout       time.Duration
}

func NewWorkerHealthMonitor(pool *SimplePool, timeout time.Duration) *WorkerHealthMonitor {
    return &WorkerHealthMonitor{
        pool:          pool,
        lastHeartbeat: make(map[int]time.Time),
        timeout:       timeout,
    }
}

func (m *WorkerHealthMonitor) RecordHeartbeat(workerID int) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.lastHeartbeat[workerID] = time.Now()
}

func (m *WorkerHealthMonitor) CheckWorkerHealth(ctx context.Context) {
    ticker := time.NewTicker(m.timeout / 2)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            m.detectStalledWorkers()
        }
    }
}

func (m *WorkerHealthMonitor) detectStalledWorkers() {
    m.mu.Lock()
    defer m.mu.Unlock()

    now := time.Now()
    for workerID, lastSeen := range m.lastHeartbeat {
        if now.Sub(lastSeen) > m.timeout {
            log.Error().
                Int("worker_id", workerID).
                Time("last_heartbeat", lastSeen).
                Dur("elapsed", now.Sub(lastSeen)).
                Msg("Worker appears to be stalled")

            // Implement recovery strategy:
            // 1. Log for monitoring/alerting
            // 2. Could restart worker (if pool supports it)
            // 3. Could drain queue and restart pool
            // 4. For now: just log and monitor
        }
    }
}

// Enhanced worker with heartbeats
func (p *SimplePool) workerWithMonitoring(id int, monitor *WorkerHealthMonitor) {
    defer p.wg.Done()

    log.Debug().Int("worker_id", id).Msg("Worker started")

    for {
        select {
        case <-p.ctx.Done():
            log.Debug().Int("worker_id", id).Msg("Worker stopped")
            return

        case task, ok := <-p.taskQueue:
            if !ok {
                log.Debug().Int("worker_id", id).Msg("Task queue closed")
                return
            }

            // Record heartbeat before and after task
            if monitor != nil {
                monitor.RecordHeartbeat(id)
            }

            p.executeTask(id, task)

            if monitor != nil {
                monitor.RecordHeartbeat(id)
            }
        }
    }
}
```

### Graceful Degradation

When worker pool is unhealthy, implement circuit breaker pattern.

```go
package worker

type CircuitBreaker struct {
    maxFailures   int
    resetTimeout  time.Duration
    failures      int
    lastFailure   time.Time
    state         CircuitState
    mu            sync.Mutex
}

type CircuitState int

const (
    CircuitClosed CircuitState = iota  // Normal operation
    CircuitOpen                         // Rejecting requests
    CircuitHalfOpen                     // Testing if recovered
)

func (cb *CircuitBreaker) Call(fn func() error) error {
    cb.mu.Lock()

    switch cb.state {
    case CircuitOpen:
        // Check if timeout elapsed
        if time.Since(cb.lastFailure) > cb.resetTimeout {
            cb.state = CircuitHalfOpen
            cb.mu.Unlock()
        } else {
            cb.mu.Unlock()
            return errors.New("circuit breaker is open")
        }
    case CircuitHalfOpen:
        cb.mu.Unlock()
    case CircuitClosed:
        cb.mu.Unlock()
    }

    err := fn()

    cb.mu.Lock()
    defer cb.mu.Unlock()

    if err != nil {
        cb.failures++
        cb.lastFailure = time.Now()

        if cb.failures >= cb.maxFailures {
            cb.state = CircuitOpen
            log.Warn().
                Int("failures", cb.failures).
                Msg("Circuit breaker opened")
        }
        return err
    }

    // Success - reset
    if cb.state == CircuitHalfOpen {
        cb.state = CircuitClosed
        cb.failures = 0
        log.Info().Msg("Circuit breaker closed - recovered")
    }

    return nil
}
```

## Testing Strategy

### Unit Tests

```go
func TestSimplePool_StartStop(t *testing.T)
func TestSimplePool_Submit(t *testing.T)
func TestSimplePool_ConcurrentExecution(t *testing.T)
func TestSimplePool_GracefulShutdown(t *testing.T)
func TestSimplePool_QueueFull(t *testing.T)
func TestSimplePool_Stats(t *testing.T)
func TestRetryableTask(t *testing.T)
func TestDocumentTask(t *testing.T)
func TestCommandTask(t *testing.T)

// Error recovery tests
func TestRetryableTask_TransientError(t *testing.T)
func TestRetryableTask_PermanentError(t *testing.T)
func TestRetryableTask_MaxRetriesExceeded(t *testing.T)
func TestPartiallyRecoverableTask_Checkpoints(t *testing.T)
func TestDeadLetterQueue_AddAndRetry(t *testing.T)
func TestCircuitBreaker_OpenClose(t *testing.T)
func TestWorkerHealthMonitor_DetectStalls(t *testing.T)
```

### Mock Task

```go
type MockTask struct {
    ID          string
    Type        string
    ExecuteFn   func(ctx context.Context) error
    ExecuteDelay time.Duration
}

func (m *MockTask) Execute(ctx context.Context) error {
    if m.ExecuteDelay > 0 {
        select {
        case <-time.After(m.ExecuteDelay):
        case <-ctx.Done():
            return ctx.Err()
        }
    }

    if m.ExecuteFn != nil {
        return m.ExecuteFn(ctx)
    }
    return nil
}

func (m *MockTask) GetID() string {
    return m.ID
}

func (m *MockTask) GetType() string {
    return m.Type
}

func TestPoolConcurrency(t *testing.T) {
    pool := NewSimplePool(3, 10)
    pool.Start(context.Background())
    defer pool.Stop(context.Background())

    var completed int32

    for i := 0; i < 10; i++ {
        task := &MockTask{
            ID:   fmt.Sprintf("task-%d", i),
            Type: "test",
            ExecuteFn: func(ctx context.Context) error {
                atomic.AddInt32(&completed, 1)
                return nil
            },
        }

        err := pool.Submit(context.Background(), task)
        if err != nil {
            t.Fatalf("Failed to submit task: %v", err)
        }
    }

    time.Sleep(100 * time.Millisecond)

    if atomic.LoadInt32(&completed) != 10 {
        t.Errorf("Expected 10 completed tasks, got %d", completed)
    }
}
```

## Performance Monitoring

### Metrics Collection

```go
package worker

import (
    "time"
)

type Metrics struct {
    mu sync.RWMutex

    TotalSubmitted  int64
    TotalCompleted  int64
    TotalFailed     int64
    QueueHighWater  int

    TaskDurations   map[string][]time.Duration
}

func (m *Metrics) RecordSubmission() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.TotalSubmitted++
}

func (m *Metrics) RecordCompletion(taskType string, duration time.Duration) {
    m.mu.Lock()
    defer m.mu.Unlock()

    m.TotalCompleted++

    if m.TaskDurations == nil {
        m.TaskDurations = make(map[string][]time.Duration)
    }
    m.TaskDurations[taskType] = append(m.TaskDurations[taskType], duration)
}

func (m *Metrics) RecordFailure() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.TotalFailed++
}

func (m *Metrics) UpdateQueueSize(size int) {
    m.mu.Lock()
    defer m.mu.Unlock()

    if size > m.QueueHighWater {
        m.QueueHighWater = size
    }
}

func (m *Metrics) GetMetrics() map[string]interface{} {
    m.mu.RLock()
    defer m.mu.RUnlock()

    metrics := map[string]interface{}{
        "total_submitted":  m.TotalSubmitted,
        "total_completed":  m.TotalCompleted,
        "total_failed":     m.TotalFailed,
        "queue_high_water": m.QueueHighWater,
        "task_durations":   m.summarizeDurations(),
    }

    return metrics
}

func (m *Metrics) summarizeDurations() map[string]interface{} {
    summary := make(map[string]interface{})

    for taskType, durations := range m.TaskDurations {
        if len(durations) == 0 {
            continue
        }

        var total time.Duration
        for _, d := range durations {
            total += d
        }

        summary[taskType] = map[string]interface{}{
            "count":   len(durations),
            "average": float64(total) / float64(len(durations)) / float64(time.Millisecond),
        }
    }

    return summary
}
```

## SOHO Deployment Considerations

### Simplifications for Homelab

1. **Fixed worker count**: No dynamic scaling
2. **In-memory queue**: No distributed queue (Redis, RabbitMQ)
3. **Simple goroutine pool**: No complex work stealing
4. **No job persistence**: Tasks lost on restart (acceptable for SOHO)
5. **Single instance**: No coordination needed

### Recommended Settings

```yaml
worker_pool:
  worker_count: 3          # For homelab CPU
  queue_size: 100          # Sufficient for typical load
  task_timeout: 5m         # Per-task timeout
  shutdown_timeout: 30s    # Graceful shutdown
```

### Resource Usage

- **Workers: 3**: ~3MB memory per worker
- **Queue: 100 tasks**: ~10MB memory
- **Total pool overhead**: ~20MB

## Integration Example

### Main Service Integration

```go
package main

func setupWorkerPool(cfg *config.Config) *worker.SimplePool {
    pool := worker.NewSimplePool(
        cfg.Service.MaxConcurrentWorkers,
        100, // queue size
    )

    pool.Start(context.Background())

    return pool
}

func submitDocumentTask(pool *worker.Pool, documentID string, handler TaskHandler) error {
    task := worker.NewDocumentTask(documentID, "process", handler)

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    return pool.Submit(ctx, task)
}
```

## Package Structure

```
internal/worker/
├── pool.go             # Worker pool implementation
├── task.go             # Task interface and types
├── document_task.go    # Document processing tasks
├── command_task.go     # Command processing tasks
├── retry.go            # Retry logic
├── metrics.go          # Performance metrics
└── worker_test.go      # Test suite
```

## Dependencies

- Standard library `context`, `sync`
- `github.com/rs/zerolog` - Logging
- `github.com/yourusername/outline-ai/internal/outline` - Outline models

---

**Status:** Ready for implementation
**Complexity:** Medium
**Priority:** High (core concurrency management)
