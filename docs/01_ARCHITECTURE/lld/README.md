# Low-Level Designs (LLD) - Index

**Project:** Outline AI Assistant
**Last Updated:** 2026-01-19
**Target Deployment:** Homelab/SOHO

## Overview

This directory contains detailed low-level designs for each major component of the Outline AI Assistant. Each LLD provides:
- Type-safe Go data structures
- Implementation algorithms
- Error handling strategies
- Testing approaches
- SOHO deployment considerations

## Design Documents

### Core Infrastructure

| # | Document | Domain | Complexity | Priority | Status |
|---|----------|--------|------------|----------|--------|
| 01 | [Configuration System](01_configuration_system.md) | Config management | Low | High | ✅ Ready |
| 02 | [Persistence Layer](02_persistence_layer.md) | SQLite state storage | Medium | High | ✅ Ready |
| 03 | [Rate Limiting](03_rate_limiting.md) | API rate limiting | Low | High | ✅ Ready |

### API Clients

| # | Document | Domain | Complexity | Priority | Status |
|---|----------|--------|------------|----------|--------|
| 04 | [Outline API Client](04_outline_api_client.md) | Outline integration | Medium | High | ✅ Ready |
| 05 | [AI Client](05_ai_client.md) | OpenAI-compatible AI | Medium | High | ✅ Ready |

### Core Components

| # | Document | Domain | Complexity | Priority | Status |
|---|----------|--------|------------|----------|--------|
| 06 | [Taxonomy Builder](06_taxonomy_builder.md) | Collection taxonomy | Low | High | ✅ Ready |
| 07 | [Webhook Receiver](07_webhook_receiver.md) | Webhook processing | Medium | High | ✅ Ready |
| 08 | [Worker Pool](08_worker_pool.md) | Concurrent processing | Low | High | ✅ Ready |

### Feature Subsystems

| # | Document | Domain | Complexity | Priority | Status |
|---|----------|--------|------------|----------|--------|
| 09 | [Command System](09_command_system.md) | Command detection/routing | Medium | High | ✅ Ready |
| 10 | [Q&A System](10_qna_system.md) | Question answering | High | High | ✅ Ready |
| 11 | [Search Enhancement](11_search_enhancement.md) | Content enhancement | Medium | High | ✅ Ready |

### Service Orchestration

| # | Document | Domain | Complexity | Priority | Status |
|---|----------|--------|------------|----------|--------|
| 12 | [Main Service](12_main_service.md) | Service lifecycle | Low | High | ✅ Ready |

### AI Integration

| # | Document | Domain | Complexity | Priority | Status |
|---|----------|--------|------------|----------|--------|
| 13 | [AI Prompt Templates](13_ai_prompts.md) | Prompt engineering | Medium | High | ✅ Ready |

## Reading Guide

### For First-Time Implementation

**Recommended reading order:**

1. **Start with infrastructure** (essential foundations):
   - 01_configuration_system.md
   - 02_persistence_layer.md
   - 03_rate_limiting.md

2. **Build API clients** (external integrations):
   - 04_outline_api_client.md
   - 05_ai_client.md

3. **Core processing** (event handling):
   - 06_taxonomy_builder.md
   - 07_webhook_receiver.md
   - 08_worker_pool.md

4. **Feature subsystems** (main functionality):
   - 09_command_system.md
   - 10_qna_system.md
   - 11_search_enhancement.md

5. **Service orchestration** (putting it all together):
   - 12_main_service.md

### For Specific Features

- **Command-driven filing**: Read 06, 09, 11
- **Q&A functionality**: Read 10, 02 (deduplication)
- **Content enhancement**: Read 11 (idempotency pattern)
- **Webhook integration**: Read 07, 08
- **Deployment**: Read 12 (lifecycle management)

## Key Design Patterns

### Type Safety First

All LLDs use **strongly-typed Go structs** instead of `map[string]interface{}`:

```go
// ✅ Correct approach throughout LLDs
type Document struct {
    ID           string    `json:"id"`
    CollectionID string    `json:"collectionId"`
    Title        string    `json:"title"`
    Text         string    `json:"text"`
}

// ❌ Avoided (except for Outline's comment data field)
var doc map[string]interface{}
```

### Idempotent Operations

All content-generation operations use **hidden HTML comment markers** for clean replacement:

```markdown
<!-- AI-SUMMARY-START -->
> **Summary**: AI-generated summary here
<!-- AI-SUMMARY-END -->
```

**Benefits:**
- Multiple runs produce clean results
- User can take ownership by removing markers
- No duplicate content accumulation

**See:** 11_search_enhancement.md for full details

### Interactive Guidance Loop

Filing commands use **interactive feedback** when AI confidence is low:

```
/ai-file → AI uncertain → ?ai-file + comment → User provides guidance → /ai-file → Success
```

**See:** 09_command_system.md for implementation

### Webhook-First Architecture

Real-time event processing with fallback:

1. **Primary**: Webhooks (< 1 second response)
2. **Fallback**: Polling (local development)

**See:** 07_webhook_receiver.md for signature validation

## SOHO Deployment Optimizations

All LLDs include **homelab/SOHO considerations**:

1. **Single instance**: No distributed systems complexity
2. **Embedded SQLite**: No separate database server
3. **In-memory caching**: No Redis needed
4. **Simple backups**: File copy sufficient
5. **Conservative limits**: Free-tier API defaults

### Resource Requirements

**Minimum Configuration:**
- **CPU**: 1 core (2 cores recommended)
- **RAM**: 512MB minimum, 1GB recommended, 2GB comfortable
- **Disk**: 100MB for application + database growth (10MB/day typical)
- **Network**: Minimal bandwidth (~1MB/hour for API calls)

**Memory Breakdown (1GB deployment):**
- Application binary: ~20MB
- Worker pool (3 workers): ~20MB
- Taxonomy cache: ~5MB
- SQLite database: ~10MB
- Event queue: ~10MB
- Go runtime: ~50MB
- Buffer/overhead: ~100MB
- Available for processing: ~785MB

**Scaling Guidelines:**
- Each additional worker: +5-10MB RAM
- Larger queue size: +100KB per 100 tasks
- More collections: +1-2MB per 100 collections
- Webhook vs polling: No significant difference

**Performance Expectations:**
- Document processing: 2-5 seconds per document
- Webhook response: <100ms
- Q&A processing: 3-8 seconds (depends on AI latency)
- Taxonomy cache build: 5-15 seconds (50 collections)
- Database operations: <10ms per query

**Deployment Platforms:**
- Docker container: 512MB-1GB memory limit
- Raspberry Pi 4 (4GB): Comfortable
- VPS (1GB RAM): Suitable
- Home server: Ideal

## Component Dependencies

```
Main Service
├── Configuration System
├── Webhook Receiver
│   ├── Worker Pool
│   └── Command System
│       ├── Outline API Client
│       │   └── Rate Limiting
│       ├── AI Client
│       │   └── Rate Limiting
│       ├── Taxonomy Builder
│       │   └── Outline API Client
│       ├── Q&A System
│       │   ├── Outline API Client
│       │   ├── AI Client
│       │   └── Persistence Layer
│       └── Search Enhancement
│           ├── Outline API Client
│           └── AI Client
└── Persistence Layer
```

## Implementation Checklist

For each component:

- [ ] Read corresponding LLD
- [ ] Create package structure
- [ ] Define domain models (type-safe structs)
- [ ] Implement core logic
- [ ] Write unit tests (TDD)
- [ ] Write integration tests
- [ ] Document configuration options
- [ ] Test SOHO deployment scenario

## Testing Strategy

All LLDs include:
- **Unit tests**: Test individual functions
- **Integration tests**: Test component interactions
- **Mock implementations**: For external dependencies
- **Table-driven tests**: For multiple test cases

**Test requirements:**
- ALWAYS use timeouts: `go test -timeout 30s ./...`
- Test happy paths AND unhappy paths
- Test edge cases
- Verify idempotency (for content generation)

## Architecture Alignment

These LLDs implement the architecture defined in:
- [HLD](../2026-01-19_01_HLD.md) - Primary design
- [Guidance Feature](../2026-01-19_03_GUIDANCE_FEATURE.md) - Interactive filing
- [Webhook Findings](../2026-01-19_04_WEBHOOK_FINDINGS.md) - Webhook integration
- [Idempotent Commands](../2026-01-19_05_IDEMPOTENT_COMMANDS.md) - Idempotency pattern

## Package Organization

Proposed Go package structure:

```
internal/
├── config/          # 01 - Configuration System
├── persistence/     # 02 - Persistence Layer
├── ratelimit/       # 03 - Rate Limiting
├── outline/         # 04 - Outline API Client
├── ai/              # 05 - AI Client
├── taxonomy/        # 06 - Taxonomy Builder
├── webhooks/        # 07 - Webhook Receiver
├── worker/          # 08 - Worker Pool
├── commands/        # 09 - Command System
├── qna/             # 10 - Q&A System
├── enhancement/     # 11 - Search Enhancement
└── service/         # 12 - Main Service
```

**Import Paths:**
- All LLDs use placeholder: `github.com/yourusername/outline-ai`
- Replace with your actual module name when implementing
- Example: `github.com/mikekao/outline-ai` or `outline-ai` for local development
- Internal packages: `github.com/yourusername/outline-ai/internal/{package}`

## Code Quality Standards

All implementations must follow:

1. **Type safety**: Strongly-typed structs everywhere
2. **TDD**: Write tests first
3. **Idiomatic Go**: Follow Go conventions
4. **Error handling**: Proper error classification
5. **Context propagation**: Support cancellation
6. **Logging**: Structured logging with zerolog
7. **Documentation**: Clear godoc comments

### Naming Conventions

**Service vs Client Pattern:**
- Use **Client** suffix for external API wrappers: `outline.Client`, `ai.Client`
- Use **Service** suffix for internal business logic: `qna.Service`, `enhancement.Service`
- Use **Builder** suffix for construction/caching: `taxonomy.Builder`
- Use **Pool** suffix for resource management: `worker.Pool`
- Use **Receiver** suffix for event ingestion: `webhook.Receiver`

**Examples:**
```go
// External API clients
type Client interface { /* Outline API methods */ }
type OpenAIClient struct { /* AI provider implementation */ }

// Internal services
type Service interface { /* Business logic */ }
type DefaultService struct { /* Implementation */ }
```

### Error Handling Standards

**Error Wrapping:**
- Use consistent format: `"failed to {action}: %w"`
- Always preserve original error with `%w`
- Package prefix in error messages: `"persistence: record not found"`

**Examples:**
```go
// ✅ Correct
if err := storage.Save(ctx, data); err != nil {
    return fmt.Errorf("failed to save data: %w", err)
}

// ✅ Package-specific errors
var ErrNotFound = errors.New("persistence: record not found")

// ❌ Avoid
return fmt.Errorf("save failed") // loses context
return err // loses action context
```

### Context Handling

**Pattern:**
- Always accept `context.Context` as first parameter
- Create child contexts for timeouts: `ctx, cancel := context.WithTimeout(ctx, 30*time.Second)`
- Always defer `cancel()` immediately after context creation
- Check `ctx.Done()` in long-running loops

**Example:**
```go
func (s *Service) Process(ctx context.Context, id string) error {
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    select {
    case result := <-s.process(ctx, id):
        return result
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

### Logging Standards

**Message Format:**
- Use lowercase, no periods: `"processing document"` not `"Processing document."`
- Use structured fields: `.Str("document_id", id).Msg("processing document")`
- Log levels:
  - `Debug`: Detailed trace information
  - `Info`: Normal operations, state changes
  - `Warn`: Recoverable errors, degraded functionality
  - `Error`: Errors requiring attention

**Examples:**
```go
// ✅ Correct
log.Info().
    Str("document_id", doc.ID).
    Str("collection_id", doc.CollectionID).
    Msg("document processed successfully")

// ❌ Avoid
log.Info().Msg("Document processed.") // capital, period, no context
```

## Operational Considerations

### Schema Migration

**Initial Setup:**
```bash
# SQLite migrations handled by GORM AutoMigrate
# See 02_persistence_layer.md for schema details
```

**Schema Changes:**
- GORM AutoMigrate handles additive changes (new columns, indexes)
- For destructive changes (remove/rename columns), manual migration required
- Keep migrations simple for SOHO deployment
- Test migrations on backup copy first

**Migration Strategy:**
1. Backup database: `cp state.db state.db.backup`
2. Apply migration via AutoMigrate or manual ALTER TABLE
3. Verify data integrity
4. Remove backup after validation

### Backup and Restore

**Automated Backups:**
```yaml
persistence:
  backup_enabled: true
  backup_interval: 24h
```

**Manual Backup:**
```bash
# Database
cp /data/state.db /backups/state-$(date +%Y%m%d).db

# Configuration
cp /config.yaml /backups/config-$(date +%Y%m%d).yaml
```

**Restore Process:**
1. Stop service
2. Replace database file
3. Verify database integrity: `sqlite3 state.db "PRAGMA integrity_check;"`
4. Restart service

**Backup Retention:**
- Daily: Keep 7 days
- Weekly: Keep 4 weeks
- Monthly: Keep 12 months

### Security Checklist

**Secrets Management:**
- [ ] Store API keys in environment variables
- [ ] Never commit secrets to version control
- [ ] Use `.env` file for local development (git-ignored)
- [ ] Rotate API keys periodically

**Network Security:**
- [ ] Use HTTPS for public webhook endpoint
- [ ] Validate webhook signatures (HMAC)
- [ ] Rate limit webhook endpoint (if using reverse proxy)
- [ ] Restrict health check endpoint to internal network

**Database Security:**
- [ ] Restrict file permissions: `chmod 600 state.db`
- [ ] Enable SQLite encryption (optional, for sensitive data)
- [ ] Encrypt backups if stored in cloud

**Service Security:**
- [ ] Run as non-root user
- [ ] Use read-only filesystem where possible
- [ ] Limit resource usage (cgroups/Docker limits)
- [ ] Monitor for unusual API usage patterns

### Monitoring and Troubleshooting

**Health Checks:**
```bash
# Service health
curl http://localhost:8080/health

# Webhook health
curl http://localhost:8081/health
```

**Common Issues:**

1. **Webhooks not received**: Check firewall, public URL, signature validation
2. **High memory usage**: Reduce worker count, check for memory leaks
3. **Slow processing**: Check AI provider latency, rate limits
4. **Database locked**: Check for concurrent access, reduce connection pool
5. **Circuit breaker open**: Check AI provider status, increase threshold

**Logging:**
```yaml
logging:
  level: "info"  # debug for troubleshooting
  format: "json"  # or "pretty" for human reading
```

**Metrics to Monitor:**
- Worker pool queue size
- API rate limit usage
- Database size growth
- Error rates by component
- Average processing time

## Next Steps

1. **Review** all LLDs to understand system design
2. **Prioritize** implementation order (see reading guide)
3. **Set up** project structure (Go module, packages)
4. **Implement** components following TDD
5. **Test** each component thoroughly
6. **Integrate** components step by step
7. **Deploy** to homelab/SOHO environment
8. **Configure** monitoring and backups
9. **Validate** security checklist
10. **Document** deployment-specific configurations

## Questions?

For clarifications or design discussions:
- Review parent [HLD](../2026-01-19_01_HLD.md)
- Check specific architecture documents
- Verify alignment with SOHO deployment constraints

---

**Status:** All LLDs complete and ready for implementation
**Complexity:** Ranges from Low to High per component
**Priority:** All High priority (core functionality)
