package mocks

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Package-level errors matching ai package
var (
	ErrCircuitBreakerOpen = errors.New("ai: circuit breaker open")
	ErrInvalidResponse    = errors.New("ai: invalid response")
	ErrTimeout            = errors.New("ai: request timeout")
	ErrTokenLimitExceeded = errors.New("ai: token limit exceeded")
	ErrAIRateLimited      = errors.New("ai: rate limited by provider")
)

// AI Domain Models

// TaxonomyCollection represents a collection in taxonomy context
type TaxonomyCollection struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	SampleDocuments []string `json:"sample_documents,omitempty"`
}

// TaxonomyContext wraps taxonomy information
type TaxonomyContext struct {
	Collections []TaxonomyCollection `json:"collections"`
}

// ClassificationRequest is the request for document classification
type ClassificationRequest struct {
	DocumentTitle   string           `json:"document_title"`
	DocumentContent string           `json:"document_content"`
	UserGuidance    string           `json:"user_guidance,omitempty"`
	Taxonomy        *TaxonomyContext `json:"taxonomy"`
}

// ClassificationResponse is the response from classification
type ClassificationResponse struct {
	CollectionID string                      `json:"collection_id"`
	Confidence   float64                     `json:"confidence"`
	Reasoning    string                      `json:"reasoning"`
	Alternatives []AlternativeClassification `json:"alternatives,omitempty"`
	SearchTerms  []string                    `json:"search_terms"`
}

// AlternativeClassification represents an alternative classification
type AlternativeClassification struct {
	CollectionID string  `json:"collection_id"`
	Confidence   float64 `json:"confidence"`
	Reasoning    string  `json:"reasoning"`
}

// ContextDocument represents a document for context
type ContextDocument struct {
	Title   string `json:"title"`
	Excerpt string `json:"excerpt"`
	URL     string `json:"url"`
}

// QuestionRequest is the request for question answering
type QuestionRequest struct {
	Question    string            `json:"question"`
	ContextDocs []ContextDocument `json:"context_documents"`
}

// QuestionResponse is the response from question answering
type QuestionResponse struct {
	Answer     string         `json:"answer"`
	Citations  []CitationInfo `json:"citations"`
	Confidence float64        `json:"confidence"`
}

// CitationInfo represents a citation
type CitationInfo struct {
	DocumentTitle string `json:"document_title"`
	DocumentURL   string `json:"document_url"`
}

// SummaryRequest is the request for summary generation
type SummaryRequest struct {
	DocumentTitle   string `json:"document_title"`
	DocumentContent string `json:"document_content"`
}

// SummaryResponse is the response from summary generation
type SummaryResponse struct {
	Summary string `json:"summary"`
}

// TitleRequest is the request for title enhancement
type TitleRequest struct {
	CurrentTitle    string `json:"current_title"`
	DocumentContent string `json:"document_content"`
}

// TitleResponse is the response from title enhancement
type TitleResponse struct {
	SuggestedTitle string  `json:"suggested_title"`
	Confidence     float64 `json:"confidence"`
}

// SearchTermsRequest is the request for search terms generation
type SearchTermsRequest struct {
	DocumentTitle   string `json:"document_title"`
	DocumentContent string `json:"document_content"`
}

// SearchTermsResponse is the response from search terms generation
type SearchTermsResponse struct {
	SearchTerms []string `json:"search_terms"`
}

// RelatedDocsRequest is the request for finding related documents
type RelatedDocsRequest struct {
	DocumentTitle   string   `json:"document_title"`
	DocumentContent string   `json:"document_content"`
	AvailableDocs   []string `json:"available_documents"`
}

// RelatedDocsResponse is the response from finding related documents
type RelatedDocsResponse struct {
	RelatedDocuments []RelatedDocument `json:"related_documents"`
}

// RelatedDocument represents a related document
type RelatedDocument struct {
	Title     string  `json:"title"`
	Relevance float64 `json:"relevance"`
	Reason    string  `json:"reason"`
}

// AIMock is a mock implementation of the ai.Client interface
type AIMock struct {
	mu sync.RWMutex

	// Configured responses
	classificationResponse *ClassificationResponse
	questionResponse       *QuestionResponse
	summaryResponse        *SummaryResponse
	titleResponse          *TitleResponse
	searchTermsResponse    *SearchTermsResponse
	relatedDocsResponse    *RelatedDocsResponse

	// Error configuration
	circuitBreakerOpen bool
	tokenLimitExceeded bool
	rateLimited        bool
	timeoutError       bool
	specificErrors     map[string]error

	// Call tracking
	callCounts map[string]int
	lastCalls  map[string]any

	// Deterministic mode
	deterministicMode bool
}

// NewAIMock creates a new mock AI client with sensible defaults
func NewAIMock() *AIMock {
	return &AIMock{
		specificErrors: make(map[string]error),
		callCounts:     make(map[string]int),
		lastCalls:      make(map[string]any),

		// Default responses
		classificationResponse: &ClassificationResponse{
			CollectionID: "default-collection",
			Confidence:   0.85,
			Reasoning:    "Default mock classification",
			SearchTerms:  []string{"mock", "test", "default"},
		},
		questionResponse: &QuestionResponse{
			Answer:     "This is a mock answer based on the provided context.",
			Confidence: 0.9,
			Citations: []CitationInfo{
				{
					DocumentTitle: "Mock Document",
					DocumentURL:   "https://example.com/mock",
				},
			},
		},
		summaryResponse: &SummaryResponse{
			Summary: "This is a mock summary of the document content.",
		},
		titleResponse: &TitleResponse{
			SuggestedTitle: "Enhanced Mock Title",
			Confidence:     0.88,
		},
		searchTermsResponse: &SearchTermsResponse{
			SearchTerms: []string{"mock", "test", "document", "example"},
		},
		relatedDocsResponse: &RelatedDocsResponse{
			RelatedDocuments: []RelatedDocument{
				{
					Title:     "Related Mock Document",
					Relevance: 0.75,
					Reason:    "Similar topic coverage",
				},
			},
		},
	}
}

// Configuration Methods

// SetClassificationResponse sets a custom classification response
func (m *AIMock) SetClassificationResponse(resp *ClassificationResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.classificationResponse = resp
}

// SetQuestionResponse sets a custom question response
func (m *AIMock) SetQuestionResponse(resp *QuestionResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.questionResponse = resp
}

// SetSummaryResponse sets a custom summary response
func (m *AIMock) SetSummaryResponse(resp *SummaryResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.summaryResponse = resp
}

// SetTitleResponse sets a custom title response
func (m *AIMock) SetTitleResponse(resp *TitleResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.titleResponse = resp
}

// SetSearchTermsResponse sets a custom search terms response
func (m *AIMock) SetSearchTermsResponse(resp *SearchTermsResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.searchTermsResponse = resp
}

// SetRelatedDocsResponse sets a custom related docs response
func (m *AIMock) SetRelatedDocsResponse(resp *RelatedDocsResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.relatedDocsResponse = resp
}

// SetCircuitBreakerOpen simulates circuit breaker state
func (m *AIMock) SetCircuitBreakerOpen(open bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.circuitBreakerOpen = open
}

// SetTokenLimitExceeded simulates token limit errors
func (m *AIMock) SetTokenLimitExceeded(exceeded bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokenLimitExceeded = exceeded
}

// SetRateLimited simulates rate limiting
func (m *AIMock) SetRateLimited(limited bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rateLimited = limited
}

// SetTimeoutError simulates timeout errors
func (m *AIMock) SetTimeoutError(timeout bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.timeoutError = timeout
}

// SetMethodError sets a specific error for a method
func (m *AIMock) SetMethodError(method string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.specificErrors[method] = err
}

// SetDeterministicMode enables deterministic responses based on input
func (m *AIMock) SetDeterministicMode(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deterministicMode = enabled
}

// GetCallCount returns the number of times a method was called
func (m *AIMock) GetCallCount(method string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.callCounts[method]
}

// GetLastCall returns the last request for a method
func (m *AIMock) GetLastCall(method string) any {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastCalls[method]
}

// Reset clears all configuration and returns to defaults
func (m *AIMock) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.circuitBreakerOpen = false
	m.tokenLimitExceeded = false
	m.rateLimited = false
	m.timeoutError = false
	m.deterministicMode = false
	m.specificErrors = make(map[string]error)
	m.callCounts = make(map[string]int)
	m.lastCalls = make(map[string]any)

	// Reset to default responses
	m.classificationResponse = &ClassificationResponse{
		CollectionID: "default-collection",
		Confidence:   0.85,
		Reasoning:    "Default mock classification",
		SearchTerms:  []string{"mock", "test", "default"},
	}
	m.questionResponse = &QuestionResponse{
		Answer:     "This is a mock answer based on the provided context.",
		Confidence: 0.9,
		Citations: []CitationInfo{
			{
				DocumentTitle: "Mock Document",
				DocumentURL:   "https://example.com/mock",
			},
		},
	}
	m.summaryResponse = &SummaryResponse{
		Summary: "This is a mock summary of the document content.",
	}
	m.titleResponse = &TitleResponse{
		SuggestedTitle: "Enhanced Mock Title",
		Confidence:     0.88,
	}
	m.searchTermsResponse = &SearchTermsResponse{
		SearchTerms: []string{"mock", "test", "document", "example"},
	}
	m.relatedDocsResponse = &RelatedDocsResponse{
		RelatedDocuments: []RelatedDocument{
			{
				Title:     "Related Mock Document",
				Relevance: 0.75,
				Reason:    "Similar topic coverage",
			},
		},
	}
}

// Internal helpers

func (m *AIMock) recordCall(method string, request any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCounts[method]++
	m.lastCalls[method] = request
}

func (m *AIMock) checkError(method string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.circuitBreakerOpen {
		return ErrCircuitBreakerOpen
	}

	if m.tokenLimitExceeded {
		return ErrTokenLimitExceeded
	}

	if m.rateLimited {
		return ErrAIRateLimited
	}

	if m.timeoutError {
		return ErrTimeout
	}

	if err, ok := m.specificErrors[method]; ok {
		return err
	}

	return nil
}

// Interface Implementation

// ClassifyDocument classifies a document into a collection
func (m *AIMock) ClassifyDocument(ctx context.Context, req *ClassificationRequest) (*ClassificationResponse, error) {
	m.recordCall("ClassifyDocument", req)

	if err := m.checkError("ClassifyDocument"); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// In deterministic mode, generate response based on input
	if m.deterministicMode && req.Taxonomy != nil && len(req.Taxonomy.Collections) > 0 {
		// Pick first collection and generate deterministic response
		firstCol := req.Taxonomy.Collections[0]
		return &ClassificationResponse{
			CollectionID: firstCol.ID,
			Confidence:   0.85,
			Reasoning:    fmt.Sprintf("Document matches %s based on content", firstCol.Name),
			SearchTerms:  []string{"deterministic", "test"},
		}, nil
	}

	// Return configured response
	return m.classificationResponse, nil
}

// AnswerQuestion answers a question based on context documents
func (m *AIMock) AnswerQuestion(ctx context.Context, req *QuestionRequest) (*QuestionResponse, error) {
	m.recordCall("AnswerQuestion", req)

	if err := m.checkError("AnswerQuestion"); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// In deterministic mode, generate response based on input
	if m.deterministicMode {
		citations := make([]CitationInfo, 0, len(req.ContextDocs))
		for _, doc := range req.ContextDocs {
			citations = append(citations, CitationInfo{
				DocumentTitle: doc.Title,
				DocumentURL:   doc.URL,
			})
		}

		return &QuestionResponse{
			Answer:     fmt.Sprintf("Answer to: %s", req.Question),
			Confidence: 0.9,
			Citations:  citations,
		}, nil
	}

	// Return configured response
	return m.questionResponse, nil
}

// GenerateSummary generates a summary of document content
func (m *AIMock) GenerateSummary(ctx context.Context, req *SummaryRequest) (*SummaryResponse, error) {
	m.recordCall("GenerateSummary", req)

	if err := m.checkError("GenerateSummary"); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// In deterministic mode, generate response based on input
	if m.deterministicMode {
		return &SummaryResponse{
			Summary: fmt.Sprintf("Summary of '%s'", req.DocumentTitle),
		}, nil
	}

	// Return configured response
	return m.summaryResponse, nil
}

// EnhanceTitle suggests an improved title for a document
func (m *AIMock) EnhanceTitle(ctx context.Context, req *TitleRequest) (*TitleResponse, error) {
	m.recordCall("EnhanceTitle", req)

	if err := m.checkError("EnhanceTitle"); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// In deterministic mode, generate response based on input
	if m.deterministicMode {
		return &TitleResponse{
			SuggestedTitle: fmt.Sprintf("Enhanced: %s", req.CurrentTitle),
			Confidence:     0.85,
		}, nil
	}

	// Return configured response
	return m.titleResponse, nil
}

// GenerateSearchTerms generates search terms for a document
func (m *AIMock) GenerateSearchTerms(ctx context.Context, req *SearchTermsRequest) (*SearchTermsResponse, error) {
	m.recordCall("GenerateSearchTerms", req)

	if err := m.checkError("GenerateSearchTerms"); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// In deterministic mode, generate response based on input
	if m.deterministicMode {
		return &SearchTermsResponse{
			SearchTerms: []string{"term1", "term2", "term3"},
		}, nil
	}

	// Return configured response
	return m.searchTermsResponse, nil
}

// FindRelatedDocuments finds documents related to the given document
func (m *AIMock) FindRelatedDocuments(ctx context.Context, req *RelatedDocsRequest) (*RelatedDocsResponse, error) {
	m.recordCall("FindRelatedDocuments", req)

	if err := m.checkError("FindRelatedDocuments"); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// In deterministic mode, generate response based on input
	if m.deterministicMode && len(req.AvailableDocs) > 0 {
		related := make([]RelatedDocument, 0, len(req.AvailableDocs))
		for i, doc := range req.AvailableDocs {
			if i >= 3 { // Limit to 3 related docs
				break
			}
			related = append(related, RelatedDocument{
				Title:     doc,
				Relevance: 0.8 - float64(i)*0.1,
				Reason:    "Related content",
			})
		}

		return &RelatedDocsResponse{
			RelatedDocuments: related,
		}, nil
	}

	// Return configured response
	return m.relatedDocsResponse, nil
}

// Ping checks if the AI service is available
func (m *AIMock) Ping(ctx context.Context) error {
	m.recordCall("Ping", nil)
	return m.checkError("Ping")
}
