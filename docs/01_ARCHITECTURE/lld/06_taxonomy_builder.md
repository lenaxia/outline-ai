# Low-Level Design: Taxonomy Builder

**Domain:** Workspace Context
**Status:** Design
**Last Updated:** 2026-01-19
**Target Deployment:** Homelab/SOHO

## Purpose

Build and cache workspace taxonomy (collections + sample documents) to provide AI context for document classification decisions.

## Design Principles

1. **Cached Context**: Reduce API calls with TTL-based caching
2. **Sample Documents**: Include representative documents for better classification
3. **Incremental Updates**: Refresh only when cache expires
4. **SOHO Optimized**: In-memory cache, no distributed caching
5. **Configurable Sampling**: Adjust sample size based on collection size

## Taxonomy Structure

### Domain Models

```go
package taxonomy

import (
    "time"

    "github.com/yourusername/outline-ai/internal/outline"
)

// Taxonomy represents the complete workspace taxonomy
// This is the source of truth for collection structure
type Taxonomy struct {
    Collections []CollectionTaxonomy `json:"collections"`
    GeneratedAt time.Time            `json:"generated_at"`
}

// CollectionTaxonomy contains full collection information with sample documents
// This model is converted to ai.TaxonomyCollection for AI consumption
type CollectionTaxonomy struct {
    ID              string   `json:"id"`
    Name            string   `json:"name"`
    Description     string   `json:"description"`
    SampleDocuments []string `json:"sample_documents,omitempty"`
    DocumentCount   int      `json:"document_count"`
}
```

## Builder Interface

### Interface Definition

```go
package taxonomy

import (
    "context"
)

type Builder interface {
    // Get taxonomy (from cache or build new)
    GetTaxonomy(ctx context.Context) (*Taxonomy, error)

    // Force rebuild (bypasses cache)
    RebuildTaxonomy(ctx context.Context) (*Taxonomy, error)

    // Clear cache
    InvalidateCache()

    // Get cache stats
    GetCacheStats() CacheStats
}

type CacheStats struct {
    Hit           int64
    Miss          int64
    LastBuildTime time.Time
    ExpiresAt     time.Time
    BuildDuration time.Duration
}
```

## Builder Implementation

### Main Builder

```go
package taxonomy

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/yourusername/outline-ai/internal/outline"
    "github.com/rs/zerolog/log"
)

type CachedBuilder struct {
    outlineClient         outline.Client
    cacheTTL              time.Duration
    maxSamplesPerCollection int
    includeSampleDocs     bool

    mu            sync.RWMutex
    cachedTaxonomy *Taxonomy
    cacheExpiresAt time.Time
    stats         CacheStats
}

func NewCachedBuilder(
    client outline.Client,
    cacheTTL time.Duration,
    maxSamples int,
    includeSamples bool,
) *CachedBuilder {
    return &CachedBuilder{
        outlineClient:         client,
        cacheTTL:              cacheTTL,
        maxSamplesPerCollection: maxSamples,
        includeSampleDocs:     includeSamples,
    }
}

func (b *CachedBuilder) GetTaxonomy(ctx context.Context) (*Taxonomy, error) {
    // Check cache first
    b.mu.RLock()
    if b.cachedTaxonomy != nil && time.Now().Before(b.cacheExpiresAt) {
        b.stats.Hit++
        taxonomy := b.cachedTaxonomy
        b.mu.RUnlock()

        log.Debug().
            Time("expires_at", b.cacheExpiresAt).
            Msg("Taxonomy cache hit")

        return taxonomy, nil
    }
    b.mu.RUnlock()

    // Cache miss - need to build
    b.mu.Lock()
    defer b.mu.Unlock()

    // Double-check after acquiring write lock
    if b.cachedTaxonomy != nil && time.Now().Before(b.cacheExpiresAt) {
        b.stats.Hit++
        return b.cachedTaxonomy, nil
    }

    b.stats.Miss++

    log.Info().Msg("Taxonomy cache miss, building new taxonomy")

    // Build new taxonomy
    taxonomy, err := b.buildTaxonomy(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to build taxonomy: %w", err)
    }

    // Cache the result
    b.cachedTaxonomy = taxonomy
    b.cacheExpiresAt = time.Now().Add(b.cacheTTL)
    b.stats.LastBuildTime = time.Now()

    log.Info().
        Int("collections", len(taxonomy.Collections)).
        Time("expires_at", b.cacheExpiresAt).
        Msg("Taxonomy built and cached")

    return taxonomy, nil
}

func (b *CachedBuilder) RebuildTaxonomy(ctx context.Context) (*Taxonomy, error) {
    b.mu.Lock()
    defer b.mu.Unlock()

    log.Info().Msg("Forcing taxonomy rebuild")

    taxonomy, err := b.buildTaxonomy(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to rebuild taxonomy: %w", err)
    }

    b.cachedTaxonomy = taxonomy
    b.cacheExpiresAt = time.Now().Add(b.cacheTTL)
    b.stats.LastBuildTime = time.Now()

    return taxonomy, nil
}

func (b *CachedBuilder) InvalidateCache() {
    b.mu.Lock()
    defer b.mu.Unlock()

    b.cachedTaxonomy = nil
    b.cacheExpiresAt = time.Time{}

    log.Info().Msg("Taxonomy cache invalidated")
}

func (b *CachedBuilder) GetCacheStats() CacheStats {
    b.mu.RLock()
    defer b.mu.RUnlock()

    return CacheStats{
        Hit:           b.stats.Hit,
        Miss:          b.stats.Miss,
        LastBuildTime: b.stats.LastBuildTime,
        ExpiresAt:     b.cacheExpiresAt,
        BuildDuration: b.stats.BuildDuration,
    }
}
```

### Taxonomy Building Logic

```go
package taxonomy

func (b *CachedBuilder) buildTaxonomy(ctx context.Context) (*Taxonomy, error) {
    startTime := time.Now()

    // Fetch all collections
    collections, err := b.outlineClient.ListCollections(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to list collections: %w", err)
    }

    taxonomy := &Taxonomy{
        Collections: make([]CollectionTaxonomy, 0, len(collections)),
        GeneratedAt: time.Now(),
    }

    // Build taxonomy for each collection
    for _, col := range collections {
        colTax, err := b.buildCollectionTaxonomy(ctx, col)
        if err != nil {
            log.Warn().
                Err(err).
                Str("collection_id", col.ID).
                Str("collection_name", col.Name).
                Msg("Failed to build taxonomy for collection, skipping")
            continue
        }

        taxonomy.Collections = append(taxonomy.Collections, *colTax)
    }

    b.stats.BuildDuration = time.Since(startTime)

    log.Info().
        Int("collections", len(taxonomy.Collections)).
        Dur("duration", b.stats.BuildDuration).
        Msg("Taxonomy build completed")

    return taxonomy, nil
}

func (b *CachedBuilder) buildCollectionTaxonomy(ctx context.Context, col *outline.Collection) (*CollectionTaxonomy, error) {
    colTax := &CollectionTaxonomy{
        ID:          col.ID,
        Name:        col.Name,
        Description: col.Description,
    }

    // Fetch sample documents if enabled
    if b.includeSampleDocs && b.maxSamplesPerCollection > 0 {
        samples, err := b.fetchSampleDocuments(ctx, col.ID)
        if err != nil {
            log.Warn().
                Err(err).
                Str("collection_id", col.ID).
                Msg("Failed to fetch sample documents")
        } else {
            colTax.SampleDocuments = samples
            colTax.DocumentCount = len(samples)
        }
    }

    return colTax, nil
}

func (b *CachedBuilder) fetchSampleDocuments(ctx context.Context, collectionID string) ([]string, error) {
    // List documents in collection
    docs, err := b.outlineClient.ListDocuments(ctx, collectionID)
    if err != nil {
        return nil, fmt.Errorf("failed to list documents: %w", err)
    }

    // Extract titles (up to max samples)
    var titles []string
    for i, doc := range docs {
        if i >= b.maxSamplesPerCollection {
            break
        }
        titles = append(titles, doc.Title)
    }

    return titles, nil
}
```

## Taxonomy Serialization

### Convert to AI Context

```go
package taxonomy

import (
    "encoding/json"

    "github.com/yourusername/outline-ai/internal/ai"
)

func (t *Taxonomy) ToAIContext() *ai.TaxonomyContext {
    collections := make([]ai.TaxonomyCollection, len(t.Collections))

    for i, col := range t.Collections {
        collections[i] = ai.TaxonomyCollection{
            ID:              col.ID,
            Name:            col.Name,
            Description:     col.Description,
            SampleDocuments: col.SampleDocuments,
        }
    }

    return &ai.TaxonomyContext{
        Collections: collections,
    }
}

func (t *Taxonomy) ToJSON() (string, error) {
    data, err := json.MarshalIndent(t, "", "  ")
    if err != nil {
        return "", err
    }
    return string(data), nil
}
```

## Cache Warming

### Periodic Refresh

```go
package taxonomy

import (
    "context"
    "time"

    "github.com/rs/zerolog/log"
)

type WarmupConfig struct {
    Enabled  bool
    Interval time.Duration
}

func StartCacheWarmup(ctx context.Context, builder Builder, cfg WarmupConfig) {
    if !cfg.Enabled {
        log.Info().Msg("Cache warmup disabled")
        return
    }

    log.Info().
        Dur("interval", cfg.Interval).
        Msg("Starting taxonomy cache warmup routine")

    ticker := time.NewTicker(cfg.Interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            log.Info().Msg("Cache warmup routine stopped")
            return
        case <-ticker.C:
            if err := warmupCache(ctx, builder); err != nil {
                log.Error().Err(err).Msg("Cache warmup failed")
            }
        }
    }
}

func warmupCache(ctx context.Context, builder Builder) error {
    log.Debug().Msg("Warming up taxonomy cache")

    _, err := builder.GetTaxonomy(ctx)
    if err != nil {
        return fmt.Errorf("failed to warmup cache: %w", err)
    }

    stats := builder.GetCacheStats()
    log.Info().
        Int64("hits", stats.Hit).
        Int64("misses", stats.Miss).
        Time("last_build", stats.LastBuildTime).
        Time("expires_at", stats.ExpiresAt).
        Msg("Cache warmup completed")

    return nil
}
```

## Filtering and Exclusions

### Collection Filtering

```go
package taxonomy

type FilterConfig struct {
    ExcludedCollectionIDs []string
    MinDocumentCount      int
}

func (b *CachedBuilder) SetFilter(cfg FilterConfig) {
    b.mu.Lock()
    defer b.mu.Unlock()

    b.excludedIDs = make(map[string]bool)
    for _, id := range cfg.ExcludedCollectionIDs {
        b.excludedIDs[id] = true
    }

    b.minDocCount = cfg.MinDocumentCount

    // Invalidate cache since filter changed
    b.cachedTaxonomy = nil
    b.cacheExpiresAt = time.Time{}

    log.Info().
        Int("excluded_collections", len(cfg.ExcludedCollectionIDs)).
        Int("min_doc_count", cfg.MinDocumentCount).
        Msg("Taxonomy filter updated")
}

func (b *CachedBuilder) shouldIncludeCollection(col *outline.Collection, docCount int) bool {
    // Check if excluded
    if b.excludedIDs[col.ID] {
        return false
    }

    // Check minimum document count
    if docCount < b.minDocCount {
        return false
    }

    return true
}
```

## Testing Strategy

### Unit Tests

```go
func TestCachedBuilder_GetTaxonomy(t *testing.T)
func TestCachedBuilder_CacheHit(t *testing.T)
func TestCachedBuilder_CacheMiss(t *testing.T)
func TestCachedBuilder_CacheExpiration(t *testing.T)
func TestCachedBuilder_RebuildTaxonomy(t *testing.T)
func TestCachedBuilder_InvalidateCache(t *testing.T)
func TestCachedBuilder_SampleDocuments(t *testing.T)
func TestCachedBuilder_CollectionFiltering(t *testing.T)
func TestTaxonomy_ToAIContext(t *testing.T)
func TestCacheWarmup(t *testing.T)
```

### Mock Outline Client

```go
type MockOutlineClient struct {
    ListCollectionsFunc func(ctx context.Context) ([]*outline.Collection, error)
    ListDocumentsFunc   func(ctx context.Context, collectionID string) ([]*outline.Document, error)
}

func (m *MockOutlineClient) ListCollections(ctx context.Context) ([]*outline.Collection, error) {
    if m.ListCollectionsFunc != nil {
        return m.ListCollectionsFunc(ctx)
    }
    return []*outline.Collection{
        {ID: "col1", Name: "Engineering", Description: "Tech docs"},
        {ID: "col2", Name: "Product", Description: "Product specs"},
    }, nil
}

func setupTestBuilder(t *testing.T) (*CachedBuilder, *MockOutlineClient) {
    mockClient := &MockOutlineClient{}
    builder := NewCachedBuilder(mockClient, 1*time.Hour, 5, true)
    return builder, mockClient
}
```

## Performance Considerations

### For SOHO Deployment

- **Cache TTL**: 1 hour (configurable)
- **Sample documents**: 5 per collection (configurable)
- **Build time**: < 10 seconds for 50 collections
- **Memory footprint**: < 1 MB for typical workspace
- **API calls**: 1 + N (1 for collections, N for document lists)

### Optimization Strategies

1. **Lazy loading**: Only fetch sample documents when needed
2. **Partial refresh**: Refresh only changed collections (future enhancement)
3. **Compression**: Compress cached taxonomy for large workspaces
4. **Parallel fetching**: Fetch collection data concurrently

### Concurrent Access

```go
// Thread-safe by design
// Read lock for cache hits (high concurrency)
// Write lock only for cache misses (infrequent)
// No external synchronization needed
```

## SOHO Deployment Considerations

### Simplifications for Homelab

1. **In-memory cache**: No Redis/Memcached needed
2. **Single instance**: No distributed cache coordination
3. **Simple TTL**: No complex cache invalidation strategy
4. **Fixed sample size**: No dynamic sampling based on collection size
5. **Synchronous builds**: No async background rebuilding

### Example Configuration

```yaml
taxonomy:
  cache_ttl: 1h
  include_sample_documents: true
  max_samples_per_collection: 5
  excluded_collection_ids: ["archive-collection-id"]
  min_document_count: 0

  # Optional cache warmup
  warmup:
    enabled: true
    interval: 30m
```

## Integration Example

### Usage in Classification Handler

```go
package handlers

func (h *FilingHandler) HandleFiling(ctx context.Context, doc *outline.Document, guidance string) error {
    // Get taxonomy (cached)
    taxonomy, err := h.taxonomyBuilder.GetTaxonomy(ctx)
    if err != nil {
        return fmt.Errorf("failed to get taxonomy: %w", err)
    }

    // Classify document
    classReq := &ai.ClassificationRequest{
        DocumentTitle:   doc.Title,
        DocumentContent: doc.Text,
        UserGuidance:    guidance,
        Taxonomy:        taxonomy.ToAIContext(),
    }

    classResp, err := h.aiClient.ClassifyDocument(ctx, classReq)
    if err != nil {
        return fmt.Errorf("classification failed: %w", err)
    }

    // ... rest of filing logic
}
```

## Package Structure

```
internal/taxonomy/
├── builder.go          # Builder interface and implementation
├── models.go           # Taxonomy domain models
├── cache.go            # Cache management
├── warmup.go           # Cache warmup routines
├── filter.go           # Collection filtering
├── serialization.go    # JSON/AI context conversion
└── taxonomy_test.go    # Test suite
```

## Dependencies

- `github.com/yourusername/outline-ai/internal/outline` - Outline API client
- `github.com/yourusername/outline-ai/internal/ai` - AI models
- `github.com/rs/zerolog` - Logging
- Standard library for caching and sync

---

**Status:** Ready for implementation
**Complexity:** Low-Medium
**Priority:** High (required for classification)
