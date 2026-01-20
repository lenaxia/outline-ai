# Low-Level Design: Configuration System

**Domain:** Configuration Management
**Status:** Design
**Last Updated:** 2026-01-19
**Target Deployment:** Homelab/SOHO

## Purpose

Centralized configuration management for the Outline AI Assistant service with YAML configuration, environment variable substitution, and startup validation.

## Design Principles

1. **Fail Fast**: Validate all configuration at startup
2. **Environment-Aware**: Support dev/staging/prod configurations
3. **Secure by Default**: Never log secrets, load from env vars
4. **Type-Safe**: Strongly-typed configuration structs
5. **Simple for SOHO**: Single config file, minimal dependencies

## Configuration Structure

### File Format

**Location:** `/config.yaml` (default), configurable via `--config` flag

```yaml
service:
  max_concurrent_workers: 3
  health_check_port: 8080
  webhook_port: 8081
  public_url: "https://your-service.com"
  dry_run: false
  log_level: "info"

outline:
  api_endpoint: "https://app.getoutline.com/api"
  api_key: "${OUTLINE_API_KEY}"
  webhook_secret: "${OUTLINE_WEBHOOK_SECRET}"
  excluded_collection_ids: []
  rate_limit_per_minute: 60

webhooks:
  enabled: true
  port: 8081
  events: ["documents.update", "documents.create"]
  signature_validation: true
  fallback_polling:
    enabled: true
    interval: 60s

ai:
  endpoint: "https://api.openai.com/v1"
  api_key: "${OPENAI_API_KEY}"
  model: "gpt-4"
  confidence_threshold: 0.7
  request_timeout: 30s
  max_tokens: 4000
  rate_limit_per_minute: 20

processing:
  max_retries: 3
  retry_backoff_base: 30s
  retry_backoff_max: 5m

taxonomy:
  cache_ttl: 1h
  include_sample_documents: true
  max_samples_per_collection: 5

qna:
  enabled: true
  max_context_documents: 5
  answer_method: "comment"

enhancement:
  enabled: true
  enhance_titles: true
  add_summaries: true
  idempotent_updates: true
  respect_user_ownership: true

commands:
  enabled: true
  available: ["/ai", "/ai-file", "/summarize", "/enhance-title", "/related"]
  filing:
    include_alternatives: true
    max_alternatives: 3
    success_comment: true
    uncertainty_comment: true
  summarize:
    use_markers: true
    respect_no_markers: true
    detect_existing_format: true
  search_terms:
    use_markers: true
    respect_no_markers: true

persistence:
  database_path: "/data/state.db"
  backup_enabled: true
  backup_interval: 24h

logging:
  level: "info"
  format: "json"
  output: "stdout"
```

## Go Data Structures

### Main Config Struct

```go
package config

import (
    "time"
)

type Config struct {
    Service     ServiceConfig     `yaml:"service"`
    Outline     OutlineConfig     `yaml:"outline"`
    Webhooks    WebhookConfig     `yaml:"webhooks"`
    AI          AIConfig          `yaml:"ai"`
    Processing  ProcessingConfig  `yaml:"processing"`
    Taxonomy    TaxonomyConfig    `yaml:"taxonomy"`
    QnA         QnAConfig         `yaml:"qna"`
    Enhancement EnhancementConfig `yaml:"enhancement"`
    Commands    CommandsConfig    `yaml:"commands"`
    Persistence PersistenceConfig `yaml:"persistence"`
    Logging     LoggingConfig     `yaml:"logging"`
}

type ServiceConfig struct {
    MaxConcurrentWorkers int    `yaml:"max_concurrent_workers"`
    HealthCheckPort      int    `yaml:"health_check_port"`
    WebhookPort          int    `yaml:"webhook_port"`
    PublicURL            string `yaml:"public_url"`
    DryRun               bool   `yaml:"dry_run"`
    LogLevel             string `yaml:"log_level"`
}

type OutlineConfig struct {
    APIEndpoint           string   `yaml:"api_endpoint"`
    APIKey                string   `yaml:"api_key"`
    WebhookSecret         string   `yaml:"webhook_secret"`
    ExcludedCollectionIDs []string `yaml:"excluded_collection_ids"`
    RateLimitPerMinute    int      `yaml:"rate_limit_per_minute"`
}

type WebhookConfig struct {
    Enabled              bool                    `yaml:"enabled"`
    Port                 int                     `yaml:"port"`
    Events               []string                `yaml:"events"`
    SignatureValidation  bool                    `yaml:"signature_validation"`
    FallbackPolling      FallbackPollingConfig   `yaml:"fallback_polling"`
}

type FallbackPollingConfig struct {
    Enabled  bool          `yaml:"enabled"`
    Interval time.Duration `yaml:"interval"`
}

type AIConfig struct {
    Endpoint            string        `yaml:"endpoint"`
    APIKey              string        `yaml:"api_key"`
    Model               string        `yaml:"model"`
    ConfidenceThreshold float64       `yaml:"confidence_threshold"`
    RequestTimeout      time.Duration `yaml:"request_timeout"`
    MaxTokens           int           `yaml:"max_tokens"`
    RateLimitPerMinute  int           `yaml:"rate_limit_per_minute"`
}

type ProcessingConfig struct {
    MaxRetries       int           `yaml:"max_retries"`
    RetryBackoffBase time.Duration `yaml:"retry_backoff_base"`
    RetryBackoffMax  time.Duration `yaml:"retry_backoff_max"`
}

type TaxonomyConfig struct {
    CacheTTL                time.Duration `yaml:"cache_ttl"`
    IncludeSampleDocuments  bool          `yaml:"include_sample_documents"`
    MaxSamplesPerCollection int           `yaml:"max_samples_per_collection"`
}

type QnAConfig struct {
    Enabled             bool   `yaml:"enabled"`
    MaxContextDocuments int    `yaml:"max_context_documents"`
    AnswerMethod        string `yaml:"answer_method"`
}

type EnhancementConfig struct {
    Enabled              bool `yaml:"enabled"`
    EnhanceTitles        bool `yaml:"enhance_titles"`
    AddSummaries         bool `yaml:"add_summaries"`
    IdempotentUpdates    bool `yaml:"idempotent_updates"`
    RespectUserOwnership bool `yaml:"respect_user_ownership"`
}

type CommandsConfig struct {
    Enabled      bool                    `yaml:"enabled"`
    Available    []string                `yaml:"available"`
    Filing       FilingCommandConfig     `yaml:"filing"`
    Summarize    SummarizeCommandConfig  `yaml:"summarize"`
    SearchTerms  SearchTermsConfig       `yaml:"search_terms"`
}

type FilingCommandConfig struct {
    IncludeAlternatives  bool `yaml:"include_alternatives"`
    MaxAlternatives      int  `yaml:"max_alternatives"`
    SuccessComment       bool `yaml:"success_comment"`
    UncertaintyComment   bool `yaml:"uncertainty_comment"`
}

type SummarizeCommandConfig struct {
    UseMarkers            bool `yaml:"use_markers"`
    RespectNoMarkers      bool `yaml:"respect_no_markers"`
    DetectExistingFormat  bool `yaml:"detect_existing_format"`
}

type SearchTermsConfig struct {
    UseMarkers       bool `yaml:"use_markers"`
    RespectNoMarkers bool `yaml:"respect_no_markers"`
}

type PersistenceConfig struct {
    DatabasePath    string        `yaml:"database_path"`
    BackupEnabled   bool          `yaml:"backup_enabled"`
    BackupInterval  time.Duration `yaml:"backup_interval"`
}

type LoggingConfig struct {
    Level  string `yaml:"level"`
    Format string `yaml:"format"`
    Output string `yaml:"output"`
}
```

## Configuration Loading

### Loading Algorithm

```go
package config

import (
    "fmt"
    "os"
    "strings"

    "github.com/spf13/viper"
)

func Load(configPath string) (*Config, error) {
    // Set defaults
    setDefaults()

    // Load from file
    viper.SetConfigFile(configPath)
    if err := viper.ReadInConfig(); err != nil {
        return nil, fmt.Errorf("failed to read config: %w", err)
    }

    // Environment variable substitution
    viper.AutomaticEnv()
    viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

    // Parse into struct
    var cfg Config
    if err := viper.Unmarshal(&cfg); err != nil {
        return nil, fmt.Errorf("failed to unmarshal config: %w", err)
    }

    // Expand environment variables in string fields
    cfg = expandEnvVars(cfg)

    // Validate configuration
    if err := validate(&cfg); err != nil {
        return nil, fmt.Errorf("config validation failed: %w", err)
    }

    return &cfg, nil
}

func setDefaults() {
    viper.SetDefault("service.max_concurrent_workers", 3)
    viper.SetDefault("service.health_check_port", 8080)
    viper.SetDefault("service.webhook_port", 8081)
    viper.SetDefault("service.dry_run", false)
    viper.SetDefault("service.log_level", "info")

    viper.SetDefault("outline.rate_limit_per_minute", 60)
    viper.SetDefault("outline.excluded_collection_ids", []string{})

    viper.SetDefault("webhooks.enabled", true)
    viper.SetDefault("webhooks.port", 8081)
    viper.SetDefault("webhooks.signature_validation", true)
    viper.SetDefault("webhooks.events", []string{"documents.update", "documents.create"})
    viper.SetDefault("webhooks.fallback_polling.enabled", true)
    viper.SetDefault("webhooks.fallback_polling.interval", "60s")

    viper.SetDefault("ai.confidence_threshold", 0.7)
    viper.SetDefault("ai.request_timeout", "30s")
    viper.SetDefault("ai.max_tokens", 4000)
    viper.SetDefault("ai.rate_limit_per_minute", 20)

    viper.SetDefault("processing.max_retries", 3)
    viper.SetDefault("processing.retry_backoff_base", "30s")
    viper.SetDefault("processing.retry_backoff_max", "5m")

    viper.SetDefault("taxonomy.cache_ttl", "1h")
    viper.SetDefault("taxonomy.include_sample_documents", true)
    viper.SetDefault("taxonomy.max_samples_per_collection", 5)

    viper.SetDefault("qna.enabled", true)
    viper.SetDefault("qna.max_context_documents", 5)
    viper.SetDefault("qna.answer_method", "comment")

    viper.SetDefault("enhancement.enabled", true)
    viper.SetDefault("enhancement.enhance_titles", true)
    viper.SetDefault("enhancement.add_summaries", true)
    viper.SetDefault("enhancement.idempotent_updates", true)
    viper.SetDefault("enhancement.respect_user_ownership", true)

    viper.SetDefault("commands.enabled", true)
    viper.SetDefault("commands.filing.include_alternatives", true)
    viper.SetDefault("commands.filing.max_alternatives", 3)
    viper.SetDefault("commands.filing.success_comment", true)
    viper.SetDefault("commands.filing.uncertainty_comment", true)

    viper.SetDefault("persistence.database_path", "/data/state.db")
    viper.SetDefault("persistence.backup_enabled", true)
    viper.SetDefault("persistence.backup_interval", "24h")

    viper.SetDefault("logging.level", "info")
    viper.SetDefault("logging.format", "json")
    viper.SetDefault("logging.output", "stdout")
}

func expandEnvVars(cfg Config) Config {
    cfg.Outline.APIKey = os.ExpandEnv(cfg.Outline.APIKey)
    cfg.Outline.WebhookSecret = os.ExpandEnv(cfg.Outline.WebhookSecret)
    cfg.AI.APIKey = os.ExpandEnv(cfg.AI.APIKey)
    return cfg
}
```

## Validation

### Validation Rules

```go
package config

import (
    "fmt"
    "net/url"
)

func validate(cfg *Config) error {
    // Service validation
    if cfg.Service.MaxConcurrentWorkers < 1 {
        return fmt.Errorf("max_concurrent_workers must be >= 1")
    }
    if cfg.Service.HealthCheckPort < 1024 || cfg.Service.HealthCheckPort > 65535 {
        return fmt.Errorf("health_check_port must be between 1024 and 65535")
    }
    if cfg.Service.WebhookPort < 1024 || cfg.Service.WebhookPort > 65535 {
        return fmt.Errorf("webhook_port must be between 1024 and 65535")
    }

    // Outline validation
    if cfg.Outline.APIKey == "" {
        return fmt.Errorf("outline.api_key is required")
    }
    if _, err := url.Parse(cfg.Outline.APIEndpoint); err != nil {
        return fmt.Errorf("outline.api_endpoint is invalid: %w", err)
    }
    if cfg.Outline.RateLimitPerMinute < 1 {
        return fmt.Errorf("outline.rate_limit_per_minute must be >= 1")
    }

    // Webhook validation
    if cfg.Webhooks.Enabled {
        if cfg.Webhooks.Port < 1024 || cfg.Webhooks.Port > 65535 {
            return fmt.Errorf("webhooks.port must be between 1024 and 65535")
        }
        if cfg.Service.PublicURL == "" {
            return fmt.Errorf("service.public_url is required when webhooks enabled")
        }
        if cfg.Outline.WebhookSecret == "" {
            return fmt.Errorf("outline.webhook_secret is required when webhooks enabled")
        }
        if len(cfg.Webhooks.Events) == 0 {
            return fmt.Errorf("webhooks.events must have at least one event")
        }
    }

    // AI validation
    if cfg.AI.APIKey == "" {
        return fmt.Errorf("ai.api_key is required")
    }
    if _, err := url.Parse(cfg.AI.Endpoint); err != nil {
        return fmt.Errorf("ai.endpoint is invalid: %w", err)
    }
    if cfg.AI.ConfidenceThreshold < 0 || cfg.AI.ConfidenceThreshold > 1 {
        return fmt.Errorf("ai.confidence_threshold must be between 0 and 1")
    }
    if cfg.AI.MaxTokens < 100 {
        return fmt.Errorf("ai.max_tokens must be >= 100")
    }
    if cfg.AI.RateLimitPerMinute < 1 {
        return fmt.Errorf("ai.rate_limit_per_minute must be >= 1")
    }

    // Processing validation
    if cfg.Processing.MaxRetries < 0 {
        return fmt.Errorf("processing.max_retries must be >= 0")
    }
    if cfg.Processing.RetryBackoffBase <= 0 {
        return fmt.Errorf("processing.retry_backoff_base must be > 0")
    }
    if cfg.Processing.RetryBackoffMax < cfg.Processing.RetryBackoffBase {
        return fmt.Errorf("processing.retry_backoff_max must be >= retry_backoff_base")
    }

    // Taxonomy validation
    if cfg.Taxonomy.CacheTTL <= 0 {
        return fmt.Errorf("taxonomy.cache_ttl must be > 0")
    }
    if cfg.Taxonomy.MaxSamplesPerCollection < 0 {
        return fmt.Errorf("taxonomy.max_samples_per_collection must be >= 0")
    }

    // QnA validation
    if cfg.QnA.Enabled {
        if cfg.QnA.MaxContextDocuments < 1 {
            return fmt.Errorf("qna.max_context_documents must be >= 1")
        }
        if cfg.QnA.AnswerMethod != "comment" && cfg.QnA.AnswerMethod != "inline" {
            return fmt.Errorf("qna.answer_method must be 'comment' or 'inline'")
        }
    }

    // Commands validation
    if cfg.Commands.Enabled {
        if len(cfg.Commands.Available) == 0 {
            return fmt.Errorf("commands.available must have at least one command")
        }
        if cfg.Commands.Filing.MaxAlternatives < 1 {
            return fmt.Errorf("commands.filing.max_alternatives must be >= 1")
        }
    }

    // Persistence validation
    if cfg.Persistence.DatabasePath == "" {
        return fmt.Errorf("persistence.database_path is required")
    }

    // Logging validation
    validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
    if !validLevels[cfg.Logging.Level] {
        return fmt.Errorf("logging.level must be one of: debug, info, warn, error")
    }

    return nil
}
```

## Startup Validation

### External Dependencies Check

```go
package config

import (
    "context"
    "fmt"
    "net/http"
    "time"
)

func ValidateConnectivity(ctx context.Context, cfg *Config) error {
    // Test Outline API connectivity
    if err := testOutlineAPI(ctx, cfg); err != nil {
        return fmt.Errorf("outline API validation failed: %w", err)
    }

    // Test AI endpoint connectivity
    if err := testAIEndpoint(ctx, cfg); err != nil {
        return fmt.Errorf("AI endpoint validation failed: %w", err)
    }

    // Verify excluded collections exist
    if err := validateCollections(ctx, cfg); err != nil {
        return fmt.Errorf("collection validation failed: %w", err)
    }

    return nil
}

func testOutlineAPI(ctx context.Context, cfg *Config) error {
    client := &http.Client{Timeout: 10 * time.Second}

    req, err := http.NewRequestWithContext(ctx, "POST",
        cfg.Outline.APIEndpoint+"/auth.info", nil)
    if err != nil {
        return err
    }
    req.Header.Set("Authorization", "Bearer "+cfg.Outline.APIKey)

    resp, err := client.Do(req)
    if err != nil {
        return fmt.Errorf("connection failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode == 401 {
        return fmt.Errorf("invalid API key")
    }
    if resp.StatusCode != 200 {
        return fmt.Errorf("unexpected status: %d", resp.StatusCode)
    }

    return nil
}

func testAIEndpoint(ctx context.Context, cfg *Config) error {
    // Simple connectivity test - don't make actual AI calls
    // Just verify endpoint is reachable
    client := &http.Client{Timeout: 5 * time.Second}

    req, err := http.NewRequestWithContext(ctx, "GET", cfg.AI.Endpoint, nil)
    if err != nil {
        return err
    }

    resp, err := client.Do(req)
    if err != nil {
        return fmt.Errorf("connection failed: %w", err)
    }
    defer resp.Body.Close()

    return nil
}

func validateCollections(ctx context.Context, cfg *Config) error {
    if len(cfg.Outline.ExcludedCollectionIDs) == 0 {
        return nil
    }

    // Verify each excluded collection exists
    // Implementation would use Outline API client
    // For now, just placeholder
    return nil
}
```

## Security Considerations

### Secret Handling

```go
package config

import "strings"

func MaskSecrets(cfg Config) Config {
    masked := cfg
    masked.Outline.APIKey = maskString(cfg.Outline.APIKey)
    masked.Outline.WebhookSecret = maskString(cfg.Outline.WebhookSecret)
    masked.AI.APIKey = maskString(cfg.AI.APIKey)
    return masked
}

func maskString(s string) string {
    if len(s) <= 8 {
        return "****"
    }
    return s[:4] + "****" + s[len(s)-4:]
}

func (c Config) String() string {
    masked := MaskSecrets(c)
    return fmt.Sprintf("%+v", masked)
}
```

## Testing Strategy

### Unit Tests

```go
func TestConfigLoad(t *testing.T)
func TestConfigValidation(t *testing.T)
func TestEnvVarSubstitution(t *testing.T)
func TestDefaultValues(t *testing.T)
func TestSecretMasking(t *testing.T)
func TestConnectivityValidation(t *testing.T)
```

### Test Cases

1. **Valid configuration**: All required fields present
2. **Missing required fields**: APIKey, APIEndpoint
3. **Invalid values**: Negative ports, invalid URLs
4. **Environment variable substitution**: `${VAR_NAME}` expansion
5. **Default values**: Unspecified fields use defaults
6. **Secret masking**: Secrets not logged
7. **Connectivity tests**: Valid/invalid API keys

## Error Handling

### Startup Errors

- **Config file not found**: Log clear error, suggest example config
- **Invalid YAML**: Show parse error with line number
- **Validation failure**: List all validation errors
- **Connectivity failure**: Show which endpoint failed, suggest debugging

### Runtime Behavior

- Configuration is **immutable** after loading
- No hot-reloading (restart service to apply changes)
- For SOHO deployment, simplicity preferred over complexity

## SOHO Deployment Considerations

### Simplifications for Homelab

1. **Single config file**: No config directory hierarchy
2. **No distributed config**: No etcd/Consul integration
3. **No hot-reload**: Restart to apply changes (acceptable for SOHO)
4. **Environment variables only for secrets**: Keep it simple
5. **Sensible defaults**: Works out-of-box with minimal config

### Example Minimal Config

```yaml
service:
  public_url: "https://your-homelab.com"

outline:
  api_key: "${OUTLINE_API_KEY}"
  webhook_secret: "${OUTLINE_WEBHOOK_SECRET}"

ai:
  api_key: "${OPENAI_API_KEY}"
```

All other fields use defaults suitable for SOHO deployment.

## Future Enhancements

1. **Config validation dry-run**: `--validate-config` flag
2. **Config generation**: Generate example config with current values
3. **Per-collection overrides**: Different settings per collection
4. **User preferences**: Per-user confidence thresholds (out of scope for v1)

## Configuration Usage Patterns

### Passing Configuration

**Best Practices:**
1. **Store pointer in main service**: `config *config.Config`
   - The main Service struct stores the full config as a pointer
   - Allows access to all config throughout service lifetime

2. **Pass sub-configs by value for simple functions**: `func setupLogging(cfg config.LoggingConfig)`
   - When a function only needs a small part of the config
   - Reduces coupling and makes dependencies explicit
   - Config structs are small enough that copying is negligible

3. **Pass pointer for setup functions**: `func setupWebhookReceiver(cfg *config.Config, ...)`
   - When multiple config sections are needed
   - When the entire config is required

**Example:**
```go
type Service struct {
    config *config.Config  // Store pointer to full config
}

func (s *Service) initializeComponents() error {
    // Pass sub-config by value to focused function
    setupLogging(s.config.Logging)

    // Pass pointer when multiple sections needed
    receiver, err := setupWebhookReceiver(s.config, processor)

    // Direct field access from stored pointer
    client := outline.NewHTTPClient(
        s.config.Outline.APIEndpoint,
        s.config.Outline.APIKey,
        limiter,
    )
}
```

## Package Structure

```
internal/config/
├── config.go          # Main config struct and loading
├── defaults.go        # Default values
├── validation.go      # Validation logic
├── connectivity.go    # External dependency checks
├── security.go        # Secret masking
└── config_test.go     # Test suite
```

## Dependencies

- `github.com/spf13/viper` - Configuration management
- Standard library only for validation

---

**Status:** Ready for implementation
**Complexity:** Low
**Priority:** High (foundational component)
