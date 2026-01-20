# Outline AI Assistant - Documentation Index

## Project Overview
A Go-based AI assistant service for Outline knowledge bases that provides:
- Command-driven document filing with interactive guidance
- Q&A system using workspace context
- Content enhancement (summaries, titles, search terms)
- Real-time processing via webhooks

## Documentation Structure

### 01_ARCHITECTURE/
High-level and low-level design documents.

| File | Size | Description |
|------|------|-------------|
| **2026-01-19_01_HLD.md** | 35K | **Primary High-Level Design** - Complete system architecture, components, flows, and examples |
| **2026-01-19_02_CHANGES.md** | 7.7K | Summary of all design changes and decisions |
| **2026-01-19_03_GUIDANCE_FEATURE.md** | 7.1K | Interactive guidance system for `/ai-file` command |
| **2026-01-19_04_WEBHOOK_FINDINGS.md** | 5.9K | Research findings on Outline webhook support |
| **2026-01-19_05_IDEMPOTENT_COMMANDS.md** | 11K | Idempotent design pattern for `/summarize` and search terms |
| **lld/** | 240K | **Low-level designs** - 12 detailed component designs (see [lld/README.md](01_ARCHITECTURE/lld/README.md)) |

### 02_BACKLOG/
Product backlog for tracking work.

- **epics/** - Epic-level planning (not yet populated)
- **stories/** - User stories (not yet populated)

### 03_WORKLOG/
Daily work logs and progress tracking (not yet populated).

## Quick Start Reading Guide

### For First-Time Readers
1. **Start here**: `2026-01-19_01_HLD.md` - Read sections 1-3 for overview
2. **Key features**: Read "Feature Subsystems" section (Command-Driven Filing, Q&A, Commands)
3. **Examples**: Review "Data Flow Examples" to see how it works

### For Implementation
1. **Architecture**: `2026-01-19_01_HLD.md` - Core components and design decisions
2. **Low-level designs**: `lld/README.md` - Detailed component designs with Go code structures
3. **Technical details**:
   - Webhooks: `2026-01-19_04_WEBHOOK_FINDINGS.md`
   - Idempotency: `2026-01-19_05_IDEMPOTENT_COMMANDS.md`
   - Guidance system: `2026-01-19_03_GUIDANCE_FEATURE.md`

### For Product/Management
1. **Changes summary**: `2026-01-19_02_CHANGES.md`
2. **Key benefits**: Read "Benefits" sections in CHANGES.md
3. **Features**: Read "Available Commands" in HLD.md

## Key Features Summary

### 1. Command-Driven Filing (`/ai-file [guidance]`)
- User explicitly triggers document filing when ready
- Optional guidance helps AI categorize ambiguous content
- Interactive feedback loop: Low confidence → `?ai-file` → User refines → Success
- Dual marker cleanup prevents re-processing

**Example:**
```markdown
/ai-file engineering documentation
```

### 2. Q&A System (`/ai`)
- Ask questions using workspace context
- AI searches relevant documents and synthesizes answers
- Responses delivered as comments with source citations
- Works on free-tier Outline (no Business/Enterprise needed)

**Example:**
```markdown
/ai What is our deployment process?
```

### 3. Content Enhancement
- **`/summarize`**: Generate/update document summary (idempotent)
- **`/enhance-title`**: Improve vague titles
- **`/related`**: Find and link related documents
- **Search terms**: Auto-generated keywords for better discovery

### 4. Real-Time Processing
- Webhook-based event processing (Outline `documents.update` events)
- < 1 second response time
- 99% reduction in API calls vs polling
- Optional polling fallback for local development

### 5. Idempotent Design
- Multiple runs of `/summarize` or filing cleanly replace previous content
- Hidden HTML comment markers track AI-generated sections
- Users can edit AI content (markers preserved) or take ownership (remove markers)
- No duplicate content accumulation

## Technology Stack

- **Language**: Go 1.21+
- **Database**: SQLite (embedded, GORM)
- **AI**: OpenAI-compatible API (configurable endpoint)
- **Configuration**: YAML with viper
- **Logging**: Zerolog (structured JSON)
- **Rate Limiting**: golang.org/x/time/rate
- **Deployment**: Kubernetes sidecar (primary), Docker standalone (secondary)

## Configuration Overview

```yaml
service:
  max_concurrent_workers: 3
  health_check_port: 8080
  webhook_port: 8081

outline:
  api_endpoint: "https://app.getoutline.com/api"
  api_key: "${OUTLINE_API_KEY}"
  webhook_secret: "${OUTLINE_WEBHOOK_SECRET}"

webhooks:
  enabled: true
  events: ["documents.update", "documents.create"]

ai:
  endpoint: "https://api.openai.com/v1"
  api_key: "${OPENAI_API_KEY}"
  model: "gpt-4"
  confidence_threshold: 0.7

commands:
  enabled: true
  available: ["/ai", "/ai-file", "/summarize", "/enhance-title", "/related"]
```

## Key Design Decisions

1. **Command-Driven vs Auto-Filing**: User control, no surprises
2. **Interactive Guidance Loop**: Transparency when AI uncertain
3. **Webhook-First**: Real-time, efficient (99% fewer API calls)
4. **Idempotent Commands**: Safe to re-run, clean results
5. **Hidden Markers**: Track AI content without cluttering UI
6. **Free-Tier Compatible**: Works on self-hosted Outline
7. **OpenAI-Compatible**: Any AI provider (OpenAI, Claude, local models)
8. **Kubernetes Sidecar**: Co-located with Outline pod for minimal latency

## Performance Characteristics

| Metric | Value |
|--------|-------|
| Response time | < 1 second (webhook) |
| API calls (idle) | ~0 per hour |
| API calls (active) | 1-10 per hour |
| Concurrent processing | 3 documents (configurable) |
| Confidence threshold | 0.7 (70%) |
| Webhook timeout | 5 seconds |

## Security Considerations

- SHA-256 HMAC webhook signature validation
- API keys loaded from environment variables
- Never log secrets (masked in logs)
- Content validation before AI processing
- Rate limiting on all endpoints
- HTTPS for all API communication

## Implementation Readiness

### Completed
- ✅ High-Level Design (HLD)
- ✅ Low-Level Designs (LLD) - All 12 components
- ✅ Design patterns documented (idempotency, webhooks, guidance)
- ✅ SOHO deployment considerations

### Ready to Start
1. **Set up Go project structure** following [lld/README.md](01_ARCHITECTURE/lld/README.md)
2. **Implement components** using TDD approach
3. **Test each component** (unit + integration tests)
4. **Deploy to homelab** using provided configurations

See `02_BACKLOG/` for sprint planning and user stories (to be populated).

## Document Naming Convention

All documents follow the pattern: `YYYY-MM-DD_NN_DESCRIPTION.md`
- `YYYY-MM-DD`: Date of creation
- `NN`: Sequence number (resets daily, starts at 01)
- `DESCRIPTION`: Brief description in UPPER_SNAKE_CASE

Example: `2026-01-19_01_HLD.md`

---

**Last Updated**: 2026-01-19
**Status**: Design Complete (HLD + 12 LLDs), Ready for Implementation
**Version**: 1.0
