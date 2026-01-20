package mocks

import (
	"context"
	"testing"
	"time"
)

// Example test showing usage of OutlineMock
func TestOutlineMock_BasicOperations(t *testing.T) {
	// Create mock client
	mock := NewOutlineMock()
	defer mock.Reset()

	ctx := context.Background()

	// Test: Add and retrieve collection
	t.Run("collection operations", func(t *testing.T) {
		collection := mock.AddCollection("col-123", "Engineering", "Engineering documentation")

		// Retrieve collection
		retrieved, err := mock.GetCollection(ctx, collection.ID)
		if err != nil {
			t.Fatalf("GetCollection failed: %v", err)
		}

		if retrieved.Name != "Engineering" {
			t.Errorf("Expected collection name 'Engineering', got '%s'", retrieved.Name)
		}

		// List collections
		collections, err := mock.ListCollections(ctx)
		if err != nil {
			t.Fatalf("ListCollections failed: %v", err)
		}

		if len(collections) != 1 {
			t.Errorf("Expected 1 collection, got %d", len(collections))
		}
	})

	// Test: Add and retrieve document
	t.Run("document operations", func(t *testing.T) {
		collection := mock.AddCollection("col-456", "Product", "Product specs")
		doc := mock.AddDocument("doc-789", collection.ID, "API Design", "REST API guidelines")

		// Retrieve document
		retrieved, err := mock.GetDocument(ctx, doc.ID)
		if err != nil {
			t.Fatalf("GetDocument failed: %v", err)
		}

		if retrieved.Title != "API Design" {
			t.Errorf("Expected document title 'API Design', got '%s'", retrieved.Title)
		}

		// List documents in collection
		docs, err := mock.ListDocuments(ctx, collection.ID)
		if err != nil {
			t.Fatalf("ListDocuments failed: %v", err)
		}

		if len(docs) != 1 {
			t.Errorf("Expected 1 document, got %d", len(docs))
		}
	})

	// Test: Create document
	t.Run("create document", func(t *testing.T) {
		collection := mock.AddCollection("col-999", "Guides", "How-to guides")

		req := &CreateDocumentRequest{
			CollectionID: collection.ID,
			Title:        "Getting Started",
			Text:         "Welcome to our platform",
			Publish:      true,
		}

		doc, err := mock.CreateDocument(ctx, req)
		if err != nil {
			t.Fatalf("CreateDocument failed: %v", err)
		}

		if doc.Title != "Getting Started" {
			t.Errorf("Expected title 'Getting Started', got '%s'", doc.Title)
		}

		if doc.PublishedAt == nil {
			t.Error("Expected document to be published")
		}
	})

	// Test: Update document
	t.Run("update document", func(t *testing.T) {
		collection := mock.AddCollection("col-111", "Updates", "Update docs")
		doc := mock.AddDocument("doc-222", collection.ID, "Original Title", "Original content")

		updateReq := &UpdateDocumentRequest{
			Title: "Updated Title",
			Text:  "Updated content",
		}

		updated, err := mock.UpdateDocument(ctx, doc.ID, updateReq)
		if err != nil {
			t.Fatalf("UpdateDocument failed: %v", err)
		}

		if updated.Title != "Updated Title" {
			t.Errorf("Expected title 'Updated Title', got '%s'", updated.Title)
		}

		if updated.Text != "Updated content" {
			t.Errorf("Expected text 'Updated content', got '%s'", updated.Text)
		}
	})

	// Test: Move document
	t.Run("move document", func(t *testing.T) {
		col1 := mock.AddCollection("col-source", "Source", "Source collection")
		col2 := mock.AddCollection("col-dest", "Destination", "Destination collection")
		doc := mock.AddDocument("doc-move", col1.ID, "Movable Doc", "Content")

		err := mock.MoveDocument(ctx, doc.ID, col2.ID)
		if err != nil {
			t.Fatalf("MoveDocument failed: %v", err)
		}

		// Verify document moved
		retrieved, err := mock.GetDocument(ctx, doc.ID)
		if err != nil {
			t.Fatalf("GetDocument failed: %v", err)
		}

		if retrieved.CollectionID != col2.ID {
			t.Errorf("Expected collection ID '%s', got '%s'", col2.ID, retrieved.CollectionID)
		}
	})

	// Test: Comments
	t.Run("comment operations", func(t *testing.T) {
		collection := mock.AddCollection("col-comments", "Comments", "Test comments")
		doc := mock.AddDocument("doc-comments", collection.ID, "Doc with Comments", "Content")

		// Create comment
		commentReq := &CreateCommentRequest{
			DocumentID: doc.ID,
			Data: CommentContent{
				Type: "doc",
				Content: []ContentNode{
					{
						Type: "paragraph",
						Content: []ContentNode{
							{Type: "text", Text: "This is a test comment"},
						},
					},
				},
			},
		}

		comment, err := mock.CreateComment(ctx, commentReq)
		if err != nil {
			t.Fatalf("CreateComment failed: %v", err)
		}

		if comment.DocumentID != doc.ID {
			t.Errorf("Expected document ID '%s', got '%s'", doc.ID, comment.DocumentID)
		}

		// List comments
		comments, err := mock.ListComments(ctx, doc.ID)
		if err != nil {
			t.Fatalf("ListComments failed: %v", err)
		}

		if len(comments) != 1 {
			t.Errorf("Expected 1 comment, got %d", len(comments))
		}
	})

	// Test: Search documents
	t.Run("search documents", func(t *testing.T) {
		// Use a fresh mock to avoid interference from other tests
		searchMock := NewOutlineMock()
		collection := searchMock.AddCollection("col-search", "Search", "Search test")
		searchMock.AddDocument("doc-search-1", collection.ID, "API Guide", "REST API design patterns")
		searchMock.AddDocument("doc-search-2", collection.ID, "Database Guide", "SQL best practices")

		result, err := searchMock.SearchDocuments(ctx, "API", nil)
		if err != nil {
			t.Fatalf("SearchDocuments failed: %v", err)
		}

		if result.TotalCount != 1 {
			t.Errorf("Expected 1 search result, got %d", result.TotalCount)
		}
	})
}

// Example test showing error scenarios
func TestOutlineMock_ErrorScenarios(t *testing.T) {
	mock := NewOutlineMock()
	defer mock.Reset()

	ctx := context.Background()

	// Test: Not found error
	t.Run("not found error", func(t *testing.T) {
		_, err := mock.GetDocument(ctx, "nonexistent")
		if err != ErrNotFound {
			t.Errorf("Expected ErrNotFound, got %v", err)
		}
	})

	// Test: Failure mode
	t.Run("failure mode", func(t *testing.T) {
		mock.SetFailureMode(true)

		_, err := mock.ListCollections(ctx)
		if err != ErrServerError {
			t.Errorf("Expected ErrServerError, got %v", err)
		}

		mock.SetFailureMode(false)
	})

	// Test: Rate limiting
	t.Run("rate limiting", func(t *testing.T) {
		mock.SetRateLimited(true)

		_, err := mock.GetCollection(ctx, "any-id")
		if err != ErrRateLimited {
			t.Errorf("Expected ErrRateLimited, got %v", err)
		}

		mock.SetRateLimited(false)
	})

	// Test: Specific method error
	t.Run("specific method error", func(t *testing.T) {
		customErr := ErrUnauthorized
		mock.SetGetDocumentError(customErr)

		_, err := mock.GetDocument(ctx, "any-id")
		if err != customErr {
			t.Errorf("Expected custom error, got %v", err)
		}

		// Other methods should still work
		_, err = mock.ListCollections(ctx)
		if err != nil {
			t.Errorf("Expected ListCollections to succeed, got %v", err)
		}
	})

	// Test: Request delay
	t.Run("request delay", func(t *testing.T) {
		mock.AddCollection("col-delay", "Delay Test", "Test delay")

		delay := 50 * time.Millisecond
		mock.SetRequestDelay(delay)

		start := time.Now()
		_, _ = mock.GetCollection(ctx, "col-delay")
		elapsed := time.Since(start)

		if elapsed < delay {
			t.Errorf("Expected delay of at least %v, got %v", delay, elapsed)
		}

		mock.SetRequestDelay(0)
	})
}

// Example test showing call tracking
func TestOutlineMock_CallTracking(t *testing.T) {
	mock := NewOutlineMock()
	defer mock.Reset()

	ctx := context.Background()

	collection := mock.AddCollection("col-track", "Tracking", "Call tracking test")

	// Make several calls
	_, _ = mock.GetCollection(ctx, collection.ID)
	_, _ = mock.GetCollection(ctx, collection.ID)
	_, _ = mock.ListCollections(ctx)

	// Verify call counts
	getCount := mock.GetCallCount("GetCollection")
	if getCount != 2 {
		t.Errorf("Expected 2 GetCollection calls, got %d", getCount)
	}

	listCount := mock.GetCallCount("ListCollections")
	if listCount != 1 {
		t.Errorf("Expected 1 ListCollections call, got %d", listCount)
	}

	// Non-called method should have 0 count
	createCount := mock.GetCallCount("CreateDocument")
	if createCount != 0 {
		t.Errorf("Expected 0 CreateDocument calls, got %d", createCount)
	}
}

// Example test showing integration scenario
func TestOutlineMock_IntegrationScenario(t *testing.T) {
	mock := NewOutlineMock()
	defer mock.Reset()

	ctx := context.Background()

	// Scenario: Process a new document
	t.Run("full document lifecycle", func(t *testing.T) {
		// 1. List collections to find target
		collections, err := mock.ListCollections(ctx)
		if err != nil {
			t.Fatalf("Failed to list collections: %v", err)
		}

		if len(collections) == 0 {
			// Create a collection if none exist
			mock.AddCollection("col-new", "New Docs", "New documentation")
			collections, _ = mock.ListCollections(ctx)
		}

		targetCollection := collections[0]

		// 2. Create a new document
		createReq := &CreateDocumentRequest{
			CollectionID: targetCollection.ID,
			Title:        "Integration Test Doc",
			Text:         "This document tests the full lifecycle",
			Publish:      true,
		}

		doc, err := mock.CreateDocument(ctx, createReq)
		if err != nil {
			t.Fatalf("Failed to create document: %v", err)
		}

		// 3. Verify document was created
		retrieved, err := mock.GetDocument(ctx, doc.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve document: %v", err)
		}

		if retrieved.ID != doc.ID {
			t.Errorf("Document ID mismatch: expected %s, got %s", doc.ID, retrieved.ID)
		}

		// 4. Add a comment
		commentReq := &CreateCommentRequest{
			DocumentID: doc.ID,
			Data: CommentContent{
				Type: "doc",
				Content: []ContentNode{
					{
						Type: "paragraph",
						Content: []ContentNode{
							{Type: "text", Text: "Reviewed and approved"},
						},
					},
				},
			},
		}

		comment, err := mock.CreateComment(ctx, commentReq)
		if err != nil {
			t.Fatalf("Failed to create comment: %v", err)
		}

		// 5. Verify comment exists
		comments, err := mock.ListComments(ctx, doc.ID)
		if err != nil {
			t.Fatalf("Failed to list comments: %v", err)
		}

		if len(comments) != 1 {
			t.Errorf("Expected 1 comment, got %d", len(comments))
		}

		if comments[0].ID != comment.ID {
			t.Errorf("Comment ID mismatch: expected %s, got %s", comment.ID, comments[0].ID)
		}

		// 6. Update the document
		updateReq := &UpdateDocumentRequest{
			Title: "Updated Integration Test Doc",
		}

		updated, err := mock.UpdateDocument(ctx, doc.ID, updateReq)
		if err != nil {
			t.Fatalf("Failed to update document: %v", err)
		}

		if updated.Title != updateReq.Title {
			t.Errorf("Title not updated: expected %s, got %s", updateReq.Title, updated.Title)
		}

		// 7. Search for the document
		searchResult, err := mock.SearchDocuments(ctx, "Integration", nil)
		if err != nil {
			t.Fatalf("Failed to search documents: %v", err)
		}

		if searchResult.TotalCount < 1 {
			t.Error("Expected to find the document in search results")
		}
	})
}
