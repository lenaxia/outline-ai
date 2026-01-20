# Low-Level Design: Persistence Layer

**Domain:** State Persistence
**Status:** Design
**Last Updated:** 2026-01-19
**Target Deployment:** Homelab/SOHO

## Purpose

Track Q&A processing state to prevent duplicate answers and optionally log command execution for audit purposes.

## Design Principles

1. **Minimal State**: Only persist what's necessary
2. **Embedded Database**: SQLite for zero-config deployment
3. **Interface-Based**: Abstract storage for future extensibility
4. **SOHO-Optimized**: Single instance, no distributed locks needed
5. **Automatic Cleanup**: Remove stale entries periodically

## Why SQLite

For homelab/SOHO deployments:
- **Zero configuration**: Single file database
- **No separate process**: Embedded in application
- **Simple backups**: Copy database file
- **Sufficient performance**: 100s of operations/sec
- **No network overhead**: Direct file access
- **ACID transactions**: Data integrity guaranteed

## Data Model

### Schema Design

```sql
-- Q&A state tracking (prevents duplicate answers)
CREATE TABLE IF NOT EXISTS question_state (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    question_hash TEXT NOT NULL UNIQUE,
    document_id TEXT NOT NULL,
    question_text TEXT NOT NULL,
    processed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    answer_delivered BOOLEAN NOT NULL DEFAULT FALSE,
    comment_id TEXT,
    last_error TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_question_state_document ON question_state(document_id);
CREATE INDEX idx_question_state_hash ON question_state(question_hash);
CREATE INDEX idx_question_state_processed ON question_state(processed_at);

-- Optional: Command execution audit log
CREATE TABLE IF NOT EXISTS command_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    document_id TEXT NOT NULL,
    command_type TEXT NOT NULL,
    command_args TEXT,
    executed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    status TEXT NOT NULL,
    error_message TEXT,
    execution_time_ms INTEGER,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_command_log_document ON command_log(document_id);
CREATE INDEX idx_command_log_executed ON command_log(executed_at);
CREATE INDEX idx_command_log_status ON command_log(status);
```

### Domain Models

```go
package persistence

import (
    "time"
)

type QuestionState struct {
    ID              int64     `gorm:"primaryKey;autoIncrement"`
    QuestionHash    string    `gorm:"uniqueIndex;not null"`
    DocumentID      string    `gorm:"index;not null"`
    QuestionText    string    `gorm:"not null"`
    ProcessedAt     time.Time `gorm:"index;not null"`
    AnswerDelivered bool      `gorm:"not null;default:false"`
    CommentID       *string   `gorm:"default:null"`
    LastError       *string   `gorm:"default:null"`
    RetryCount      int       `gorm:"not null;default:0"`
    CreatedAt       time.Time `gorm:"autoCreateTime"`
    UpdatedAt       time.Time `gorm:"autoUpdateTime"`
}

type CommandLog struct {
    ID               int64     `gorm:"primaryKey;autoIncrement"`
    DocumentID       string    `gorm:"index;not null"`
    CommandType      string    `gorm:"not null"`
    CommandArgs      *string   `gorm:"default:null"`
    ExecutedAt       time.Time `gorm:"index;not null"`
    Status           string    `gorm:"index;not null"`
    ErrorMessage     *string   `gorm:"default:null"`
    ExecutionTimeMs  *int      `gorm:"default:null"`
    CreatedAt        time.Time `gorm:"autoCreateTime"`
}

const (
    CommandStatusSuccess  = "success"
    CommandStatusFailed   = "failed"
    CommandStatusRetrying = "retrying"
)
```

## Storage Interface

### Abstract Interface

```go
package persistence

import (
    "context"
    "time"
)

type Storage interface {
    // Q&A state management
    HasAnsweredQuestion(ctx context.Context, questionHash string) (bool, error)
    MarkQuestionAnswered(ctx context.Context, state *QuestionState) error
    GetQuestionState(ctx context.Context, questionHash string) (*QuestionState, error)
    UpdateQuestionState(ctx context.Context, state *QuestionState) error
    DeleteStaleQuestions(ctx context.Context, olderThan time.Time) (int64, error)

    // Command logging (optional)
    LogCommand(ctx context.Context, log *CommandLog) error
    GetCommandHistory(ctx context.Context, documentID string, limit int) ([]*CommandLog, error)

    // Health and maintenance
    Ping(ctx context.Context) error
    Close() error
    Backup(ctx context.Context, destinationPath string) error
}
```

## SQLite Implementation

### Implementation

```go
package persistence

import (
    "context"
    "crypto/sha256"
    "fmt"
    "io"
    "os"
    "time"

    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
    "gorm.io/gorm/logger"
)

type SQLiteStorage struct {
    db *gorm.DB
}

func NewSQLiteStorage(dbPath string, logLevel logger.LogLevel) (*SQLiteStorage, error) {
    db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
        Logger: logger.Default.LogMode(logLevel),
    })
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }

    // Run migrations
    if err := db.AutoMigrate(&QuestionState{}, &CommandLog{}); err != nil {
        return nil, fmt.Errorf("failed to migrate database: %w", err)
    }

    // Configure connection pool
    sqlDB, err := db.DB()
    if err != nil {
        return nil, fmt.Errorf("failed to get underlying DB: %w", err)
    }

    // SQLite connection pool settings
    sqlDB.SetMaxOpenConns(1) // SQLite: single writer
    sqlDB.SetMaxIdleConns(1)
    sqlDB.SetConnMaxLifetime(time.Hour)

    return &SQLiteStorage{db: db}, nil
}

// Q&A state management

func (s *SQLiteStorage) HasAnsweredQuestion(ctx context.Context, questionHash string) (bool, error) {
    var count int64
    err := s.db.WithContext(ctx).
        Model(&QuestionState{}).
        Where("question_hash = ? AND answer_delivered = ?", questionHash, true).
        Count(&count).Error

    if err != nil {
        return false, fmt.Errorf("failed to check question state: %w", err)
    }

    return count > 0, nil
}

func (s *SQLiteStorage) MarkQuestionAnswered(ctx context.Context, state *QuestionState) error {
    state.AnswerDelivered = true
    state.ProcessedAt = time.Now()

    result := s.db.WithContext(ctx).Create(state)
    if result.Error != nil {
        return fmt.Errorf("failed to mark question answered: %w", result.Error)
    }

    return nil
}

func (s *SQLiteStorage) GetQuestionState(ctx context.Context, questionHash string) (*QuestionState, error) {
    var state QuestionState
    err := s.db.WithContext(ctx).
        Where("question_hash = ?", questionHash).
        First(&state).Error

    if err == gorm.ErrRecordNotFound {
        return nil, ErrQuestionNotFound
    }
    if err != nil {
        return nil, fmt.Errorf("failed to get question state: %w", err)
    }

    return &state, nil
}

func (s *SQLiteStorage) UpdateQuestionState(ctx context.Context, state *QuestionState) error {
    state.UpdatedAt = time.Now()

    result := s.db.WithContext(ctx).Save(state)
    if result.Error != nil {
        return fmt.Errorf("failed to update question state: %w", result.Error)
    }

    return nil
}

func (s *SQLiteStorage) DeleteStaleQuestions(ctx context.Context, olderThan time.Time) (int64, error) {
    result := s.db.WithContext(ctx).
        Where("processed_at < ?", olderThan).
        Delete(&QuestionState{})

    if result.Error != nil {
        return 0, fmt.Errorf("failed to delete stale questions: %w", result.Error)
    }

    return result.RowsAffected, nil
}

// Command logging

func (s *SQLiteStorage) LogCommand(ctx context.Context, log *CommandLog) error {
    result := s.db.WithContext(ctx).Create(log)
    if result.Error != nil {
        return fmt.Errorf("failed to log command: %w", result.Error)
    }

    return nil
}

func (s *SQLiteStorage) GetCommandHistory(ctx context.Context, documentID string, limit int) ([]*CommandLog, error) {
    var logs []*CommandLog
    err := s.db.WithContext(ctx).
        Where("document_id = ?", documentID).
        Order("executed_at DESC").
        Limit(limit).
        Find(&logs).Error

    if err != nil {
        return nil, fmt.Errorf("failed to get command history: %w", err)
    }

    return logs, nil
}

// Health and maintenance

func (s *SQLiteStorage) Ping(ctx context.Context) error {
    sqlDB, err := s.db.DB()
    if err != nil {
        return err
    }
    return sqlDB.PingContext(ctx)
}

func (s *SQLiteStorage) Close() error {
    sqlDB, err := s.db.DB()
    if err != nil {
        return err
    }
    return sqlDB.Close()
}

func (s *SQLiteStorage) Backup(ctx context.Context, destinationPath string) error {
    // Get underlying SQLite connection
    sqlDB, err := s.db.DB()
    if err != nil {
        return fmt.Errorf("failed to get DB connection: %w", err)
    }

    // Force checkpoint to write WAL to main DB
    if _, err := sqlDB.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
        return fmt.Errorf("failed to checkpoint WAL: %w", err)
    }

    // Get source file path from connection
    var dbPath string
    row := sqlDB.QueryRowContext(ctx, "PRAGMA database_list")
    var seq int
    var name string
    if err := row.Scan(&seq, &name, &dbPath); err != nil {
        return fmt.Errorf("failed to get database path: %w", err)
    }

    // Copy database file
    source, err := os.Open(dbPath)
    if err != nil {
        return fmt.Errorf("failed to open source: %w", err)
    }
    defer source.Close()

    destination, err := os.Create(destinationPath)
    if err != nil {
        return fmt.Errorf("failed to create destination: %w", err)
    }
    defer destination.Close()

    if _, err := io.Copy(destination, source); err != nil {
        return fmt.Errorf("failed to copy database: %w", err)
    }

    return destination.Sync()
}
```

### Question Hash Generation

**Note:** Question hash generation is implemented in the `qna` package with proper normalization to handle variations in question formatting. See LLD-10 (Q&A System) for the canonical implementation.

```go
package persistence

// GenerateQuestionHash is provided by the qna package
// to ensure consistent normalization across the system.
// Use qna.GenerateQuestionHash(documentID, questionText)
```

## Cleanup Strategy

### Automatic Cleanup

```go
package persistence

import (
    "context"
    "time"

    "github.com/rs/zerolog/log"
)

type CleanupConfig struct {
    QuestionStateRetention time.Duration
    CleanupInterval        time.Duration
}

func StartCleanupRoutine(ctx context.Context, storage Storage, cfg CleanupConfig) {
    ticker := time.NewTicker(cfg.CleanupInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            log.Info().Msg("Cleanup routine stopped")
            return
        case <-ticker.C:
            if err := runCleanup(ctx, storage, cfg); err != nil {
                log.Error().Err(err).Msg("Cleanup failed")
            }
        }
    }
}

func runCleanup(ctx context.Context, storage Storage, cfg CleanupConfig) error {
    cutoff := time.Now().Add(-cfg.QuestionStateRetention)

    deleted, err := storage.DeleteStaleQuestions(ctx, cutoff)
    if err != nil {
        return fmt.Errorf("failed to delete stale questions: %w", err)
    }

    if deleted > 0 {
        log.Info().
            Int64("deleted", deleted).
            Time("cutoff", cutoff).
            Msg("Cleaned up stale question states")
    }

    return nil
}
```

## Backup Strategy

### Periodic Backups

```go
package persistence

import (
    "context"
    "fmt"
    "path/filepath"
    "time"

    "github.com/rs/zerolog/log"
)

type BackupConfig struct {
    Enabled         bool
    Interval        time.Duration
    BackupDirectory string
    MaxBackups      int
}

func StartBackupRoutine(ctx context.Context, storage Storage, cfg BackupConfig) {
    if !cfg.Enabled {
        log.Info().Msg("Backups disabled")
        return
    }

    ticker := time.NewTicker(cfg.Interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            log.Info().Msg("Backup routine stopped")
            return
        case <-ticker.C:
            if err := runBackup(ctx, storage, cfg); err != nil {
                log.Error().Err(err).Msg("Backup failed")
            }
        }
    }
}

func runBackup(ctx context.Context, storage Storage, cfg BackupConfig) error {
    timestamp := time.Now().Format("20060102-150405")
    backupPath := filepath.Join(cfg.BackupDirectory, fmt.Sprintf("state-%s.db", timestamp))

    if err := storage.Backup(ctx, backupPath); err != nil {
        return fmt.Errorf("backup failed: %w", err)
    }

    log.Info().Str("path", backupPath).Msg("Database backup completed")

    // Cleanup old backups
    return cleanupOldBackups(cfg.BackupDirectory, cfg.MaxBackups)
}

func cleanupOldBackups(directory string, maxBackups int) error {
    // Implementation: List files, sort by timestamp, delete oldest if > maxBackups
    // Simplified for LLD - full implementation in code
    return nil
}
```

## Transaction Management

### Transaction Support

```go
package persistence

import (
    "context"
    "fmt"
)

type TransactionFunc func(ctx context.Context, tx Storage) error

func (s *SQLiteStorage) RunInTransaction(ctx context.Context, fn TransactionFunc) error {
    return s.db.Transaction(func(tx *gorm.DB) error {
        txStorage := &SQLiteStorage{db: tx}
        return fn(ctx, txStorage)
    })
}
```

## Error Handling

### Error Types

```go
package persistence

import (
    "errors"
    "fmt"
)

// Package-specific errors for persistence layer
var (
    ErrNotFound           = errors.New("persistence: record not found")
    ErrQuestionNotFound   = errors.New("persistence: question not found")
    ErrDuplicateEntry     = errors.New("persistence: duplicate entry")
    ErrDatabaseLocked     = errors.New("persistence: database locked")
    ErrInvalidInput       = errors.New("persistence: invalid input")
)

func wrapError(err error) error {
    if err == nil {
        return nil
    }

    switch {
    case errors.Is(err, gorm.ErrRecordNotFound):
        return ErrNotFound
    case errors.Is(err, gorm.ErrDuplicatedKey):
        return ErrDuplicateEntry
    default:
        return err
    }
}

// IsQuestionNotFound checks if error is ErrQuestionNotFound
func IsQuestionNotFound(err error) bool {
    return errors.Is(err, ErrQuestionNotFound)
}
```

## Testing Strategy

### Unit Tests

```go
func TestSQLiteStorage_MarkQuestionAnswered(t *testing.T)
func TestSQLiteStorage_HasAnsweredQuestion(t *testing.T)
func TestSQLiteStorage_DeleteStaleQuestions(t *testing.T)
func TestSQLiteStorage_LogCommand(t *testing.T)
func TestSQLiteStorage_GetCommandHistory(t *testing.T)
func TestSQLiteStorage_Backup(t *testing.T)
func TestSQLiteStorage_Transactions(t *testing.T)
// Note: TestGenerateQuestionHash is in qna package (LLD-10)
```

### Integration Tests

```go
func TestSQLiteStorage_ConcurrentWrites(t *testing.T)
func TestSQLiteStorage_BackupRestore(t *testing.T)
func TestCleanupRoutine(t *testing.T)
func TestBackupRoutine(t *testing.T)
```

### Test Database Setup

```go
func setupTestDB(t *testing.T) (*SQLiteStorage, func()) {
    tmpDB := filepath.Join(t.TempDir(), "test.db")

    storage, err := NewSQLiteStorage(tmpDB, logger.Silent)
    if err != nil {
        t.Fatalf("Failed to create test DB: %v", err)
    }

    cleanup := func() {
        storage.Close()
        os.Remove(tmpDB)
    }

    return storage, cleanup
}
```

## Performance Considerations

### For SOHO Deployment

- **Expected Load**: 10-100 operations per hour
- **Database Size**: < 10 MB (thousands of questions)
- **Backup Size**: < 10 MB
- **Query Performance**: < 10ms per query

### Optimizations

1. **Indexes**: On frequently queried columns
2. **WAL Mode**: Better concurrency (SQLite default)
3. **Connection Pooling**: Single connection for SQLite
4. **Batch Operations**: Group multiple inserts when possible

### Monitoring

```go
func (s *SQLiteStorage) GetStats(ctx context.Context) (*Stats, error) {
    var stats Stats

    s.db.WithContext(ctx).Model(&QuestionState{}).Count(&stats.TotalQuestions)
    s.db.WithContext(ctx).Model(&CommandLog{}).Count(&stats.TotalCommands)

    // Database size
    sqlDB, _ := s.db.DB()
    var pageCount, pageSize int64
    sqlDB.QueryRow("PRAGMA page_count").Scan(&pageCount)
    sqlDB.QueryRow("PRAGMA page_size").Scan(&pageSize)
    stats.DatabaseSizeBytes = pageCount * pageSize

    return &stats, nil
}

type Stats struct {
    TotalQuestions     int64
    TotalCommands      int64
    DatabaseSizeBytes  int64
}
```

## SOHO Deployment Considerations

### Simplifications

1. **Single instance**: No distributed locking needed
2. **No replication**: Single database file
3. **Simple backups**: File copy is sufficient
4. **No sharding**: Single database handles SOHO load
5. **No connection pooling complexity**: One connection sufficient

### Migration Path

If scaling beyond SOHO becomes necessary:
1. Implement PostgreSQL storage (same interface)
2. Add connection pooling
3. Add replication for HA
4. Migrate data: SQLite → PostgreSQL

The abstract interface ensures easy migration.

## Package Structure

```
internal/persistence/
├── interface.go       # Storage interface
├── sqlite.go          # SQLite implementation
├── models.go          # Domain models
├── cleanup.go         # Cleanup routines
├── backup.go          # Backup routines
├── errors.go          # Error types
└── persistence_test.go # Test suite

Note: Question hashing is implemented in internal/qna/deduplication.go
```

## Dependencies

- `gorm.io/gorm` - ORM for database operations
- `gorm.io/driver/sqlite` - SQLite driver for GORM
- Standard library for backups and cleanup

---

**Status:** Ready for implementation
**Complexity:** Medium
**Priority:** High (required for Q&A deduplication)
