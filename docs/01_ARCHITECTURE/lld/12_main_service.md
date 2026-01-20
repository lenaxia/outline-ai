# Low-Level Design: Main Service Orchestration

**Domain:** Service Lifecycle Management
**Status:** Design
**Last Updated:** 2026-01-19
**Target Deployment:** Homelab/SOHO

## Purpose

Orchestrate all components, manage service lifecycle, handle graceful shutdown, and provide health monitoring for the Outline AI Assistant.

## Design Principles

1. **Fail Fast**: Validate everything at startup
2. **Graceful Shutdown**: Clean up resources properly
3. **Observable**: Health checks and metrics
4. **Resilient**: Handle component failures gracefully
5. **SOHO Optimized**: Single-instance deployment

## Service Architecture

### Main Components

```go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/yourusername/outline-ai/internal/ai"
    "github.com/yourusername/outline-ai/internal/command"
    "github.com/yourusername/outline-ai/internal/config"
    "github.com/yourusername/outline-ai/internal/enhancement"
    "github.com/yourusername/outline-ai/internal/outline"
    "github.com/yourusername/outline-ai/internal/persistence"
    "github.com/yourusername/outline-ai/internal/qna"
    "github.com/yourusername/outline-ai/internal/ratelimit"
    "github.com/yourusername/outline-ai/internal/taxonomy"
    "github.com/yourusername/outline-ai/internal/webhook"
    "github.com/yourusername/outline-ai/internal/worker"
    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
)

type Service struct {
    config *config.Config

    // Core components
    outlineClient   outline.Client
    aiClient        ai.Client
    storage         persistence.Storage
    taxonomyBuilder taxonomy.Builder

    // Processing components
    workerPool       *worker.SimplePool
    commandProcessor command.Processor
    qnaService       qna.Service
    enhancementService enhancement.Service

    // Event sources
    webhookReceiver *webhook.HTTPReceiver

    // Health monitoring
    healthServer *HealthServer

    // Lifecycle
    ctx    context.Context
    cancel context.CancelFunc
}
```

## Initialization

### Service Initialization

```go
package main

func NewService(configPath string) (*Service, error) {
    // Load configuration
    cfg, err := config.Load(configPath)
    if err != nil {
        return nil, fmt.Errorf("failed to load config: %w", err)
    }

    // Setup logging
    setupLogging(cfg.Logging)

    log.Info().
        Str("version", Version).
        Str("commit", GitCommit).
        Msg("Starting Outline AI Assistant")

    // Validate connectivity
    if err := config.ValidateConnectivity(context.Background(), cfg); err != nil {
        return nil, fmt.Errorf("connectivity validation failed: %w", err)
    }

    // Initialize components
    svc := &Service{
        config: cfg,
    }

    if err := svc.initializeComponents(); err != nil {
        return nil, fmt.Errorf("component initialization failed: %w", err)
    }

    log.Info().Msg("Service initialized successfully")

    return svc, nil
}

func (s *Service) initializeComponents() error {
    // Initialize rate limiters
    outlineLimiter := ratelimit.NewTokenBucketLimiter(
        s.config.Outline.RateLimitPerMinute,
    )

    // Initialize Outline client
    s.outlineClient = outline.NewHTTPClient(
        s.config.Outline.APIEndpoint,
        s.config.Outline.APIKey,
        outlineLimiter,
    )

    // Initialize AI client
    aiClient, err := ai.NewOpenAIClient(
        s.config.AI.Endpoint,
        s.config.AI.APIKey,
        s.config.AI.Model,
        s.config.AI.MaxTokens,
        s.config.AI.RequestTimeout,
    )
    if err != nil {
        return fmt.Errorf("failed to create AI client: %w", err)
    }
    s.aiClient = aiClient

    // Initialize persistence
    storage, err := persistence.NewSQLiteStorage(
        s.config.Persistence.DatabasePath,
        logger.Info,
    )
    if err != nil {
        return fmt.Errorf("failed to create storage: %w", err)
    }
    s.storage = storage

    // Initialize taxonomy builder
    s.taxonomyBuilder = taxonomy.NewCachedBuilder(
        s.outlineClient,
        s.config.Taxonomy.CacheTTL,
        s.config.Taxonomy.MaxSamplesPerCollection,
        s.config.Taxonomy.IncludeSampleDocuments,
    )

    // Initialize worker pool
    s.workerPool = worker.NewSimplePool(
        s.config.Service.MaxConcurrentWorkers,
        100, // queue size
    )

    // Initialize Q&A service
    searcher := qna.NewRelevanceSearcher(s.outlineClient)
    s.qnaService = qna.NewDefaultService(
        s.aiClient,
        s.outlineClient,
        s.storage,
        searcher,
        s.config.QnA.MaxContextDocuments,
    )

    // Initialize enhancement service
    s.enhancementService = enhancement.NewDefaultService(
        s.aiClient,
        s.outlineClient,
        enhancement.Config{
            RespectUserOwnership: s.config.Enhancement.RespectUserOwnership,
            IdempotentUpdates:    s.config.Enhancement.IdempotentUpdates,
        },
    )

    // Initialize command system
    if err := s.initializeCommandSystem(); err != nil {
        return fmt.Errorf("failed to initialize command system: %w", err)
    }

    // Initialize webhook receiver
    if s.config.Webhooks.Enabled {
        if err := s.initializeWebhooks(); err != nil {
            return fmt.Errorf("failed to initialize webhooks: %w", err)
        }
    }

    // Initialize health server
    s.healthServer = NewHealthServer(
        s.config.Service.HealthCheckPort,
        s,
    )

    return nil
}

func (s *Service) initializeCommandSystem() error {
    // Create command detector
    detector := command.NewRegexDetector()

    // Create command router
    router := command.NewRouter()

    // Register handlers
    if s.config.Commands.Enabled {
        // /ai handler
        aiHandler := command.NewAIQuestionHandler(
            s.aiClient,
            s.outlineClient,
            qna.NewRelevanceSearcher(s.outlineClient),
        )
        router.RegisterHandler(aiHandler)

        // /ai-file handler
        fileHandler := command.NewAIFileHandler(
            s.aiClient,
            s.outlineClient,
            s.taxonomyBuilder,
            s.config.AI.ConfidenceThreshold,
        )
        router.RegisterHandler(fileHandler)

        // /summarize handler
        summarizeHandler := command.NewSummarizeHandler(
            s.aiClient,
            s.outlineClient,
        )
        router.RegisterHandler(summarizeHandler)

        // Add more handlers as needed
    }

    // Create command processor
    s.commandProcessor = command.NewDefaultProcessor(
        detector,
        router,
        s.outlineClient,
    )

    return nil
}

func (s *Service) initializeWebhooks() error {
    s.webhookReceiver = webhook.NewHTTPReceiver(
        s.config.Webhooks.Port,
        s.config.Outline.WebhookSecret,
        1000, // queue size
    )

    // Register document event handler
    docHandler := webhook.NewDocumentEventHandler(
        s.outlineClient,
        s.commandProcessor,
    )
    s.webhookReceiver.RegisterHandler("documents.update", docHandler)
    s.webhookReceiver.RegisterHandler("documents.create", docHandler)

    return nil
}
```

## Service Lifecycle

### Start and Stop

```go
package main

func (s *Service) Start() error {
    s.ctx, s.cancel = context.WithCancel(context.Background())

    log.Info().Msg("Starting service components")

    // Start worker pool
    s.workerPool.Start(s.ctx)

    // Start webhook receiver if enabled
    if s.config.Webhooks.Enabled {
        go func() {
            if err := s.webhookReceiver.Start(s.ctx); err != nil {
                log.Error().Err(err).Msg("Webhook receiver failed")
            }
        }()
    }

    // Start fallback polling if enabled
    if s.config.Webhooks.FallbackPolling.Enabled {
        go s.startFallbackPolling(s.ctx)
    }

    // Start background tasks
    go s.startBackgroundTasks(s.ctx)

    // Start health server
    go func() {
        if err := s.healthServer.Start(s.ctx); err != nil {
            log.Error().Err(err).Msg("Health server failed")
        }
    }()

    log.Info().Msg("Service started successfully")

    // Wait for shutdown signal
    s.waitForShutdown()

    return nil
}

func (s *Service) Stop(ctx context.Context) error {
    log.Info().Msg("Stopping service")

    // Cancel context to signal all goroutines
    s.cancel()

    // Stop health server
    if err := s.healthServer.Stop(ctx); err != nil {
        log.Warn().Err(err).Msg("Error stopping health server")
    }

    // Stop webhook receiver
    if s.webhookReceiver != nil {
        if err := s.webhookReceiver.Stop(ctx); err != nil {
            log.Warn().Err(err).Msg("Error stopping webhook receiver")
        }
    }

    // Stop worker pool
    if err := s.workerPool.Stop(ctx); err != nil {
        log.Warn().Err(err).Msg("Error stopping worker pool")
    }

    // Close storage
    if err := s.storage.Close(); err != nil {
        log.Warn().Err(err).Msg("Error closing storage")
    }

    log.Info().Msg("Service stopped")

    return nil
}

func (s *Service) waitForShutdown() {
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

    sig := <-sigChan
    log.Info().
        Str("signal", sig.String()).
        Msg("Shutdown signal received")

    // Create shutdown context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := s.Stop(ctx); err != nil {
        log.Error().Err(err).Msg("Error during shutdown")
    }
}
```

## Background Tasks

### Periodic Tasks

```go
package main

func (s *Service) startBackgroundTasks(ctx context.Context) {
    log.Info().Msg("Starting background tasks")

    // Taxonomy cache warmup
    if s.config.Taxonomy.CacheTTL > 0 {
        go taxonomy.StartCacheWarmup(ctx, s.taxonomyBuilder, taxonomy.WarmupConfig{
            Enabled:  true,
            Interval: s.config.Taxonomy.CacheTTL / 2,
        })
    }

    // Database cleanup
    if s.config.Persistence.BackupEnabled {
        go persistence.StartCleanupRoutine(ctx, s.storage, persistence.CleanupConfig{
            QuestionStateRetention: 30 * 24 * time.Hour, // 30 days
            CleanupInterval:        24 * time.Hour,       // Daily
        })
    }

    // Database backup
    if s.config.Persistence.BackupEnabled {
        go persistence.StartBackupRoutine(ctx, s.storage, persistence.BackupConfig{
            Enabled:         true,
            Interval:        s.config.Persistence.BackupInterval,
            BackupDirectory: "/data/backups",
            MaxBackups:      7,
        })
    }
}

func (s *Service) startFallbackPolling(ctx context.Context) {
    log.Info().
        Dur("interval", s.config.Webhooks.FallbackPolling.Interval).
        Msg("Starting fallback polling")

    ticker := time.NewTicker(s.config.Webhooks.FallbackPolling.Interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            log.Info().Msg("Fallback polling stopped")
            return
        case <-ticker.C:
            if err := s.pollForCommands(ctx); err != nil {
                log.Error().Err(err).Msg("Polling failed")
            }
        }
    }
}

func (s *Service) pollForCommands(ctx context.Context) error {
    log.Debug().Msg("Polling for commands")

    // Search for command markers
    commandPatterns := []string{"/ai ", "/ai-file", "/summarize", "/enhance-title"}

    for _, pattern := range commandPatterns {
        results, err := s.outlineClient.SearchDocuments(ctx, pattern, &outline.SearchOptions{
            Limit: 10,
        })
        if err != nil {
            log.Warn().
                Err(err).
                Str("pattern", pattern).
                Msg("Search failed")
            continue
        }

        // Process each document
        for _, doc := range results.Documents {
            // Submit to worker pool
            task := worker.NewDocumentTask(doc.ID, "command", &DocumentTaskHandler{
                processor: s.commandProcessor,
            })

            if err := s.workerPool.Submit(ctx, task); err != nil {
                log.Warn().
                    Err(err).
                    Str("document_id", doc.ID).
                    Msg("Failed to submit task")
            }
        }
    }

    return nil
}

type DocumentTaskHandler struct {
    processor command.Processor
}

func (h *DocumentTaskHandler) Handle(ctx context.Context, documentID string) error {
    return h.processor.ProcessDocument(ctx, documentID)
}
```

## Health Monitoring

### Health Server

```go
package main

import (
    "context"
    "encoding/json"
    "net/http"
    "time"
)

type HealthServer struct {
    server  *http.Server
    service *Service
}

func NewHealthServer(port int, service *Service) *HealthServer {
    mux := http.NewServeMux()

    hs := &HealthServer{
        service: service,
    }

    mux.HandleFunc("/health", hs.handleHealth)
    mux.HandleFunc("/ready", hs.handleReady)
    mux.HandleFunc("/metrics", hs.handleMetrics)

    hs.server = &http.Server{
        Addr:    fmt.Sprintf(":%d", port),
        Handler: mux,
    }

    return hs
}

func (hs *HealthServer) Start(ctx context.Context) error {
    log.Info().
        Str("addr", hs.server.Addr).
        Msg("Starting health server")

    if err := hs.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        return err
    }

    return nil
}

func (hs *HealthServer) Stop(ctx context.Context) error {
    return hs.server.Shutdown(ctx)
}

func (hs *HealthServer) handleHealth(w http.ResponseWriter, r *http.Request) {
    health := map[string]interface{}{
        "status":    "healthy",
        "timestamp": time.Now(),
        "version":   Version,
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(health)
}

func (hs *HealthServer) handleReady(w http.ResponseWriter, r *http.Request) {
    // Check if service is ready
    ready := hs.checkReadiness()

    if ready {
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "status": "ready",
        })
    } else {
        w.WriteHeader(http.StatusServiceUnavailable)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "status": "not ready",
        })
    }
}

func (hs *HealthServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
    metrics := hs.collectMetrics()

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(metrics)
}

func (hs *HealthServer) checkReadiness() bool {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // Check Outline API
    if err := hs.service.outlineClient.Ping(ctx); err != nil {
        log.Warn().Err(err).Msg("Outline API not ready")
        return false
    }

    // Check AI API
    if err := hs.service.aiClient.Ping(ctx); err != nil {
        log.Warn().Err(err).Msg("AI API not ready")
        return false
    }

    // Check storage
    if err := hs.service.storage.Ping(ctx); err != nil {
        log.Warn().Err(err).Msg("Storage not ready")
        return false
    }

    return true
}

func (hs *HealthServer) collectMetrics() map[string]interface{} {
    poolStats := hs.service.workerPool.GetStats()
    taxonomyStats := hs.service.taxonomyBuilder.GetCacheStats()

    metrics := map[string]interface{}{
        "worker_pool": map[string]interface{}{
            "workers_count":     poolStats.WorkersCount,
            "active_workers":    poolStats.ActiveWorkers,
            "queued_tasks":      poolStats.QueuedTasks,
            "completed_tasks":   poolStats.CompletedTasks,
            "failed_tasks":      poolStats.FailedTasks,
            "average_exec_time": poolStats.AverageExecTime,
        },
        "taxonomy_cache": map[string]interface{}{
            "hits":            taxonomyStats.Hit,
            "misses":          taxonomyStats.Miss,
            "last_build_time": taxonomyStats.LastBuildTime,
            "expires_at":      taxonomyStats.ExpiresAt,
        },
    }

    if hs.service.webhookReceiver != nil {
        webhookStats := hs.service.webhookReceiver.GetStats()
        metrics["webhooks"] = map[string]interface{}{
            "total_received":         webhookStats.TotalReceived,
            "valid_signatures":       webhookStats.ValidSignatures,
            "invalid_signatures":     webhookStats.InvalidSignatures,
            "processed_successfully": webhookStats.ProcessedSuccessfully,
            "processing_failed":      webhookStats.ProcessingFailed,
            "last_event_time":        webhookStats.LastEventTime,
        }
    }

    return metrics
}
```

## Logging Setup

### Structured Logging

```go
package main

func setupLogging(cfg config.LoggingConfig) {
    // Set log level
    level, err := zerolog.ParseLevel(cfg.Level)
    if err != nil {
        level = zerolog.InfoLevel
    }
    zerolog.SetGlobalLevel(level)

    // Configure output
    if cfg.Format == "pretty" {
        log.Logger = log.Output(zerolog.ConsoleWriter{
            Out:        os.Stdout,
            TimeFormat: time.RFC3339,
        })
    } else {
        // JSON format (default)
        log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
    }

    // Add caller information for debug level
    if level == zerolog.DebugLevel {
        log.Logger = log.Logger.With().Caller().Logger()
    }
}
```

## Main Entry Point

### Main Function

```go
package main

import (
    "flag"
    "os"
)

var (
    Version   = "dev"
    GitCommit = "unknown"
    BuildTime = "unknown"
)

func main() {
    configPath := flag.String("config", "config.yaml", "Path to configuration file")
    version := flag.Bool("version", false, "Show version information")
    flag.Parse()

    if *version {
        fmt.Printf("Outline AI Assistant\n")
        fmt.Printf("Version: %s\n", Version)
        fmt.Printf("Commit: %s\n", GitCommit)
        fmt.Printf("Built: %s\n", BuildTime)
        os.Exit(0)
    }

    // Create service
    service, err := NewService(*configPath)
    if err != nil {
        log.Fatal().Err(err).Msg("Failed to create service")
    }

    // Start service
    if err := service.Start(); err != nil {
        log.Fatal().Err(err).Msg("Service failed")
    }
}
```

## Testing Strategy

### Integration Tests

```go
func TestService_StartStop(t *testing.T)
func TestService_ComponentInitialization(t *testing.T)
func TestService_HealthCheck(t *testing.T)
func TestService_GracefulShutdown(t *testing.T)
func TestService_PollingFallback(t *testing.T)
func TestService_BackgroundTasks(t *testing.T)
```

## Deployment

### Docker Container

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=1 GOOS=linux go build -o outline-ai cmd/outline-ai/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates sqlite

WORKDIR /app
COPY --from=builder /app/outline-ai .

EXPOSE 8080 8081

CMD ["./outline-ai", "--config", "/config/config.yaml"]
```

### Docker Compose

```yaml
version: '3.8'

services:
  outline-ai:
    build: .
    ports:
      - "8080:8080"  # Health check
      - "8081:8081"  # Webhooks
    volumes:
      - ./config.yaml:/config/config.yaml
      - ./data:/data
    environment:
      - OUTLINE_API_KEY=${OUTLINE_API_KEY}
      - OUTLINE_WEBHOOK_SECRET=${OUTLINE_WEBHOOK_SECRET}
      - OPENAI_API_KEY=${OPENAI_API_KEY}
    restart: unless-stopped
```

### Systemd Service

```ini
[Unit]
Description=Outline AI Assistant
After=network.target

[Service]
Type=simple
User=outline-ai
WorkingDirectory=/opt/outline-ai
ExecStart=/opt/outline-ai/outline-ai --config /etc/outline-ai/config.yaml
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

## SOHO Deployment Considerations

### Simplifications for Homelab

1. **Single instance**: No load balancing or HA
2. **Local SQLite**: No distributed database
3. **File-based config**: No config server
4. **Simple logging**: Stdout/file, no log aggregation
5. **Basic metrics**: No Prometheus/Grafana integration

### Resource Requirements

- **CPU**: 1-2 cores
- **Memory**: 512MB - 1GB
- **Storage**: 100MB (+ database growth)
- **Network**: Minimal (API calls only)

## Package Structure

```
cmd/outline-ai/
└── main.go             # Entry point

internal/
├── service/
│   ├── service.go      # Main service
│   ├── health.go       # Health server
│   ├── lifecycle.go    # Start/stop
│   └── tasks.go        # Background tasks
```

## Dependencies

All internal packages plus:
- `github.com/rs/zerolog` - Logging
- `github.com/spf13/viper` - Config (via config package)

---

**Status:** Ready for implementation
**Complexity:** High (orchestrates all components)
**Priority:** High (required for service execution)
