package mocks

import (
	"context"
	"testing"
	"time"
)

// Example test showing usage of StorageMock
func TestStorageMock_BasicOperations(t *testing.T) {
	mock := NewStorageMock()
	defer mock.Reset()

	ctx := context.Background()

	// Test: Mark question answered
	t.Run("mark question answered", func(t *testing.T) {
		state := &QuestionState{
			QuestionHash:    "hash-123",
			DocumentID:      "doc-456",
			QuestionText:    "What is REST?",
			AnswerDelivered: false,
			CommentID:       StringPtr("comment-789"),
		}

		err := mock.MarkQuestionAnswered(ctx, state)
		if err != nil {
			t.Fatalf("MarkQuestionAnswered failed: %v", err)
		}

		// Verify ID was assigned
		if state.ID == 0 {
			t.Error("Expected ID to be assigned")
		}

		// Verify timestamps were set
		if state.CreatedAt.IsZero() {
			t.Error("Expected CreatedAt to be set")
		}
		if state.UpdatedAt.IsZero() {
			t.Error("Expected UpdatedAt to be set")
		}
		if state.ProcessedAt.IsZero() {
			t.Error("Expected ProcessedAt to be set")
		}

		// Verify AnswerDelivered was set to true
		if !state.AnswerDelivered {
			t.Error("Expected AnswerDelivered to be true")
		}
	})

	// Test: Check if question answered
	t.Run("has answered question", func(t *testing.T) {
		// Question that was answered in previous test
		answered, err := mock.HasAnsweredQuestion(ctx, "hash-123")
		if err != nil {
			t.Fatalf("HasAnsweredQuestion failed: %v", err)
		}

		if !answered {
			t.Error("Expected question to be answered")
		}

		// Question that hasn't been answered
		answered, err = mock.HasAnsweredQuestion(ctx, "nonexistent-hash")
		if err != nil {
			t.Fatalf("HasAnsweredQuestion failed: %v", err)
		}

		if answered {
			t.Error("Expected question to not be answered")
		}
	})

	// Test: Get question state
	t.Run("get question state", func(t *testing.T) {
		state, err := mock.GetQuestionState(ctx, "hash-123")
		if err != nil {
			t.Fatalf("GetQuestionState failed: %v", err)
		}

		if state.QuestionHash != "hash-123" {
			t.Errorf("Expected hash 'hash-123', got '%s'", state.QuestionHash)
		}

		if state.DocumentID != "doc-456" {
			t.Errorf("Expected document ID 'doc-456', got '%s'", state.DocumentID)
		}

		if state.QuestionText != "What is REST?" {
			t.Errorf("Expected question text 'What is REST?', got '%s'", state.QuestionText)
		}

		// Try to get non-existent state
		_, err = mock.GetQuestionState(ctx, "nonexistent")
		if err != ErrQuestionNotFound {
			t.Errorf("Expected ErrQuestionNotFound, got %v", err)
		}
	})

	// Test: Update question state
	t.Run("update question state", func(t *testing.T) {
		// Get existing state
		state, err := mock.GetQuestionState(ctx, "hash-123")
		if err != nil {
			t.Fatalf("GetQuestionState failed: %v", err)
		}

		// Update it
		state.RetryCount = 3
		errorMsg := "Temporary failure"
		state.LastError = &errorMsg

		err = mock.UpdateQuestionState(ctx, state)
		if err != nil {
			t.Fatalf("UpdateQuestionState failed: %v", err)
		}

		// Verify update
		updated, err := mock.GetQuestionState(ctx, "hash-123")
		if err != nil {
			t.Fatalf("GetQuestionState failed: %v", err)
		}

		if updated.RetryCount != 3 {
			t.Errorf("Expected retry count 3, got %d", updated.RetryCount)
		}

		if updated.LastError == nil || *updated.LastError != "Temporary failure" {
			t.Error("Expected LastError to be updated")
		}

		// Try to update non-existent state
		newState := &QuestionState{
			QuestionHash: "nonexistent",
		}
		err = mock.UpdateQuestionState(ctx, newState)
		if err != ErrQuestionNotFound {
			t.Errorf("Expected ErrQuestionNotFound, got %v", err)
		}
	})

	// Test: Delete stale questions
	t.Run("delete stale questions", func(t *testing.T) {
		// Add some old questions
		oldState1 := &QuestionState{
			QuestionHash:    "old-hash-1",
			DocumentID:      "doc-old",
			QuestionText:    "Old question 1",
			ProcessedAt:     time.Now().Add(-48 * time.Hour),
			AnswerDelivered: true,
		}
		mock.SeedQuestionState(oldState1)

		oldState2 := &QuestionState{
			QuestionHash:    "old-hash-2",
			DocumentID:      "doc-old",
			QuestionText:    "Old question 2",
			ProcessedAt:     time.Now().Add(-72 * time.Hour),
			AnswerDelivered: true,
		}
		mock.SeedQuestionState(oldState2)

		// Add a recent question
		recentState := &QuestionState{
			QuestionHash:    "recent-hash",
			DocumentID:      "doc-recent",
			QuestionText:    "Recent question",
			ProcessedAt:     time.Now().Add(-1 * time.Hour),
			AnswerDelivered: true,
		}
		mock.SeedQuestionState(recentState)

		// Delete questions older than 24 hours
		cutoff := time.Now().Add(-24 * time.Hour)
		deleted, err := mock.DeleteStaleQuestions(ctx, cutoff)
		if err != nil {
			t.Fatalf("DeleteStaleQuestions failed: %v", err)
		}

		if deleted != 2 {
			t.Errorf("Expected 2 deleted questions, got %d", deleted)
		}

		// Verify old questions are gone
		_, err = mock.GetQuestionState(ctx, "old-hash-1")
		if err != ErrQuestionNotFound {
			t.Error("Expected old question 1 to be deleted")
		}

		_, err = mock.GetQuestionState(ctx, "old-hash-2")
		if err != ErrQuestionNotFound {
			t.Error("Expected old question 2 to be deleted")
		}

		// Verify recent question still exists
		_, err = mock.GetQuestionState(ctx, "recent-hash")
		if err != nil {
			t.Error("Expected recent question to still exist")
		}
	})

	// Test: Command logging
	t.Run("log command", func(t *testing.T) {
		log := &CommandLog{
			DocumentID:      "doc-123",
			CommandType:     "classify",
			CommandArgs:     StringPtr("--collection=engineering"),
			Status:          CommandStatusSuccess,
			ExecutionTimeMs: IntPtr(150),
		}

		err := mock.LogCommand(ctx, log)
		if err != nil {
			t.Fatalf("LogCommand failed: %v", err)
		}

		// Verify ID was assigned
		if log.ID == 0 {
			t.Error("Expected ID to be assigned")
		}

		// Verify timestamps were set
		if log.CreatedAt.IsZero() {
			t.Error("Expected CreatedAt to be set")
		}
		if log.ExecutedAt.IsZero() {
			t.Error("Expected ExecutedAt to be set")
		}
	})

	// Test: Get command history
	t.Run("get command history", func(t *testing.T) {
		// Add multiple commands for same document
		for range 5 {
			log := &CommandLog{
				DocumentID:  "doc-history",
				CommandType: "test",
				Status:      CommandStatusSuccess,
			}
			_ = mock.LogCommand(ctx, log)
			time.Sleep(1 * time.Millisecond) // Ensure different timestamps
		}

		// Get all history
		history, err := mock.GetCommandHistory(ctx, "doc-history", 0)
		if err != nil {
			t.Fatalf("GetCommandHistory failed: %v", err)
		}

		if len(history) != 5 {
			t.Errorf("Expected 5 commands in history, got %d", len(history))
		}

		// Get limited history
		history, err = mock.GetCommandHistory(ctx, "doc-history", 3)
		if err != nil {
			t.Fatalf("GetCommandHistory failed: %v", err)
		}

		if len(history) != 3 {
			t.Errorf("Expected 3 commands in limited history, got %d", len(history))
		}

		// Get history for non-existent document
		history, err = mock.GetCommandHistory(ctx, "nonexistent", 10)
		if err != nil {
			t.Fatalf("GetCommandHistory failed: %v", err)
		}

		if len(history) != 0 {
			t.Errorf("Expected empty history, got %d commands", len(history))
		}
	})
}

// Example test showing error scenarios
func TestStorageMock_ErrorScenarios(t *testing.T) {
	mock := NewStorageMock()
	defer mock.Reset()

	ctx := context.Background()

	// Test: Duplicate entry error
	t.Run("duplicate entry error", func(t *testing.T) {
		state := &QuestionState{
			QuestionHash: "duplicate-hash",
			DocumentID:   "doc-123",
			QuestionText: "Duplicate question",
		}

		// First insert should succeed
		err := mock.MarkQuestionAnswered(ctx, state)
		if err != nil {
			t.Fatalf("First MarkQuestionAnswered failed: %v", err)
		}

		// Second insert with same hash should fail
		state2 := &QuestionState{
			QuestionHash: "duplicate-hash",
			DocumentID:   "doc-456",
			QuestionText: "Different question, same hash",
		}

		err = mock.MarkQuestionAnswered(ctx, state2)
		if err != ErrDuplicateEntry {
			t.Errorf("Expected ErrDuplicateEntry, got %v", err)
		}
	})

	// Test: Failure mode
	t.Run("failure mode", func(t *testing.T) {
		mock.SetFailureMode(true)

		state := &QuestionState{
			QuestionHash: "fail-hash",
			DocumentID:   "doc-fail",
			QuestionText: "This should fail",
		}

		err := mock.MarkQuestionAnswered(ctx, state)
		if err != ErrDatabaseLocked {
			t.Errorf("Expected ErrDatabaseLocked, got %v", err)
		}

		mock.SetFailureMode(false)
	})

	// Test: Method-specific error
	t.Run("method specific error", func(t *testing.T) {
		customErr := ErrInvalidInput
		mock.SetMethodError("GetQuestionState", customErr)

		_, err := mock.GetQuestionState(ctx, "any-hash")
		if err != customErr {
			t.Errorf("Expected custom error, got %v", err)
		}

		// Other methods should still work
		err = mock.Ping(ctx)
		if err != nil {
			t.Errorf("Expected Ping to succeed, got %v", err)
		}
	})

	// Test: Invalid backup path
	t.Run("invalid backup path", func(t *testing.T) {
		err := mock.Backup(ctx, "")
		if err != ErrInvalidInput {
			t.Errorf("Expected ErrInvalidInput, got %v", err)
		}

		// Valid path should succeed
		err = mock.Backup(ctx, "/tmp/backup.db")
		if err != nil {
			t.Errorf("Expected Backup to succeed, got %v", err)
		}
	})
}

// Example test showing seeding and helper methods
func TestStorageMock_SeedingAndHelpers(t *testing.T) {
	mock := NewStorageMock()
	defer mock.Reset()

	ctx := context.Background()

	// Test: Seed question state
	t.Run("seed question state", func(t *testing.T) {
		state := &QuestionState{
			QuestionHash:    "seeded-hash",
			DocumentID:      "doc-seeded",
			QuestionText:    "Seeded question",
			AnswerDelivered: true,
			CommentID:       StringPtr("comment-123"),
			RetryCount:      2,
		}

		mock.SeedQuestionState(state)

		// Verify it was seeded
		retrieved, err := mock.GetQuestionState(ctx, "seeded-hash")
		if err != nil {
			t.Fatalf("GetQuestionState failed: %v", err)
		}

		if retrieved.RetryCount != 2 {
			t.Errorf("Expected retry count 2, got %d", retrieved.RetryCount)
		}

		// Verify timestamps were auto-set
		if retrieved.CreatedAt.IsZero() {
			t.Error("Expected CreatedAt to be set")
		}
	})

	// Test: Seed command log
	t.Run("seed command log", func(t *testing.T) {
		log := &CommandLog{
			DocumentID:   "doc-seeded-log",
			CommandType:  "test-command",
			Status:       CommandStatusSuccess,
			ErrorMessage: StringPtr("No error"),
		}

		mock.SeedCommandLog(log)

		// Verify it was seeded
		history, err := mock.GetCommandHistory(ctx, "doc-seeded-log", 10)
		if err != nil {
			t.Fatalf("GetCommandHistory failed: %v", err)
		}

		if len(history) != 1 {
			t.Errorf("Expected 1 command in history, got %d", len(history))
		}

		if history[0].CommandType != "test-command" {
			t.Errorf("Expected command type 'test-command', got '%s'", history[0].CommandType)
		}
	})

	// Test: Generate question hash helper
	t.Run("generate question hash", func(t *testing.T) {
		hash := GenerateQuestionHash("doc-123", "What is REST?")
		if hash == "" {
			t.Error("Expected non-empty hash")
		}

		// Same inputs should produce same hash
		hash2 := GenerateQuestionHash("doc-123", "What is REST?")
		if hash != hash2 {
			t.Error("Expected same hash for same inputs")
		}

		// Different inputs should produce different hash
		hash3 := GenerateQuestionHash("doc-456", "What is REST?")
		if hash == hash3 {
			t.Error("Expected different hash for different document ID")
		}
	})

	// Test: Get question state by hash (testing helper)
	t.Run("get question state by hash helper", func(t *testing.T) {
		state := &QuestionState{
			QuestionHash: "helper-hash",
			DocumentID:   "doc-helper",
			QuestionText: "Helper test",
		}
		mock.SeedQuestionState(state)

		// Use helper method
		retrieved := mock.GetQuestionStateByHash("helper-hash")
		if retrieved == nil {
			t.Fatal("Expected to retrieve seeded state")
		}

		if retrieved.DocumentID != "doc-helper" {
			t.Errorf("Expected document ID 'doc-helper', got '%s'", retrieved.DocumentID)
		}

		// Non-existent hash
		retrieved = mock.GetQuestionStateByHash("nonexistent")
		if retrieved != nil {
			t.Error("Expected nil for non-existent hash")
		}
	})

	// Test: Clear method
	t.Run("clear storage", func(t *testing.T) {
		// Add some data
		state := &QuestionState{
			QuestionHash: "clear-test",
			DocumentID:   "doc-clear",
			QuestionText: "Clear test",
		}
		mock.SeedQuestionState(state)

		log := &CommandLog{
			DocumentID:  "doc-clear",
			CommandType: "test",
			Status:      CommandStatusSuccess,
		}
		mock.SeedCommandLog(log)

		// Clear storage
		mock.Clear()

		// Verify data is gone
		_, err := mock.GetQuestionState(ctx, "clear-test")
		if err != ErrQuestionNotFound {
			t.Error("Expected data to be cleared")
		}

		history, err := mock.GetCommandHistory(ctx, "doc-clear", 10)
		if err != nil {
			t.Fatalf("GetCommandHistory failed: %v", err)
		}
		if len(history) != 0 {
			t.Error("Expected command history to be cleared")
		}
	})
}

// Example test showing call tracking
func TestStorageMock_CallTracking(t *testing.T) {
	mock := NewStorageMock()
	defer mock.Reset()

	ctx := context.Background()

	// Make several calls
	state := &QuestionState{
		QuestionHash: "track-hash",
		DocumentID:   "doc-track",
		QuestionText: "Track test",
	}

	_ = mock.MarkQuestionAnswered(ctx, state)
	_, _ = mock.HasAnsweredQuestion(ctx, "track-hash")
	_, _ = mock.HasAnsweredQuestion(ctx, "track-hash")
	_, _ = mock.GetQuestionState(ctx, "track-hash")

	// Verify call counts
	markCount := mock.GetCallCount("MarkQuestionAnswered")
	if markCount != 1 {
		t.Errorf("Expected 1 MarkQuestionAnswered call, got %d", markCount)
	}

	hasCount := mock.GetCallCount("HasAnsweredQuestion")
	if hasCount != 2 {
		t.Errorf("Expected 2 HasAnsweredQuestion calls, got %d", hasCount)
	}

	getCount := mock.GetCallCount("GetQuestionState")
	if getCount != 1 {
		t.Errorf("Expected 1 GetQuestionState call, got %d", getCount)
	}

	// Non-called method should have 0 count
	deleteCount := mock.GetCallCount("DeleteStaleQuestions")
	if deleteCount != 0 {
		t.Errorf("Expected 0 DeleteStaleQuestions calls, got %d", deleteCount)
	}
}

// Example test showing integration scenario
func TestStorageMock_IntegrationScenario(t *testing.T) {
	mock := NewStorageMock()
	defer mock.Reset()

	ctx := context.Background()

	// Scenario: Question deduplication workflow
	t.Run("question deduplication workflow", func(t *testing.T) {
		documentID := "doc-integration"
		questionText := "What is the purpose of this document?"

		// 1. Generate question hash
		questionHash := GenerateQuestionHash(documentID, questionText)

		// 2. Check if question already answered
		answered, err := mock.HasAnsweredQuestion(ctx, questionHash)
		if err != nil {
			t.Fatalf("HasAnsweredQuestion failed: %v", err)
		}

		if answered {
			t.Fatal("Expected question to not be answered yet")
		}

		// 3. Process question and mark as answered
		state := &QuestionState{
			QuestionHash: questionHash,
			DocumentID:   documentID,
			QuestionText: questionText,
			CommentID:    StringPtr("comment-integration-123"),
		}

		err = mock.MarkQuestionAnswered(ctx, state)
		if err != nil {
			t.Fatalf("MarkQuestionAnswered failed: %v", err)
		}

		// 4. Log the command execution
		log := &CommandLog{
			DocumentID:      documentID,
			CommandType:     "answer_question",
			CommandArgs:     StringPtr(questionText),
			Status:          CommandStatusSuccess,
			ExecutionTimeMs: IntPtr(250),
		}

		err = mock.LogCommand(ctx, log)
		if err != nil {
			t.Fatalf("LogCommand failed: %v", err)
		}

		// 5. Verify question is now marked as answered
		answered, err = mock.HasAnsweredQuestion(ctx, questionHash)
		if err != nil {
			t.Fatalf("HasAnsweredQuestion failed: %v", err)
		}

		if !answered {
			t.Fatal("Expected question to be answered")
		}

		// 6. Verify we can retrieve the state
		retrievedState, err := mock.GetQuestionState(ctx, questionHash)
		if err != nil {
			t.Fatalf("GetQuestionState failed: %v", err)
		}

		if retrievedState.CommentID == nil || *retrievedState.CommentID != "comment-integration-123" {
			t.Error("Expected CommentID to be preserved")
		}

		// 7. Verify command history
		history, err := mock.GetCommandHistory(ctx, documentID, 10)
		if err != nil {
			t.Fatalf("GetCommandHistory failed: %v", err)
		}

		if len(history) != 1 {
			t.Errorf("Expected 1 command in history, got %d", len(history))
		}

		if history[0].Status != CommandStatusSuccess {
			t.Errorf("Expected status 'success', got '%s'", history[0].Status)
		}

		// 8. Try to answer the same question again (should be prevented)
		sameQuestionHash := GenerateQuestionHash(documentID, questionText)
		answered, err = mock.HasAnsweredQuestion(ctx, sameQuestionHash)
		if err != nil {
			t.Fatalf("HasAnsweredQuestion failed: %v", err)
		}

		if !answered {
			t.Error("Expected duplicate question to be detected")
		}

		t.Log("Integration scenario completed successfully:")
		t.Logf("  - Question hash: %s", questionHash)
		t.Logf("  - State ID: %d", retrievedState.ID)
		t.Logf("  - Commands logged: %d", len(history))
	})
}
