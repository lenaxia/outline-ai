package mocks

import (
	"context"
	"testing"
)

// Example test showing usage of AIMock
func TestAIMock_BasicOperations(t *testing.T) {
	mock := NewAIMock()
	defer mock.Reset()

	ctx := context.Background()

	// Test: Classification with default response
	t.Run("classify document with defaults", func(t *testing.T) {
		req := &ClassificationRequest{
			DocumentTitle:   "API Design Guide",
			DocumentContent: "This guide covers REST API best practices",
			Taxonomy: &TaxonomyContext{
				Collections: []TaxonomyCollection{
					{
						ID:          "engineering-docs",
						Name:        "Engineering",
						Description: "Engineering documentation",
					},
				},
			},
		}

		resp, err := mock.ClassifyDocument(ctx, req)
		if err != nil {
			t.Fatalf("ClassifyDocument failed: %v", err)
		}

		if resp.CollectionID != "default-collection" {
			t.Errorf("Expected default-collection, got %s", resp.CollectionID)
		}

		if resp.Confidence <= 0 || resp.Confidence > 1 {
			t.Errorf("Invalid confidence: %f", resp.Confidence)
		}

		if len(resp.SearchTerms) == 0 {
			t.Error("Expected search terms to be populated")
		}
	})

	// Test: Custom classification response
	t.Run("classify document with custom response", func(t *testing.T) {
		customResp := &ClassificationResponse{
			CollectionID: "custom-collection",
			Confidence:   0.95,
			Reasoning:    "Custom reasoning",
			SearchTerms:  []string{"custom", "terms"},
			Alternatives: []AlternativeClassification{
				{
					CollectionID: "alt-collection",
					Confidence:   0.75,
					Reasoning:    "Alternative option",
				},
			},
		}

		mock.SetClassificationResponse(customResp)

		req := &ClassificationRequest{
			DocumentTitle:   "Test Document",
			DocumentContent: "Test content",
			Taxonomy: &TaxonomyContext{
				Collections: []TaxonomyCollection{
					{ID: "custom-collection", Name: "Custom", Description: "Custom collection"},
				},
			},
		}

		resp, err := mock.ClassifyDocument(ctx, req)
		if err != nil {
			t.Fatalf("ClassifyDocument failed: %v", err)
		}

		if resp.CollectionID != "custom-collection" {
			t.Errorf("Expected custom-collection, got %s", resp.CollectionID)
		}

		if resp.Confidence != 0.95 {
			t.Errorf("Expected confidence 0.95, got %f", resp.Confidence)
		}

		if len(resp.Alternatives) != 1 {
			t.Errorf("Expected 1 alternative, got %d", len(resp.Alternatives))
		}
	})

	// Test: Question answering
	t.Run("answer question", func(t *testing.T) {
		req := &QuestionRequest{
			Question: "What is REST?",
			ContextDocs: []ContextDocument{
				{
					Title:   "REST API Guide",
					Excerpt: "REST stands for Representational State Transfer...",
					URL:     "https://example.com/rest-guide",
				},
			},
		}

		resp, err := mock.AnswerQuestion(ctx, req)
		if err != nil {
			t.Fatalf("AnswerQuestion failed: %v", err)
		}

		if resp.Answer == "" {
			t.Error("Expected non-empty answer")
		}

		if resp.Confidence <= 0 || resp.Confidence > 1 {
			t.Errorf("Invalid confidence: %f", resp.Confidence)
		}

		if len(resp.Citations) == 0 {
			t.Error("Expected citations to be populated")
		}
	})

	// Test: Summary generation
	t.Run("generate summary", func(t *testing.T) {
		req := &SummaryRequest{
			DocumentTitle:   "Long Document",
			DocumentContent: "This is a very long document with lots of content...",
		}

		resp, err := mock.GenerateSummary(ctx, req)
		if err != nil {
			t.Fatalf("GenerateSummary failed: %v", err)
		}

		if resp.Summary == "" {
			t.Error("Expected non-empty summary")
		}
	})

	// Test: Title enhancement
	t.Run("enhance title", func(t *testing.T) {
		req := &TitleRequest{
			CurrentTitle:    "doc1",
			DocumentContent: "This document explains REST APIs in detail...",
		}

		resp, err := mock.EnhanceTitle(ctx, req)
		if err != nil {
			t.Fatalf("EnhanceTitle failed: %v", err)
		}

		if resp.SuggestedTitle == "" {
			t.Error("Expected non-empty suggested title")
		}

		if resp.Confidence <= 0 || resp.Confidence > 1 {
			t.Errorf("Invalid confidence: %f", resp.Confidence)
		}
	})

	// Test: Search terms generation
	t.Run("generate search terms", func(t *testing.T) {
		req := &SearchTermsRequest{
			DocumentTitle:   "API Documentation",
			DocumentContent: "Guide to building REST APIs with authentication",
		}

		resp, err := mock.GenerateSearchTerms(ctx, req)
		if err != nil {
			t.Fatalf("GenerateSearchTerms failed: %v", err)
		}

		if len(resp.SearchTerms) == 0 {
			t.Error("Expected search terms to be populated")
		}
	})

	// Test: Find related documents
	t.Run("find related documents", func(t *testing.T) {
		req := &RelatedDocsRequest{
			DocumentTitle:   "API Design",
			DocumentContent: "REST API design patterns",
			AvailableDocs:   []string{"API Guide", "Database Guide", "Frontend Guide"},
		}

		resp, err := mock.FindRelatedDocuments(ctx, req)
		if err != nil {
			t.Fatalf("FindRelatedDocuments failed: %v", err)
		}

		if len(resp.RelatedDocuments) == 0 {
			t.Error("Expected related documents to be populated")
		}

		for _, doc := range resp.RelatedDocuments {
			if doc.Relevance <= 0 || doc.Relevance > 1 {
				t.Errorf("Invalid relevance score: %f", doc.Relevance)
			}
		}
	})
}

// Example test showing error scenarios
func TestAIMock_ErrorScenarios(t *testing.T) {
	mock := NewAIMock()
	defer mock.Reset()

	ctx := context.Background()

	// Test: Circuit breaker open
	t.Run("circuit breaker open", func(t *testing.T) {
		mock.SetCircuitBreakerOpen(true)

		req := &ClassificationRequest{
			DocumentTitle:   "Test",
			DocumentContent: "Test",
			Taxonomy:        &TaxonomyContext{Collections: []TaxonomyCollection{}},
		}

		_, err := mock.ClassifyDocument(ctx, req)
		if err != ErrCircuitBreakerOpen {
			t.Errorf("Expected ErrCircuitBreakerOpen, got %v", err)
		}

		mock.SetCircuitBreakerOpen(false)
	})

	// Test: Token limit exceeded
	t.Run("token limit exceeded", func(t *testing.T) {
		mock.SetTokenLimitExceeded(true)

		req := &SummaryRequest{
			DocumentTitle:   "Very Long Document",
			DocumentContent: "Lots of content...",
		}

		_, err := mock.GenerateSummary(ctx, req)
		if err != ErrTokenLimitExceeded {
			t.Errorf("Expected ErrTokenLimitExceeded, got %v", err)
		}

		mock.SetTokenLimitExceeded(false)
	})

	// Test: Rate limited
	t.Run("rate limited", func(t *testing.T) {
		mock.SetRateLimited(true)

		req := &QuestionRequest{
			Question:    "Test question?",
			ContextDocs: []ContextDocument{},
		}

		_, err := mock.AnswerQuestion(ctx, req)
		if err != ErrAIRateLimited {
			t.Errorf("Expected ErrAIRateLimited, got %v", err)
		}

		mock.SetRateLimited(false)
	})

	// Test: Timeout error
	t.Run("timeout error", func(t *testing.T) {
		mock.SetTimeoutError(true)

		req := &TitleRequest{
			CurrentTitle:    "Test",
			DocumentContent: "Content",
		}

		_, err := mock.EnhanceTitle(ctx, req)
		if err != ErrTimeout {
			t.Errorf("Expected ErrTimeout, got %v", err)
		}

		mock.SetTimeoutError(false)
	})

	// Test: Method-specific error
	t.Run("method specific error", func(t *testing.T) {
		customErr := ErrInvalidResponse
		mock.SetMethodError("GenerateSearchTerms", customErr)

		req := &SearchTermsRequest{
			DocumentTitle:   "Test",
			DocumentContent: "Content",
		}

		_, err := mock.GenerateSearchTerms(ctx, req)
		if err != customErr {
			t.Errorf("Expected custom error, got %v", err)
		}

		// Other methods should still work
		pingErr := mock.Ping(ctx)
		if pingErr != nil {
			t.Errorf("Expected Ping to succeed, got %v", pingErr)
		}
	})
}

// Example test showing deterministic mode
func TestAIMock_DeterministicMode(t *testing.T) {
	mock := NewAIMock()
	defer mock.Reset()

	ctx := context.Background()

	mock.SetDeterministicMode(true)

	// Test: Deterministic classification
	t.Run("deterministic classification", func(t *testing.T) {
		taxonomy := &TaxonomyContext{
			Collections: []TaxonomyCollection{
				{
					ID:          "col-1",
					Name:        "Collection 1",
					Description: "First collection",
				},
				{
					ID:          "col-2",
					Name:        "Collection 2",
					Description: "Second collection",
				},
			},
		}

		req := &ClassificationRequest{
			DocumentTitle:   "Test Document",
			DocumentContent: "Test content",
			Taxonomy:        taxonomy,
		}

		resp, err := mock.ClassifyDocument(ctx, req)
		if err != nil {
			t.Fatalf("ClassifyDocument failed: %v", err)
		}

		// Should pick first collection
		if resp.CollectionID != "col-1" {
			t.Errorf("Expected col-1, got %s", resp.CollectionID)
		}
	})

	// Test: Deterministic question answering
	t.Run("deterministic question answering", func(t *testing.T) {
		req := &QuestionRequest{
			Question: "What is the answer?",
			ContextDocs: []ContextDocument{
				{Title: "Doc 1", Excerpt: "Content 1", URL: "url1"},
				{Title: "Doc 2", Excerpt: "Content 2", URL: "url2"},
			},
		}

		resp, err := mock.AnswerQuestion(ctx, req)
		if err != nil {
			t.Fatalf("AnswerQuestion failed: %v", err)
		}

		// Should include citations from all context docs
		if len(resp.Citations) != 2 {
			t.Errorf("Expected 2 citations, got %d", len(resp.Citations))
		}

		if resp.Citations[0].DocumentTitle != "Doc 1" {
			t.Errorf("Expected citation for Doc 1, got %s", resp.Citations[0].DocumentTitle)
		}
	})

	// Test: Deterministic related documents
	t.Run("deterministic related documents", func(t *testing.T) {
		req := &RelatedDocsRequest{
			DocumentTitle:   "Main Doc",
			DocumentContent: "Content",
			AvailableDocs:   []string{"Doc A", "Doc B", "Doc C", "Doc D"},
		}

		resp, err := mock.FindRelatedDocuments(ctx, req)
		if err != nil {
			t.Fatalf("FindRelatedDocuments failed: %v", err)
		}

		// Should limit to 3 docs with decreasing relevance
		if len(resp.RelatedDocuments) != 3 {
			t.Errorf("Expected 3 related docs, got %d", len(resp.RelatedDocuments))
		}

		// Check relevance scores decrease
		for i := 1; i < len(resp.RelatedDocuments); i++ {
			if resp.RelatedDocuments[i].Relevance >= resp.RelatedDocuments[i-1].Relevance {
				t.Error("Expected decreasing relevance scores")
			}
		}
	})
}

// Example test showing call tracking
func TestAIMock_CallTracking(t *testing.T) {
	mock := NewAIMock()
	defer mock.Reset()

	ctx := context.Background()

	// Make several calls
	req := &ClassificationRequest{
		DocumentTitle:   "Test",
		DocumentContent: "Content",
		Taxonomy:        &TaxonomyContext{Collections: []TaxonomyCollection{}},
	}

	_, _ = mock.ClassifyDocument(ctx, req)
	_, _ = mock.ClassifyDocument(ctx, req)

	questionReq := &QuestionRequest{
		Question:    "Test?",
		ContextDocs: []ContextDocument{},
	}
	_, _ = mock.AnswerQuestion(ctx, questionReq)

	// Verify call counts
	classifyCount := mock.GetCallCount("ClassifyDocument")
	if classifyCount != 2 {
		t.Errorf("Expected 2 ClassifyDocument calls, got %d", classifyCount)
	}

	questionCount := mock.GetCallCount("AnswerQuestion")
	if questionCount != 1 {
		t.Errorf("Expected 1 AnswerQuestion call, got %d", questionCount)
	}

	// Verify last call
	lastCall := mock.GetLastCall("ClassifyDocument")
	if lastCall == nil {
		t.Error("Expected last call to be recorded")
	}

	lastReq, ok := lastCall.(*ClassificationRequest)
	if !ok {
		t.Error("Expected last call to be ClassificationRequest")
	}

	if lastReq.DocumentTitle != "Test" {
		t.Errorf("Expected document title 'Test', got '%s'", lastReq.DocumentTitle)
	}
}

// Example test showing integration scenario
func TestAIMock_IntegrationScenario(t *testing.T) {
	mock := NewAIMock()
	defer mock.Reset()

	ctx := context.Background()

	// Scenario: Document classification and enhancement workflow
	t.Run("document processing workflow", func(t *testing.T) {
		// 1. Classify the document
		taxonomy := &TaxonomyContext{
			Collections: []TaxonomyCollection{
				{
					ID:              "engineering",
					Name:            "Engineering",
					Description:     "Technical documentation",
					SampleDocuments: []string{"API Guide", "Architecture Docs"},
				},
			},
		}

		classifyReq := &ClassificationRequest{
			DocumentTitle:   "REST API Design",
			DocumentContent: "This document covers REST API best practices...",
			UserGuidance:    "This is a technical document",
			Taxonomy:        taxonomy,
		}

		// Configure realistic classification response
		mock.SetClassificationResponse(&ClassificationResponse{
			CollectionID: "engineering",
			Confidence:   0.92,
			Reasoning:    "Technical content about APIs matches engineering collection",
			SearchTerms:  []string{"REST", "API", "design", "best practices"},
		})

		classifyResp, err := mock.ClassifyDocument(ctx, classifyReq)
		if err != nil {
			t.Fatalf("Classification failed: %v", err)
		}

		if classifyResp.CollectionID != "engineering" {
			t.Errorf("Expected engineering collection, got %s", classifyResp.CollectionID)
		}

		// 2. Enhance the title
		titleReq := &TitleRequest{
			CurrentTitle:    "doc1",
			DocumentContent: classifyReq.DocumentContent,
		}

		mock.SetTitleResponse(&TitleResponse{
			SuggestedTitle: "REST API Design Best Practices",
			Confidence:     0.88,
		})

		titleResp, err := mock.EnhanceTitle(ctx, titleReq)
		if err != nil {
			t.Fatalf("Title enhancement failed: %v", err)
		}

		if titleResp.SuggestedTitle == "" {
			t.Error("Expected enhanced title")
		}

		// 3. Generate summary
		summaryReq := &SummaryRequest{
			DocumentTitle:   titleResp.SuggestedTitle,
			DocumentContent: classifyReq.DocumentContent,
		}

		mock.SetSummaryResponse(&SummaryResponse{
			Summary: "A comprehensive guide covering REST API design patterns and best practices for building scalable web services.",
		})

		summaryResp, err := mock.GenerateSummary(ctx, summaryReq)
		if err != nil {
			t.Fatalf("Summary generation failed: %v", err)
		}

		if summaryResp.Summary == "" {
			t.Error("Expected summary")
		}

		// 4. Find related documents
		relatedReq := &RelatedDocsRequest{
			DocumentTitle:   titleResp.SuggestedTitle,
			DocumentContent: classifyReq.DocumentContent,
			AvailableDocs:   []string{"GraphQL API Guide", "Database Design", "Authentication Guide"},
		}

		mock.SetRelatedDocsResponse(&RelatedDocsResponse{
			RelatedDocuments: []RelatedDocument{
				{
					Title:     "GraphQL API Guide",
					Relevance: 0.85,
					Reason:    "Also covers API design",
				},
				{
					Title:     "Authentication Guide",
					Relevance: 0.72,
					Reason:    "Related to API security",
				},
			},
		})

		relatedResp, err := mock.FindRelatedDocuments(ctx, relatedReq)
		if err != nil {
			t.Fatalf("Find related documents failed: %v", err)
		}

		if len(relatedResp.RelatedDocuments) == 0 {
			t.Error("Expected related documents")
		}

		// Verify workflow completed successfully
		t.Logf("Workflow completed successfully:")
		t.Logf("  - Classified to: %s (confidence: %.2f)", classifyResp.CollectionID, classifyResp.Confidence)
		t.Logf("  - Enhanced title: %s", titleResp.SuggestedTitle)
		t.Logf("  - Generated summary: %s", summaryResp.Summary)
		t.Logf("  - Found %d related documents", len(relatedResp.RelatedDocuments))
	})
}
