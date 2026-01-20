# Test Mocks

This directory contains production-ready mock implementations of the core interfaces used in the Outline AI Assistant.

## Available Mocks

### 1. Outline API Client Mock (`outline_mock.go`)

Mock implementation of the `outline.Client` interface for testing Outline API interactions.

**Features:**
- In-memory storage for documents, collections, and comments
- Configurable failure scenarios
- Rate limiting simulation
- Helper methods to seed test data
- Realistic error responses

**Usage Example:**

```go
import "github.com/yourusername/outline-ai/test/mocks"

func TestDocumentProcessing(t *testing.T) {
    // Create mock client
    mockClient := mocks.NewOutlineMock()

    // Seed test data
    collection := mockClient.AddCollection("col-123", "Engineering", "Engineering docs")
    mockClient.AddDocument("doc-456", collection.ID, "API Guide", "Content here...")

    // Use in your tests
    doc, err := mockClient.GetDocument(context.Background(), "doc-456")
    require.NoError(t, err)
    assert.Equal(t, "API Guide", doc.Title)

    // Simulate failures
    mockClient.SetFailureMode(true)
    _, err = mockClient.GetDocument(context.Background(), "doc-456")
    assert.Error(t, err)
}
```

### 2. AI Client Mock (`ai_mock.go`)

Mock implementation of the `ai.Client` interface for testing AI-powered features.

**Features:**
- Configurable responses for all operations
- Deterministic outputs for consistent testing
- Token limit simulation
- Circuit breaker simulation
- Pre-configured response scenarios

**Usage Example:**

```go
import "github.com/yourusername/outline-ai/test/mocks"

func TestDocumentClassification(t *testing.T) {
    // Create mock client with default responses
    mockAI := mocks.NewAIMock()

    // Configure classification response
    mockAI.SetClassificationResponse(&ai.ClassificationResponse{
        CollectionID: "engineering-docs",
        Confidence:   0.95,
        Reasoning:    "Document discusses API design",
        SearchTerms:  []string{"api", "rest", "design"},
    })

    // Use in your tests
    resp, err := mockAI.ClassifyDocument(context.Background(), &ai.ClassificationRequest{
        DocumentTitle:   "API Design Guide",
        DocumentContent: "Best practices for REST APIs",
    })
    require.NoError(t, err)
    assert.Equal(t, 0.95, resp.Confidence)

    // Simulate circuit breaker
    mockAI.SetCircuitBreakerOpen(true)
    _, err = mockAI.ClassifyDocument(context.Background(), req)
    assert.ErrorIs(t, err, ai.ErrCircuitBreakerOpen)
}
```

### 3. Storage Mock (`storage_mock.go`)

Mock implementation of the `persistence.Storage` interface for testing state persistence.

**Features:**
- In-memory question state tracking
- Full transaction support
- No actual database required
- Helper methods to seed test data
- Realistic error scenarios

**Usage Example:**

```go
import "github.com/yourusername/outline-ai/test/mocks"

func TestQuestionDeduplication(t *testing.T) {
    // Create mock storage
    mockStorage := mocks.NewStorageMock()

    // Test question tracking
    state := &persistence.QuestionState{
        QuestionHash:    "abc123",
        DocumentID:      "doc-456",
        QuestionText:    "What is REST?",
        AnswerDelivered: true,
        CommentID:       ptr("comment-789"),
    }

    err := mockStorage.MarkQuestionAnswered(context.Background(), state)
    require.NoError(t, err)

    // Verify deduplication
    answered, err := mockStorage.HasAnsweredQuestion(context.Background(), "abc123")
    require.NoError(t, err)
    assert.True(t, answered)
}
```

## Configuration Options

### Outline Mock

```go
mock := mocks.NewOutlineMock()

// Enable failure mode for all operations
mock.SetFailureMode(true)

// Enable rate limiting simulation
mock.SetRateLimited(true)

// Configure specific error for an operation
mock.SetGetDocumentError(errors.New("custom error"))

// Reset to clean state
mock.Reset()
```

### AI Mock

```go
mock := mocks.NewAIMock()

// Configure circuit breaker state
mock.SetCircuitBreakerOpen(true)

// Set token limit error
mock.SetTokenLimitExceeded(true)

// Configure custom responses
mock.SetQuestionResponse(&ai.QuestionResponse{
    Answer:     "REST is...",
    Confidence: 0.9,
    Citations:  []ai.CitationInfo{...},
})

// Reset to defaults
mock.Reset()
```

### Storage Mock

```go
mock := mocks.NewStorageMock()

// Seed test data
mock.SeedQuestionState(&persistence.QuestionState{...})

// Configure failure scenarios
mock.SetFailureMode(true)

// Clear all data
mock.Clear()
```

## Best Practices

1. **Reset Between Tests**: Always call `Reset()` or `Clear()` at the start of each test to ensure isolation.

2. **Use Table-Driven Tests**: Leverage the configurable nature of mocks for comprehensive test coverage.

3. **Test Error Paths**: Use failure modes to test error handling in your code.

4. **Verify Call Counts**: Some mocks track call counts - use these to verify behavior.

5. **Seed Realistic Data**: Use helper methods to create realistic test data that matches production scenarios.

## Thread Safety

All mocks use proper synchronization (mutex locks) and are safe to use concurrently in tests that use parallel execution (`t.Parallel()`).

## Integration with Testing Frameworks

These mocks work with:
- Standard Go `testing` package
- `testify/assert` and `testify/require`
- `testify/suite` for test suites
- Any testing framework that accepts standard Go interfaces

## Contributing

When adding new methods to the interfaces, update the corresponding mock implementations:

1. Add the method to the mock struct
2. Implement default behavior
3. Add configuration methods for custom responses
4. Update the example tests
5. Document usage in this README

## See Also

- [LLD-04: Outline API Client](../../docs/01_ARCHITECTURE/lld/04_outline_api_client.md)
- [LLD-05: AI Client](../../docs/01_ARCHITECTURE/lld/05_ai_client.md)
- [LLD-02: Persistence Layer](../../docs/01_ARCHITECTURE/lld/02_persistence_layer.md)
