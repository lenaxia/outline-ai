package mocks

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Package-level errors matching outline package
var (
	ErrUnauthorized   = errors.New("outline: unauthorized")
	ErrNotFound       = errors.New("outline: not found")
	ErrRateLimited    = errors.New("outline: rate limited")
	ErrServerError    = errors.New("outline: server error")
	ErrInvalidRequest = errors.New("outline: invalid request")
)

// Collection represents an Outline collection
type Collection struct {
	ID          string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Document represents an Outline document
type Document struct {
	ID           string
	CollectionID string
	Title        string
	Text         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	PublishedAt  *time.Time
}

// CreateDocumentRequest is the request to create a document
type CreateDocumentRequest struct {
	CollectionID     string
	Title            string
	Text             string
	Publish          bool
	ParentDocumentID *string
}

// UpdateDocumentRequest is the request to update a document
type UpdateDocumentRequest struct {
	Title string
	Text  string
	Done  bool
}

// Comment represents an Outline comment
type Comment struct {
	ID         string
	DocumentID string
	Data       string
	CreatedAt  time.Time
}

// CommentContent represents comment content structure
type CommentContent struct {
	Type    string        `json:"type"`
	Content []ContentNode `json:"content,omitempty"`
}

// ContentNode represents a node in comment content
type ContentNode struct {
	Type    string        `json:"type"`
	Text    string        `json:"text,omitempty"`
	Content []ContentNode `json:"content,omitempty"`
}

// CreateCommentRequest is the request to create a comment
type CreateCommentRequest struct {
	DocumentID string
	Data       CommentContent
}

// SearchOptions contains options for document search
type SearchOptions struct {
	Limit        int
	Offset       int
	CollectionID string
}

// SearchResult contains search results
type SearchResult struct {
	Documents  []*Document
	TotalCount int
}

// OutlineMock is a mock implementation of the outline.Client interface
type OutlineMock struct {
	mu sync.RWMutex

	// In-memory storage
	collections map[string]*Collection
	documents   map[string]*Document
	comments    map[string][]*Comment

	// Configuration
	failureMode       bool
	rateLimited       bool
	specificErrors    map[string]error
	callCounts        map[string]int
	requestDelay      time.Duration

	// Counters for IDs
	docCounter     int
	commentCounter int
}

// NewOutlineMock creates a new mock Outline client
func NewOutlineMock() *OutlineMock {
	return &OutlineMock{
		collections:    make(map[string]*Collection),
		documents:      make(map[string]*Document),
		comments:       make(map[string][]*Comment),
		specificErrors: make(map[string]error),
		callCounts:     make(map[string]int),
	}
}

// Helper Methods for Test Setup

// AddCollection adds a collection to the mock storage
func (m *OutlineMock) AddCollection(id, name, description string) *Collection {
	m.mu.Lock()
	defer m.mu.Unlock()

	col := &Collection{
		ID:          id,
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	m.collections[id] = col
	return col
}

// AddDocument adds a document to the mock storage
func (m *OutlineMock) AddDocument(id, collectionID, title, text string) *Document {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	doc := &Document{
		ID:           id,
		CollectionID: collectionID,
		Title:        title,
		Text:         text,
		CreatedAt:    now,
		UpdatedAt:    now,
		PublishedAt:  &now,
	}
	m.documents[id] = doc
	return doc
}

// AddComment adds a comment to the mock storage
func (m *OutlineMock) AddComment(id, documentID, data string) *Comment {
	m.mu.Lock()
	defer m.mu.Unlock()

	comment := &Comment{
		ID:         id,
		DocumentID: documentID,
		Data:       data,
		CreatedAt:  time.Now(),
	}

	if m.comments[documentID] == nil {
		m.comments[documentID] = make([]*Comment, 0)
	}
	m.comments[documentID] = append(m.comments[documentID], comment)
	return comment
}

// Configuration Methods

// SetFailureMode configures all operations to fail
func (m *OutlineMock) SetFailureMode(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failureMode = enabled
}

// SetRateLimited simulates rate limiting
func (m *OutlineMock) SetRateLimited(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rateLimited = enabled
}

// SetGetDocumentError sets a specific error for GetDocument
func (m *OutlineMock) SetGetDocumentError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.specificErrors["GetDocument"] = err
}

// SetCreateCommentError sets a specific error for CreateComment
func (m *OutlineMock) SetCreateCommentError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.specificErrors["CreateComment"] = err
}

// SetRequestDelay adds artificial delay to all operations
func (m *OutlineMock) SetRequestDelay(delay time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requestDelay = delay
}

// GetCallCount returns the number of times a method was called
func (m *OutlineMock) GetCallCount(method string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.callCounts[method]
}

// Reset clears all data and configuration
func (m *OutlineMock) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.collections = make(map[string]*Collection)
	m.documents = make(map[string]*Document)
	m.comments = make(map[string][]*Comment)
	m.specificErrors = make(map[string]error)
	m.callCounts = make(map[string]int)
	m.failureMode = false
	m.rateLimited = false
	m.requestDelay = 0
	m.docCounter = 0
	m.commentCounter = 0
}

// Interface Implementation

func (m *OutlineMock) recordCall(method string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCounts[method]++
}

func (m *OutlineMock) checkError(method string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.requestDelay > 0 {
		time.Sleep(m.requestDelay)
	}

	if m.failureMode {
		return ErrServerError
	}

	if m.rateLimited {
		return ErrRateLimited
	}

	if err, ok := m.specificErrors[method]; ok {
		return err
	}

	return nil
}

// ListCollections returns all collections
func (m *OutlineMock) ListCollections(ctx context.Context) ([]*Collection, error) {
	m.recordCall("ListCollections")

	if err := m.checkError("ListCollections"); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	collections := make([]*Collection, 0, len(m.collections))
	for _, col := range m.collections {
		collections = append(collections, col)
	}

	return collections, nil
}

// GetCollection returns a collection by ID
func (m *OutlineMock) GetCollection(ctx context.Context, id string) (*Collection, error) {
	m.recordCall("GetCollection")

	if err := m.checkError("GetCollection"); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	col, ok := m.collections[id]
	if !ok {
		return nil, ErrNotFound
	}

	return col, nil
}

// GetDocument returns a document by ID
func (m *OutlineMock) GetDocument(ctx context.Context, id string) (*Document, error) {
	m.recordCall("GetDocument")

	if err := m.checkError("GetDocument"); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	doc, ok := m.documents[id]
	if !ok {
		return nil, ErrNotFound
	}

	return doc, nil
}

// ListDocuments returns all documents in a collection
func (m *OutlineMock) ListDocuments(ctx context.Context, collectionID string) ([]*Document, error) {
	m.recordCall("ListDocuments")

	if err := m.checkError("ListDocuments"); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if collection exists
	if _, ok := m.collections[collectionID]; !ok {
		return nil, ErrNotFound
	}

	documents := make([]*Document, 0)
	for _, doc := range m.documents {
		if doc.CollectionID == collectionID {
			documents = append(documents, doc)
		}
	}

	return documents, nil
}

// CreateDocument creates a new document
func (m *OutlineMock) CreateDocument(ctx context.Context, req *CreateDocumentRequest) (*Document, error) {
	m.recordCall("CreateDocument")

	if err := m.checkError("CreateDocument"); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if collection exists
	if _, ok := m.collections[req.CollectionID]; !ok {
		return nil, ErrNotFound
	}

	// Generate ID
	m.docCounter++
	id := fmt.Sprintf("doc-%d", m.docCounter)

	now := time.Now()
	doc := &Document{
		ID:           id,
		CollectionID: req.CollectionID,
		Title:        req.Title,
		Text:         req.Text,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if req.Publish {
		doc.PublishedAt = &now
	}

	m.documents[id] = doc
	return doc, nil
}

// UpdateDocument updates an existing document
func (m *OutlineMock) UpdateDocument(ctx context.Context, id string, req *UpdateDocumentRequest) (*Document, error) {
	m.recordCall("UpdateDocument")

	if err := m.checkError("UpdateDocument"); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	doc, ok := m.documents[id]
	if !ok {
		return nil, ErrNotFound
	}

	if req.Title != "" {
		doc.Title = req.Title
	}
	if req.Text != "" {
		doc.Text = req.Text
	}

	doc.UpdatedAt = time.Now()

	return doc, nil
}

// MoveDocument moves a document to a different collection
func (m *OutlineMock) MoveDocument(ctx context.Context, id string, collectionID string) error {
	m.recordCall("MoveDocument")

	if err := m.checkError("MoveDocument"); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	doc, ok := m.documents[id]
	if !ok {
		return ErrNotFound
	}

	if _, ok := m.collections[collectionID]; !ok {
		return ErrNotFound
	}

	doc.CollectionID = collectionID
	doc.UpdatedAt = time.Now()

	return nil
}

// SearchDocuments searches for documents
func (m *OutlineMock) SearchDocuments(ctx context.Context, query string, opts *SearchOptions) (*SearchResult, error) {
	m.recordCall("SearchDocuments")

	if err := m.checkError("SearchDocuments"); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Simple substring search
	var matches []*Document
	for _, doc := range m.documents {
		// Filter by collection if specified
		if opts != nil && opts.CollectionID != "" && doc.CollectionID != opts.CollectionID {
			continue
		}

		// Simple text matching
		if containsIgnoreCase(doc.Title, query) || containsIgnoreCase(doc.Text, query) {
			matches = append(matches, doc)
		}
	}

	// Apply pagination
	start := 0
	end := len(matches)

	if opts != nil {
		if opts.Offset > 0 {
			start = min(opts.Offset, len(matches))
		}
		if opts.Limit > 0 {
			end = min(start+opts.Limit, len(matches))
		}
	}

	result := &SearchResult{
		Documents:  matches[start:end],
		TotalCount: len(matches),
	}

	return result, nil
}

// CreateComment creates a comment on a document
func (m *OutlineMock) CreateComment(ctx context.Context, req *CreateCommentRequest) (*Comment, error) {
	m.recordCall("CreateComment")

	if err := m.checkError("CreateComment"); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if document exists
	if _, ok := m.documents[req.DocumentID]; !ok {
		return nil, ErrNotFound
	}

	// Generate ID
	m.commentCounter++
	id := fmt.Sprintf("comment-%d", m.commentCounter)

	// Convert CommentContent to string (simplified)
	data := extractTextFromCommentContent(req.Data)

	comment := &Comment{
		ID:         id,
		DocumentID: req.DocumentID,
		Data:       data,
		CreatedAt:  time.Now(),
	}

	if m.comments[req.DocumentID] == nil {
		m.comments[req.DocumentID] = make([]*Comment, 0)
	}
	m.comments[req.DocumentID] = append(m.comments[req.DocumentID], comment)

	return comment, nil
}

// ListComments returns all comments for a document
func (m *OutlineMock) ListComments(ctx context.Context, documentID string) ([]*Comment, error) {
	m.recordCall("ListComments")

	if err := m.checkError("ListComments"); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if document exists
	if _, ok := m.documents[documentID]; !ok {
		return nil, ErrNotFound
	}

	comments := m.comments[documentID]
	if comments == nil {
		comments = make([]*Comment, 0)
	}

	return comments, nil
}

// Ping checks if the service is available
func (m *OutlineMock) Ping(ctx context.Context) error {
	m.recordCall("Ping")
	return m.checkError("Ping")
}

// Helper functions

func containsIgnoreCase(s, substr string) bool {
	// Simple case-insensitive contains (not production-grade)
	if len(s) == 0 || len(substr) == 0 {
		return false
	}

	// Convert both to lowercase and check if substr is in s
	sLower := toLowerSimple(s)
	substrLower := toLowerSimple(substr)

	// Simple substring search
	for i := 0; i <= len(sLower)-len(substrLower); i++ {
		if sLower[i:i+len(substrLower)] == substrLower {
			return true
		}
	}
	return false
}

func toLowerSimple(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c = c + ('a' - 'A')
		}
		result[i] = c
	}
	return string(result)
}

func extractTextFromCommentContent(content CommentContent) string {
	// Simple text extraction from nested content structure
	var builder strings.Builder
	for _, node := range content.Content {
		if node.Text != "" {
			builder.WriteString(node.Text)
		}
		for _, child := range node.Content {
			if child.Text != "" {
				builder.WriteString(child.Text)
			}
		}
	}
	return builder.String()
}
