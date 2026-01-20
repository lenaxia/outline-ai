package mocks

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Package-level errors matching persistence package
var (
	ErrNotFoundStorage      = errors.New("persistence: record not found")
	ErrQuestionNotFound     = errors.New("persistence: question not found")
	ErrDuplicateEntry       = errors.New("persistence: duplicate entry")
	ErrDatabaseLocked       = errors.New("persistence: database locked")
	ErrInvalidInput         = errors.New("persistence: invalid input")
)

// QuestionState represents the state of a question
type QuestionState struct {
	ID              int64
	QuestionHash    string
	DocumentID      string
	QuestionText    string
	ProcessedAt     time.Time
	AnswerDelivered bool
	CommentID       *string
	LastError       *string
	RetryCount      int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// CommandLog represents a logged command execution
type CommandLog struct {
	ID              int64
	DocumentID      string
	CommandType     string
	CommandArgs     *string
	ExecutedAt      time.Time
	Status          string
	ErrorMessage    *string
	ExecutionTimeMs *int
	CreatedAt       time.Time
}

// Command status constants
const (
	CommandStatusSuccess  = "success"
	CommandStatusFailed   = "failed"
	CommandStatusRetrying = "retrying"
)

// StorageMock is a mock implementation of the persistence.Storage interface
type StorageMock struct {
	mu sync.RWMutex

	// In-memory storage
	questionStates map[string]*QuestionState // keyed by question hash
	commandLogs    map[string][]*CommandLog  // keyed by document ID

	// Configuration
	failureMode    bool
	specificErrors map[string]error
	callCounts     map[string]int

	// Counters for IDs
	questionIDCounter int64
	commandIDCounter  int64

	// Transaction support
	inTransaction bool
}

// NewStorageMock creates a new mock storage instance
func NewStorageMock() *StorageMock {
	return &StorageMock{
		questionStates: make(map[string]*QuestionState),
		commandLogs:    make(map[string][]*CommandLog),
		specificErrors: make(map[string]error),
		callCounts:     make(map[string]int),
	}
}

// Helper Methods for Test Setup

// SeedQuestionState adds a question state to the mock storage
func (m *StorageMock) SeedQuestionState(state *QuestionState) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state.ID == 0 {
		m.questionIDCounter++
		state.ID = m.questionIDCounter
	}

	if state.CreatedAt.IsZero() {
		state.CreatedAt = time.Now()
	}
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = time.Now()
	}
	if state.ProcessedAt.IsZero() {
		state.ProcessedAt = time.Now()
	}

	m.questionStates[state.QuestionHash] = state
}

// SeedCommandLog adds a command log to the mock storage
func (m *StorageMock) SeedCommandLog(log *CommandLog) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if log.ID == 0 {
		m.commandIDCounter++
		log.ID = m.commandIDCounter
	}

	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}
	if log.ExecutedAt.IsZero() {
		log.ExecutedAt = time.Now()
	}

	if m.commandLogs[log.DocumentID] == nil {
		m.commandLogs[log.DocumentID] = make([]*CommandLog, 0)
	}
	m.commandLogs[log.DocumentID] = append(m.commandLogs[log.DocumentID], log)
}

// GetQuestionStateByHash returns a question state by hash (for testing)
func (m *StorageMock) GetQuestionStateByHash(hash string) *QuestionState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.questionStates[hash]
}

// Configuration Methods

// SetFailureMode configures all operations to fail
func (m *StorageMock) SetFailureMode(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failureMode = enabled
}

// SetMethodError sets a specific error for a method
func (m *StorageMock) SetMethodError(method string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.specificErrors[method] = err
}

// GetCallCount returns the number of times a method was called
func (m *StorageMock) GetCallCount(method string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.callCounts[method]
}

// Clear removes all data from storage
func (m *StorageMock) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.questionStates = make(map[string]*QuestionState)
	m.commandLogs = make(map[string][]*CommandLog)
	m.questionIDCounter = 0
	m.commandIDCounter = 0
}

// Reset clears all data and configuration
func (m *StorageMock) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.questionStates = make(map[string]*QuestionState)
	m.commandLogs = make(map[string][]*CommandLog)
	m.specificErrors = make(map[string]error)
	m.callCounts = make(map[string]int)
	m.failureMode = false
	m.questionIDCounter = 0
	m.commandIDCounter = 0
	m.inTransaction = false
}

// Internal helpers

func (m *StorageMock) recordCall(method string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCounts[method]++
}

func (m *StorageMock) checkError(method string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.failureMode {
		return ErrDatabaseLocked
	}

	if err, ok := m.specificErrors[method]; ok {
		return err
	}

	return nil
}

// Interface Implementation - Q&A State Management

// HasAnsweredQuestion checks if a question has been answered
func (m *StorageMock) HasAnsweredQuestion(ctx context.Context, questionHash string) (bool, error) {
	m.recordCall("HasAnsweredQuestion")

	if err := m.checkError("HasAnsweredQuestion"); err != nil {
		return false, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.questionStates[questionHash]
	if !exists {
		return false, nil
	}

	return state.AnswerDelivered, nil
}

// MarkQuestionAnswered marks a question as answered
func (m *StorageMock) MarkQuestionAnswered(ctx context.Context, state *QuestionState) error {
	m.recordCall("MarkQuestionAnswered")

	if err := m.checkError("MarkQuestionAnswered"); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for duplicate
	if _, exists := m.questionStates[state.QuestionHash]; exists {
		return ErrDuplicateEntry
	}

	// Assign ID and timestamps
	m.questionIDCounter++
	state.ID = m.questionIDCounter
	state.AnswerDelivered = true
	state.ProcessedAt = time.Now()
	state.CreatedAt = time.Now()
	state.UpdatedAt = time.Now()

	m.questionStates[state.QuestionHash] = state

	return nil
}

// GetQuestionState retrieves a question state by hash
func (m *StorageMock) GetQuestionState(ctx context.Context, questionHash string) (*QuestionState, error) {
	m.recordCall("GetQuestionState")

	if err := m.checkError("GetQuestionState"); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.questionStates[questionHash]
	if !exists {
		return nil, ErrQuestionNotFound
	}

	return state, nil
}

// UpdateQuestionState updates an existing question state
func (m *StorageMock) UpdateQuestionState(ctx context.Context, state *QuestionState) error {
	m.recordCall("UpdateQuestionState")

	if err := m.checkError("UpdateQuestionState"); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.questionStates[state.QuestionHash]
	if !exists {
		return ErrQuestionNotFound
	}

	// Update fields
	state.ID = existing.ID
	state.CreatedAt = existing.CreatedAt
	state.UpdatedAt = time.Now()

	m.questionStates[state.QuestionHash] = state

	return nil
}

// DeleteStaleQuestions removes questions older than the specified time
func (m *StorageMock) DeleteStaleQuestions(ctx context.Context, olderThan time.Time) (int64, error) {
	m.recordCall("DeleteStaleQuestions")

	if err := m.checkError("DeleteStaleQuestions"); err != nil {
		return 0, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var deleted int64
	for hash, state := range m.questionStates {
		if state.ProcessedAt.Before(olderThan) {
			delete(m.questionStates, hash)
			deleted++
		}
	}

	return deleted, nil
}

// Interface Implementation - Command Logging

// LogCommand logs a command execution
func (m *StorageMock) LogCommand(ctx context.Context, log *CommandLog) error {
	m.recordCall("LogCommand")

	if err := m.checkError("LogCommand"); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Assign ID and timestamps
	m.commandIDCounter++
	log.ID = m.commandIDCounter
	log.CreatedAt = time.Now()

	if log.ExecutedAt.IsZero() {
		log.ExecutedAt = time.Now()
	}

	if m.commandLogs[log.DocumentID] == nil {
		m.commandLogs[log.DocumentID] = make([]*CommandLog, 0)
	}
	m.commandLogs[log.DocumentID] = append(m.commandLogs[log.DocumentID], log)

	return nil
}

// GetCommandHistory retrieves command history for a document
func (m *StorageMock) GetCommandHistory(ctx context.Context, documentID string, limit int) ([]*CommandLog, error) {
	m.recordCall("GetCommandHistory")

	if err := m.checkError("GetCommandHistory"); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	logs := m.commandLogs[documentID]
	if logs == nil {
		return []*CommandLog{}, nil
	}

	// Sort by executed_at DESC (newest first)
	// For simplicity, assuming logs are already in order
	// In production, would sort by ExecutedAt

	// Apply limit
	if limit > 0 && len(logs) > limit {
		logs = logs[len(logs)-limit:]
	}

	// Return a copy to prevent external modification
	result := make([]*CommandLog, len(logs))
	copy(result, logs)

	// Reverse to get newest first
	for i := 0; i < len(result)/2; i++ {
		j := len(result) - i - 1
		result[i], result[j] = result[j], result[i]
	}

	return result, nil
}

// Interface Implementation - Health and Maintenance

// Ping checks if the storage is available
func (m *StorageMock) Ping(ctx context.Context) error {
	m.recordCall("Ping")
	return m.checkError("Ping")
}

// Close closes the storage (no-op for mock)
func (m *StorageMock) Close() error {
	m.recordCall("Close")
	return m.checkError("Close")
}

// Backup creates a backup of the storage
func (m *StorageMock) Backup(ctx context.Context, destinationPath string) error {
	m.recordCall("Backup")

	if err := m.checkError("Backup"); err != nil {
		return err
	}

	// Mock implementation - just verify path is not empty
	if destinationPath == "" {
		return ErrInvalidInput
	}

	// In a real scenario, would write data to file
	// For mock, just succeed
	return nil
}

// RunInTransaction executes a function within a transaction
// Note: This is a simplified mock implementation
// Real implementation would handle rollback on error
func (m *StorageMock) RunInTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	m.recordCall("RunInTransaction")

	if err := m.checkError("RunInTransaction"); err != nil {
		return err
	}

	m.mu.Lock()
	m.inTransaction = true
	m.mu.Unlock()

	// Execute the function
	err := fn(ctx)

	m.mu.Lock()
	m.inTransaction = false
	m.mu.Unlock()

	return err
}

// Helper Functions

// IsQuestionNotFound checks if error is ErrQuestionNotFound
func IsQuestionNotFound(err error) bool {
	return errors.Is(err, ErrQuestionNotFound)
}

// GenerateQuestionHash generates a hash for a question
// In production, this would be in the qna package with proper normalization
func GenerateQuestionHash(documentID, questionText string) string {
	// Simplified hash generation for testing
	// Real implementation would normalize text and use proper hashing
	return fmt.Sprintf("hash-%s-%s", documentID, questionText)
}

// Helper to create string pointer
func StringPtr(s string) *string {
	return &s
}

// Helper to create int pointer
func IntPtr(i int) *int {
	return &i
}
