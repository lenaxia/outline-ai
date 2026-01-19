# Outline AI Assistant - LLM Implementation Guide

**Version:** 1.0
**Last Updated:** 2026-01-19
**Project Status:** Design Complete, Ready for Implementation

---

## Table of Contents

1. [Project Overview](#project-overview)
2. [Critical Guidelines & Hard Rules](#critical-guidelines--hard-rules)
3. [Repository Structure](#repository-structure)
4. [Architecture Overview](#architecture-overview)
5. [Technology Stack](#technology-stack)
6. [Development Workflow](#development-workflow)
7. [Common Commands](#common-commands)
8. [Branch Management](#branch-management)
9. [Documentation Standards](#documentation-standards)
10. [Testing Requirements](#testing-requirements)

---

## Project Overview

Outline AI Assistant is a **Go-based service providing AI-powered knowledge management for Outline**, designed for real-time document processing via webhooks. This project is **100% LLM-implemented** with significant human-in-the-loop oversight.

**Core Principles:**
- Command-driven operations (user control)
- Webhook-first architecture (real-time, efficient)
- Idempotent operations (safe to re-run)
- Works on free-tier Outline (self-hosted compatible)
- OpenAI-compatible (any AI provider)
- **Kubernetes sidecar deployment** (primary)
- Single binary, minimal dependencies

**Primary Source Documents:**
- [`docs/01_ARCHITECTURE/2026-01-19_01_HLD.md`](docs/01_ARCHITECTURE/2026-01-19_01_HLD.md) - Authoritative high-level design
- [`docs/01_ARCHITECTURE/`](docs/01_ARCHITECTURE/) - All architecture documentation
- [`docs/02_BACKLOG/`](docs/02_BACKLOG/) - Sprint stories and epics
- [`docs/03_WORKLOG/`](docs/03_WORKLOG/) - Progress updates and handoffs

**Key Features:**
1. **Command-Driven Filing** (`/ai-file [guidance]`) - AI categorizes and files documents
2. **Interactive Q&A** (`/ai [question]`) - Answers questions using workspace context
3. **Content Enhancement** (`/summarize`, `/enhance-title`, `/related`) - Improves document quality
4. **Idempotent Operations** - Multiple runs produce clean results (hidden marker pattern)
5. **Interactive Guidance Loop** - Low confidence → `?ai-file` → user refines → success

---

## Critical Guidelines & Hard Rules

### 0. Test Driven Development (TDD)

**MANDATORY:** Write tests BEFORE writing functional code. ALWAYS.

```go
// ✅ Correct workflow:
// 1. Write test
// 2. Run test (should fail)
// 3. Write minimal code to pass
// 4. Run test (should pass)
// 5. Refactor if needed
```

**Test Requirements:**
- Multiple happy path tests
- Multiple unhappy path tests
- Edge case coverage
- ALWAYS use timeouts when running tests to detect hangs
- Tests must pass before marking work complete

### 1. Type Safety First

**ALWAYS DO ✅:**
- Define strongly-typed structs for ALL data structures
- Create domain types for related fields
- Export structs from packages for reuse

**NEVER DO ❌:**
- NEVER use `map[string]interface{}` for structured data
- NEVER use `interface{}` when you know the type
- NEVER use type assertions when you can use generics
- NEVER pass untyped data between functions

**When Maps Are Acceptable:**

ONLY use `map[string]interface{}` when:
1. Parsing external JSON/YAML with unknown structure (convert to struct immediately)
2. Interacting with reflection-based libraries (convert to struct ASAP)
3. Truly dynamic configuration where structure is unknowable

**Example:**
```go
// ✅ Parse webhook payload to struct immediately
func parseWebhookEvent(data []byte) (*DocumentUpdateEvent, error) {
    var raw map[string]interface{}
    if err := json.Unmarshal(data, &raw); err != nil {
        return nil, err
    }

    event := &DocumentUpdateEvent{
        EventType: raw["event"].(string),
        DocumentID: raw["model"].(map[string]interface{})["id"].(string),
        ActorID: raw["actorId"].(string),
    }

    return event, nil  // Return typed struct, not map
}
```

**Justification:**
1. Compile-time verification - Catch errors before runtime
2. IDE autocomplete - Developer productivity
3. Refactoring safety - Rename propagates everywhere
4. Self-documenting - Struct shows what's available
5. Performance - No reflection, no runtime type checking

### 2. Idiomatic Go

- Follow Go conventions, not patterns from other languages
- Use Go's multiple return values (value, error) pattern
- Avoid global state and exceptions
- Create custom error types for domain-specific errors
- Prefer minimal or no concurrency when possible
- Use context for cancellation and timeouts

### 3. Explicit Over Implicit

- Go favors explicit error handling
- Explicit type declarations
- No magic or hidden behavior
- Make dependencies explicit (dependency injection)

### 4. Concurrency Guidelines

**For This Project:**
- Worker pool pattern for document processing (controlled parallelism)
- Goroutines for webhook processing (non-blocking responses)
- Context for cancellation and timeout propagation
- Always consider synchronization and race conditions
- Use `sync.WaitGroup` for tracking goroutines
- Use buffered channels as semaphores for concurrency control

**When to Add Concurrency:**
- Processing multiple documents independently
- Long-running operations (AI calls) that shouldn't block webhooks
- Background tasks (taxonomy cache refresh, state cleanup)

**When NOT to Add Concurrency:**
- Simple request/response flows
- Database operations (SQLite has limited concurrency)
- Configuration loading
- Logging operations

### 5. Communication Tone

**MANDATORY TONE RULES:**

- Always be neutral, factual, objective
- Do NOT be sensational, overly agreeable, or a sycophant
- Don't be a cheerleader, be a critical collaborator
- Never agree with something just because the user stated it
- Always validate statements and ensure you have meaningful and solid proof before making claims
- Provide honest and objective feedback
- If you agree, do so based on evidence or sound reasoning, not to please

### 6. Code Quality

- No comments unless necessary and makes sense
- If you leave comments, make them timeless (won't be outdated)
- Comments get outdated and mislead LLMs
- Code should be self-documenting through clear naming
- If you see incorrect comments, either remove or update them

### 7. Technical Debt

**ZERO TOLERANCE:**
- Do NOT create adapters for backwards compatibility
- Always remove legacy code
- Implement full final implementation
- Do NOT accrue technical debt
- Do NOT use weird hacks to get tests to pass
- Do the work needed to fix code or tests properly

### 8. Uncertainty Protocol

**If uncertain about proper behavior: ASK THE USER**

Do not guess, assume, or implement workarounds.

### 9. Tools are production code

If you create a script or tool, you MUST create it using TDD and
ensure proper and high test coverage. Never assume a script works
unless you have demonstrable proof with passing tests. If a script
or tool lacks tests, assume it is broken until proven otherwise.

### 10. Understand the architecture

Make sure you understand the entire architecture and how the changes
you are about to make fit within that architecture. Understand the
goals of what you are trying to achieve and how to go about it.

ALWAYS review the HLD and relevant design docs:
- `docs/01_ARCHITECTURE/2026-01-19_01_HLD.md` - Primary design
- `docs/01_ARCHITECTURE/` - All architecture documentation

### 11. Idempotency by Design

**CRITICAL for this project:**
- `/summarize` and search terms MUST use hidden HTML comment markers
- Multiple runs MUST produce clean results (replace, not append)
- Marker pattern: `<!-- AI-SUMMARY-START -->` ... `<!-- AI-SUMMARY-END -->`
- Respect user ownership: if markers removed, skip updates
- See `docs/01_ARCHITECTURE/2026-01-19_05_IDEMPOTENT_COMMANDS.md`

### 12. Webhook-First Architecture

**CRITICAL for this project:**
- Primary event detection: Outline webhooks (`documents.update`)
- Fallback: Polling mode (local development only)
- MUST respond to webhooks within 5 seconds (use worker queue)
- MUST validate webhook signatures (SHA-256 HMAC)
- See `docs/01_ARCHITECTURE/2026-01-19_04_WEBHOOK_FINDINGS.md`

---

## Repository Structure

```
outline-ai-assistant/
├── README.md                    # User-facing project README
├── README-LLM.md               # This file - LLM implementation guide
├── go.mod                      # Go module definition
├── go.sum                      # Go dependency checksums
│
├── cmd/                        # Application entrypoints
│   └── outline-autofiler/      # Main service application
│       └── main.go
│
├── internal/                   # Private application code
│   ├── config/                 # Configuration management
│   ├── models/                 # Domain models (structs)
│   ├── persistence/            # State persistence (SQLite)
│   │   ├── interface.go        # Storage interface
│   │   └── sqlite.go           # SQLite implementation
│   ├── outline/                # Outline API client
│   │   ├── client.go           # HTTP client with rate limiting
│   │   └── client_test.go
│   ├── ai/                     # AI client (OpenAI-compatible)
│   │   ├── client.go           # AI client with circuit breaker
│   │   └── client_test.go
│   ├── ratelimit/              # Rate limiting infrastructure
│   │   └── limiter.go          # Token bucket rate limiter
│   ├── taxonomy/               # Collection taxonomy builder
│   │   └── builder.go          # Taxonomy with caching
│   ├── worker/                 # Worker pool
│   │   └── pool.go             # Concurrent processing
│   ├── processor/              # Document processing orchestrator
│   │   ├── processor.go        # Main processing pipeline
│   │   └── processor_test.go
│   ├── webhooks/               # Webhook receiver
│   │   ├── receiver.go         # HTTP endpoint + validation
│   │   └── receiver_test.go
│   ├── commands/               # Command system
│   │   ├── router.go           # Command dispatcher
│   │   ├── handlers.go         # Command handlers
│   │   └── handlers_test.go
│   ├── qna/                    # Q&A system
│   │   ├── detector.go         # Question detection
│   │   ├── search.go           # Workspace search
│   │   ├── answerer.go         # Answer generation
│   │   └── delivery.go         # Answer delivery
│   ├── service/                # Main service loop
│   │   ├── service.go          # Service coordination
│   │   └── service_test.go
│   ├── server/                 # HTTP server
│   │   └── server.go           # Health check endpoints
│   └── logging/                # Structured logging
│       └── logger.go           # Zerolog setup
│
├── pkg/                        # Public/reusable packages
│   └── (future shared utilities)
│
├── docs/                       # Documentation
│   ├── README.md               # Documentation index
│   ├── 01_ARCHITECTURE/        # High-level designs
│   │   ├── 2026-01-19_01_HLD.md              # Primary design (35K)
│   │   ├── 2026-01-19_02_CHANGES.md          # Changes summary
│   │   ├── 2026-01-19_03_GUIDANCE_FEATURE.md # Interactive guidance
│   │   ├── 2026-01-19_04_WEBHOOK_FINDINGS.md # Webhook research
│   │   ├── 2026-01-19_05_IDEMPOTENT_COMMANDS.md # Idempotency pattern
│   │   └── lld/                               # Low-level designs
│   ├── 02_BACKLOG/             # Sprint stories and epics
│   │   ├── epics/
│   │   └── stories/
│   └── 03_WORKLOG/             # Progress updates and handoffs
│
├── k8s/                        # Kubernetes manifests
│   ├── deployment-sidecar.yaml # Sidecar deployment example
│   ├── deployment-standalone.yaml # Standalone deployment
│   ├── configmap.yaml          # ConfigMap for config.yaml
│   ├── secret.yaml.example     # Secret template
│   ├── service.yaml            # Service for webhooks
│   └── pvc.yaml                # PersistentVolumeClaim for state
│
├── config.yaml.example         # Example configuration
├── .env.example                # Example environment variables
├── .gitignore                  # Git ignore patterns
├── Dockerfile                  # Container image
├── docker-compose.yaml         # Docker Compose for local dev
├── Makefile                    # Build targets
└── tests/                      # Integration tests
    └── e2e/
```

**Key Principles:**
- Every major folder has a README.md
- READMEs are the first thing to read when entering a folder
- READMEs are short but lay out rules for reading/editing

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         Main Service                            │
│                                                                 │
│  ┌─────────────────────────────────────────┐                   │
│  │      Webhook Receiver (Primary)         │                   │
│  │   POST /webhooks - documents.update     │                   │
│  │   • Signature validation                │                   │
│  │   • Event filtering                     │                   │
│  └────────────────┬────────────────────────┘                   │
│                   │                                            │
│  ┌────────────────▼────────────────┐                           │
│  │  Fallback Polling (Optional)    │                           │
│  │  • Local dev / webhook failures │                           │
│  └────────────────┬────────────────┘                           │
│                   │                                            │
│                   ▼                                            │
│        ┌──────────────────────┐                                │
│        │  Command Processor   │                                │
│        │  • Detect markers    │                                │
│        │  • Parse guidance    │                                │
│        │  • Route to handlers │                                │
│        └──────────┬───────────┘                                │
│                   │                                            │
│                   ▼                                            │
│           ┌─────────────────┐                                  │
│           │   Worker Pool   │                                  │
│           │  (Concurrency)  │                                  │
│           └────────┬────────┘                                  │
└────────────────────┼─────────────────────────────────────────
                     │
   ┌─────────────────┼─────────────────┐
   │                 │                 │
┌──▼──────┐   ┌──────▼──────┐   ┌─────▼─────┐
│ Outline │   │  AI Client  │   │   State   │
│ Client  │   │  (OpenAI)   │   │   Store   │
│         │   │             │   │ (SQLite)  │
│ Rate    │   │ Rate        │   │ Q&A       │
│ Limited │   │ Limited     │   │ Tracking  │
└──┬──────┘   └──────┬──────┘   └───────────┘
   │                 │
┌──▼─────────────┐   │
│ Outline API    │   │
│ /collections   │   │
│ /documents     │   │
│ /documents.move│   │
│ /comments.create│  │
└────────────────┘   │
                ┌────▼──────────────┐
                │ OpenAI-Compatible │
                │ Endpoint          │
                └───────────────────┘

    ┌────────────────────────────────────┐
    │     Observability Layer            │
    │  ┌──────────────┐  ┌─────────────┐│
    │  │ Health Check │  │   Logging   ││
    │  │   Server     │  │  (Zerolog)  ││
    │  │ :8080/health │  │   Metrics   ││
    │  └──────────────┘  └─────────────┘│
    └────────────────────────────────────┘
```

**Key Components:**

1. **Webhook Receiver**: Real-time event processing from Outline
2. **Command Processor**: Detect and route commands (`/ai-file`, `/ai`, etc.)
3. **Worker Pool**: Concurrent document processing (configurable parallelism)
4. **Outline Client**: Rate-limited API client with retry logic
5. **AI Client**: OpenAI-compatible client with circuit breaker
6. **Taxonomy Builder**: Collection taxonomy with caching (TTL: 1 hour)
7. **Persistence Layer**: SQLite for Q&A state tracking
8. **Health Check Server**: Monitoring endpoints

**Data Flow:**

**Command-Driven Filing:**
1. User adds `/ai-file [guidance]` to document
2. Webhook triggers on `documents.update`
3. Command detected → Worker pool
4. Fetch document + taxonomy
5. AI analyzes → collection + confidence
6. If confidence low: convert to `?ai-file`, add comment
7. If confidence high: append search terms, move document
8. Remove command markers

**Q&A System:**
1. User adds `/ai [question]` to document
2. Webhook triggers
3. Extract question → Search workspace
4. Fetch top N relevant documents
5. Build context → AI generates answer
6. Create comment with answer + citations
7. Remove command marker
8. Track in state DB

---

## Technology Stack

### Implementation Language: Go

**Justification:**
- Single static binary
- Excellent standard library (HTTP, JSON, templates)
- Built-in concurrency (goroutines, channels)
- Strong typing and compile-time checks
- Low memory footprint
- Easy cross-compilation
- Good library ecosystem (GORM, viper, zerolog)

### Database: SQLite (embedded)

**Why:**
- Zero-config, single-file database
- Embedded (no separate process)
- GORM support
- Sufficient for single-instance deployment
- Easy backups (copy file)
- Future extensibility (interface for PostgreSQL)

### Configuration: Viper

**Why:**
- Environment variable support
- YAML configuration files
- Automatic config reloading (optional)
- Strong typing with structs
- De facto standard in Go

### Logging: Zerolog

**Why:**
- Structured JSON logging
- High performance (zero allocation)
- Context propagation
- Correlation IDs
- Production-ready

### Rate Limiting: golang.org/x/time/rate

**Why:**
- Token bucket algorithm
- Standard library extension
- Simple API
- Thread-safe
- Well-tested

### AI Client: sashabaranov/go-openai

**Why:**
- OpenAI-compatible API support
- Custom base URL (any provider)
- Streaming support
- Active maintenance
- Strong typing

### HTTP Client: Standard Library + Custom Wrapper

**Why:**
- No external dependencies
- Full control over retry logic
- Rate limiting integration
- Timeout configuration
- Circuit breaker pattern

### Deployment Model: Kubernetes Sidecar

**Why:**
- Co-located with Outline pod (minimal latency)
- Shared network namespace (localhost communication possible)
- Shared volumes for state persistence
- Automatic scaling with Outline
- Resource isolation via container limits
- Health check integration with Kubernetes
- ConfigMap for configuration
- Secrets for API keys

**Alternative: Standalone Deployment**
- Separate deployment/pod (if preferred)
- Service-to-service communication
- Independent scaling
- Requires proper service discovery

---

## Development Workflow

### 1. Before Starting Work

**ALWAYS:**
1. Read the relevant folder's README.md
2. Review [`docs/01_ARCHITECTURE/2026-01-19_01_HLD.md`](docs/01_ARCHITECTURE/2026-01-19_01_HLD.md)
3. Check [`docs/02_BACKLOG/`](docs/02_BACKLOG/) for current sprint stories
4. Check [`docs/03_WORKLOG/`](docs/03_WORKLOG/) for recent progress
5. Understand the specific design pattern (idempotency, webhooks, etc.)

### 2. During Work

**ALWAYS:**
1. Write tests FIRST (TDD)
2. Use strongly-typed structs (NO `map[string]interface{}`)
3. Follow idempotency patterns for content generation
4. Use worker pool for concurrent processing
5. Validate webhook signatures
6. Update relevant README.md files as you work
7. Update backlog story checklists `[ ]` → `[x]`
8. Commit regularly (every logical unit of work)

### 3. After Completing Work

**ALWAYS:**
1. Run all tests with timeout: `go test -timeout 30s ./...`
2. Run with race detector: `go test -timeout 30s -race ./...`
3. Verify tests pass
4. Check code coverage: `go test -timeout 30s -cover ./...`
5. Update backlog story status
6. Create handoff document in [`docs/03_WORKLOG/`](docs/03_WORKLOG/)
7. Update this README-LLM.md if needed
8. Commit all changes with descriptive message

### 4. Major Features

**ALWAYS:**
1. Create feature branch (document in [Branch Management](#branch-management))
2. Update branch list in this README
3. Work in branch
4. Merge to main when complete and tested
5. Update branch list (mark as merged)

---

## Common Commands

### Setup

```bash
# Initialize Go module
go mod init github.com/user/outline-ai-assistant

# Download dependencies
go mod download

# Tidy dependencies
go mod tidy
```

### Development

```bash
# Run application
go run cmd/outline-autofiler/main.go

# Run with config file
go run cmd/outline-autofiler/main.go -config config.yaml

# Build binary
go build -o bin/outline-autofiler cmd/outline-autofiler/main.go

# Run with race detector
go run -race cmd/outline-autofiler/main.go
```

### Testing

```bash
# Run all tests with timeout (ALWAYS use timeout)
go test -timeout 30s ./...

# Run tests with coverage
go test -timeout 30s -cover ./...

# Run tests with verbose output
go test -timeout 30s -v ./...

# Run specific package tests
go test -timeout 30s ./internal/commands/...

# Run tests with race detector
go test -timeout 30s -race ./...

# Generate coverage report
go test -timeout 30s -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run tests for specific feature
go test -timeout 30s -run TestSummarizeIdempotency ./internal/commands/
```

### Code Quality

```bash
# Format code
go fmt ./...

# Run static analysis
go vet ./...

# Run all quality checks
go fmt ./... && go vet ./...

# Check for suspicious constructs
go vet -all ./...
```

### Configuration

```bash
# Validate configuration
go run cmd/outline-autofiler/main.go -config config.yaml -validate

# Show configuration
go run cmd/outline-autofiler/main.go -config config.yaml -show-config

# Dry-run mode (no actual operations)
go run cmd/outline-autofiler/main.go -config config.yaml -dry-run
```

### Kubernetes Deployment (Primary)

```bash
# Build container image
docker build -t outline-ai-assistant:latest .

# Push to registry (if using external registry)
docker tag outline-ai-assistant:latest registry.example.com/outline-ai-assistant:latest
docker push registry.example.com/outline-ai-assistant:latest

# Deploy as sidecar to existing Outline pod
kubectl apply -f k8s/deployment-sidecar.yaml

# Check pod status
kubectl get pods -l app=outline

# View sidecar logs
kubectl logs -f deployment/outline -c ai-assistant

# Port forward for local testing
kubectl port-forward deployment/outline 8080:8080 8081:8081

# Update config via ConfigMap
kubectl create configmap outline-ai-config --from-file=config.yaml
kubectl rollout restart deployment/outline

# Update secrets
kubectl create secret generic outline-ai-secrets \
  --from-literal=outline-api-key=your-key \
  --from-literal=openai-api-key=your-key
```

**Kubernetes Sidecar Pattern:**
```yaml
# Example sidecar configuration in Outline deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: outline
spec:
  template:
    spec:
      containers:
      - name: outline
        image: outlinewiki/outline:latest
        # ... existing Outline config

      - name: ai-assistant
        image: outline-ai-assistant:latest
        ports:
        - containerPort: 8080  # Health check
          name: health
        - containerPort: 8081  # Webhooks
          name: webhooks
        env:
        - name: OUTLINE_API_KEY
          valueFrom:
            secretKeyRef:
              name: outline-ai-secrets
              key: outline-api-key
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: outline-ai-secrets
              key: openai-api-key
        volumeMounts:
        - name: config
          mountPath: /config.yaml
          subPath: config.yaml
        - name: state
          mountPath: /data
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
      volumes:
      - name: config
        configMap:
          name: outline-ai-config
      - name: state
        persistentVolumeClaim:
          claimName: outline-ai-state
```

### Docker (Secondary / Local Development)

```bash
# Build Docker image
docker build -t outline-ai-assistant:latest .

# Run standalone container (for testing)
docker run -p 8080:8080 -p 8081:8081 \
  -v $(pwd)/config.yaml:/config.yaml \
  -v $(pwd)/state.db:/data/state.db \
  -e OUTLINE_API_KEY=your-key \
  -e OPENAI_API_KEY=your-key \
  outline-ai-assistant:latest

# Run with docker-compose (for local Outline + AI assistant)
docker-compose up -d

# View logs
docker-compose logs -f ai-assistant
```

### Webhook Setup

```bash
# Register webhook with Outline (example using curl)
curl -X POST https://app.getoutline.com/api/webhookSubscriptions.create \
  -H "Authorization: Bearer $OUTLINE_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Outline AI Assistant",
    "url": "https://your-service.com/webhooks",
    "secret": "your-webhook-secret",
    "events": ["documents.update", "documents.create"]
  }'

# List webhooks
curl -X POST https://app.getoutline.com/api/webhookSubscriptions.list \
  -H "Authorization: Bearer $OUTLINE_API_KEY"

# Test webhook locally with ngrok
ngrok http 8081
```

---

## Branch Management

**Active Branches:**

| Branch Name | Purpose | Status | Created | Owner |
|-------------|---------|--------|---------|-------|
| `main` | Stable code | Active | 2026-01-19 | - |

**Merged Branches:**

| Branch Name | Purpose | Merged Date | PR/Commit |
|-------------|---------|-------------|-----------|
| _(none yet)_ | - | - | - |

**Branch Naming Convention:**
- Feature: `feature/short-description`
- Bugfix: `bugfix/issue-description`
- Hotfix: `hotfix/critical-issue`
- Docs: `docs/what-changed`

**Branch Workflow:**
1. Create branch from `main`
2. Document in table above
3. Work in branch
4. Regular commits
5. Merge to `main` when complete
6. Update table (move to "Merged Branches")
7. Delete branch

---

## Documentation Standards

### Design Documents

**Location:** [`docs/01_ARCHITECTURE/`](docs/01_ARCHITECTURE/)

**Naming:** `YYYY-MM-DD_NN_DOCUMENT_NAME.md` where NN is daily sequence (01, 02, etc.)

**Purpose:**
- High-level designs
- Architecture decisions
- Technical specifications
- Design patterns

**When to Create:**
- Before implementing major features
- When making architectural decisions
- When defining new subsystems

### Worklog Documents

**Location:** [`docs/03_WORKLOG/`](docs/03_WORKLOG/)

**Naming:** `YYYY-MM-DD_NN_description.md`

**Purpose:**
- Progress updates
- Handoff documents
- Session summaries
- Blockers and decisions

**When to Create:**
- After completing significant work
- Before context switch
- When handing off to another session
- When documenting blockers

### Backlog Stories

**Location:** [`docs/02_BACKLOG/`](docs/02_BACKLOG/)

**Structure:**
- `epics/` - High-level features
- `stories/` - Specific functionality
- Tasks: Defined within story files using checklists `[ ]`

**Format:**
```markdown
# Epic: Feature Name

## User Story: As a [role], I want [goal] so that [benefit]

### Acceptance Criteria
- [ ] Criterion 1
- [ ] Criterion 2

### Tasks
- [ ] Task 1
- [ ] Task 2
- [ ] Task 3

### Status
- Status: Not Started | In Progress | Complete
- Assigned: LLM Session ID or Human
- Started: YYYY-MM-DD
- Completed: YYYY-MM-DD
```

### README Files

**Every major folder MUST have a README.md:**

**Template:**
```markdown
# Folder Name

## Purpose
Brief description of what this folder contains.

## Rules
- Rule 1 for reading/editing files here
- Rule 2 for reading/editing files here

## Structure
- `subfolder/` - Description
- `file.go` - Description

## Key Files
- `important.go` - Why it's important
```

**Update Frequency:**
- When folder structure changes
- When new files are added
- When rules change

---

## Testing Requirements

### Test-Driven Development (TDD)

**MANDATORY WORKFLOW:**

1. **Write Test First**
   ```go
   func TestSummarizeIdempotency(t *testing.T) {
       // Arrange
       doc := &Document{
           Content: "# Title\n\nContent here...",
       }

       // Act - First run
       err := AddSummary(doc, "Summary text")

       // Assert - Summary added
       if err != nil {
           t.Errorf("Expected no error, got %v", err)
       }
       if !strings.Contains(doc.Content, "<!-- AI-SUMMARY-START -->") {
           t.Error("Expected summary markers")
       }

       // Act - Second run (idempotency test)
       err = AddSummary(doc, "Updated summary")

       // Assert - Single summary, replaced
       count := strings.Count(doc.Content, "<!-- AI-SUMMARY-START -->")
       if count != 1 {
           t.Errorf("Expected 1 summary marker, got %d", count)
       }
   }
   ```

2. **Run Test (Should Fail)**
   ```bash
   go test -timeout 30s ./internal/commands/
   ```

3. **Write Minimal Code**
   ```go
   func AddSummary(doc *Document, summary string) error {
       // Minimal implementation
       return nil
   }
   ```

4. **Run Test (Should Pass)**
   ```bash
   go test -timeout 30s ./internal/commands/
   ```

5. **Refactor If Needed**

### Test Coverage Requirements

**MANDATORY:**
- Multiple happy path tests
- Multiple unhappy path tests
- Edge case coverage
- Error condition tests
- Idempotency tests (for content generation)
- Webhook signature validation tests
- Rate limiting tests

**Example:**
```go
func TestWebhookSignatureValidation(t *testing.T) {
    tests := []struct {
        name       string
        payload    []byte
        signature  string
        secret     string
        wantValid  bool
    }{
        {"valid signature", []byte(`{"event":"documents.update"}`), "validhmac", "secret", true},
        {"invalid signature", []byte(`{"event":"documents.update"}`), "invalidhmac", "secret", false},
        {"empty signature", []byte(`{"event":"documents.update"}`), "", "secret", false},
        {"tampered payload", []byte(`{"event":"documents.delete"}`), "validhmac", "secret", false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            valid := ValidateWebhookSignature(tt.payload, tt.signature, tt.secret)
            if valid != tt.wantValid {
                t.Errorf("ValidateWebhookSignature() = %v, want %v", valid, tt.wantValid)
            }
        })
    }
}
```

### Test Organization

```
internal/
├── commands/
│   ├── handlers.go
│   └── handlers_test.go
├── webhooks/
│   ├── receiver.go
│   └── receiver_test.go
├── processor/
│   ├── processor.go
│   └── processor_test.go
```

**Rules:**
- Test files in same package as code
- Use `_test.go` suffix
- Use table-driven tests for multiple cases
- Use subtests with `t.Run()`
- Mock external dependencies (Outline API, AI API)

### Running Tests

**ALWAYS use timeout:**
```bash
# Good ✅
go test -timeout 30s ./...

# Bad ❌ (can hang forever)
go test ./...
```

**Before marking work complete:**
```bash
# Run all tests with race detector and timeout
go test -timeout 30s -race ./...

# Check coverage
go test -timeout 30s -cover ./...

# Generate coverage report
go test -timeout 30s -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

---

## Quick Reference

### Type Safety Checklist

- [ ] Using structs instead of `map[string]interface{}`?
- [ ] All types explicitly declared?
- [ ] Using generics instead of `interface{}`?
- [ ] Exported structs for reuse?
- [ ] Webhook events parsed to typed structs?
- [ ] AI responses parsed to typed structs?

### TDD Checklist

- [ ] Tests written BEFORE code?
- [ ] Tests run with timeout?
- [ ] Multiple happy paths tested?
- [ ] Multiple unhappy paths tested?
- [ ] Edge cases covered?
- [ ] All tests passing?
- [ ] Race detector clean?

### Idempotency Checklist

- [ ] Using hidden HTML comment markers?
- [ ] Marker pattern: `<!-- AI-X-START -->` ... `<!-- AI-X-END -->`?
- [ ] Replace content between markers (not append)?
- [ ] Respect user ownership (skip if markers removed)?
- [ ] Tested multiple runs produce clean results?

### Webhook Checklist

- [ ] Signature validation implemented?
- [ ] Respond within 5 seconds?
- [ ] Use worker queue for long operations?
- [ ] Handle `documents.update` events?
- [ ] Parse event payload to typed struct?
- [ ] Test with invalid signatures?

### Documentation Checklist

- [ ] README.md updated in affected folders?
- [ ] Backlog story checklist updated?
- [ ] Worklog entry created (if significant)?
- [ ] Architecture diagram updated (if changed)?
- [ ] Branch list updated (if applicable)?

### Commit Checklist

- [ ] All tests passing?
- [ ] Code formatted (`go fmt`)?
- [ ] No linter errors (`go vet`)?
- [ ] Documentation updated?
- [ ] Commit message descriptive?

---

## Getting Help

### When Uncertain

**ASK THE USER** - Do not guess or assume.

### Common Questions

**Q: Should I use a map here?**
A: No, unless parsing external data. Define a struct immediately.

**Q: Should I add concurrency?**
A: Only if there's clear benefit. Use worker pool pattern for document processing.

**Q: Should I add a comment?**
A: No, unless ABSOLUTELY necessary. Make code self-documenting.

**Q: Should I maintain backwards compatibility?**
A: No, implement the full final solution. No technical debt.

**Q: How do I make operations idempotent?**
A: Use hidden HTML comment markers. See `docs/01_ARCHITECTURE/2026-01-19_05_IDEMPOTENT_COMMANDS.md`

**Q: How do I validate webhook signatures?**
A: Use SHA-256 HMAC. See `docs/01_ARCHITECTURE/2026-01-19_04_WEBHOOK_FINDINGS.md`

**Q: Should I use polling or webhooks?**
A: Webhooks are primary. Polling is fallback only for local dev.

**Q: How do I handle low AI confidence?**
A: Convert `/ai-file` → `?ai-file`, add comment explaining uncertainty with alternatives.

---

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2026-01-19 | Initial creation - Design complete |

---

**Remember:** This is a living document. Update it as the project evolves.
