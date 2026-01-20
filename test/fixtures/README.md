# Test Fixtures

This directory contains comprehensive test data fixtures for the Outline AI Assistant project.

## Directory Structure

```
test/fixtures/
├── README.md                           # This file
├── documents/                          # Sample documents
│   ├── technical_doc.json             # Technical/engineering document
│   ├── marketing_doc.json             # Marketing content document
│   ├── ambiguous_doc.json             # Document with unclear classification
│   ├── with_commands.json             # Document containing various commands
│   └── with_existing_summary.json     # Document with AI-generated summary
├── collections/                        # Collection definitions
│   └── sample_collections.json        # Array of example collections
└── ai_responses/                       # AI API responses
    ├── filing_high_confidence.json    # High confidence classification
    ├── filing_low_confidence.json     # Low confidence with alternatives
    ├── qna_answer.json                # Q&A response with citations
    └── summary.json                   # Summary generation response
```

## Fixture Descriptions

### Documents

All document fixtures follow the Outline API response format with these fields:
- `id`: Unique document identifier
- `collectionId`: Parent collection ID
- `title`: Document title
- `text`: Full markdown content
- `createdAt`: ISO 8601 timestamp
- `updatedAt`: ISO 8601 timestamp
- `publishedAt`: ISO 8601 timestamp (optional)

#### technical_doc.json
A typical engineering document about API design and authentication. Contains technical terminology, code examples, and architectural details. Clearly belongs in an Engineering collection.

#### marketing_doc.json
Marketing-focused content about product launches and go-to-market strategy. Contains marketing terminology, customer messaging, and campaign details. Clearly belongs in a Marketing collection.

#### ambiguous_doc.json
A document about "Mobile App API Documentation" that could reasonably fit in either Engineering (API implementation) or Product (mobile app features). Used for testing low-confidence classification scenarios.

#### with_commands.json
Contains multiple command markers for testing command detection:
- `/ai-file engineering related` - Filing command with guidance
- `/ai What is our API rate limit?` - Question command
- `/summarize` - Summary generation command

#### with_existing_summary.json
A document that already has an AI-generated summary with HTML comment markers. Used for testing idempotent summary updates and marker detection.

### Collections

#### sample_collections.json
Array of realistic collections representing different departments:
- Engineering: Technical documentation and code
- Product: Product specs and roadmaps
- Marketing: Marketing materials and campaigns
- Customer Success: Support docs and onboarding
- Sales: Sales playbooks and proposals
- Operations: Internal processes and policies

Each collection includes:
- `id`: Unique identifier
- `name`: Human-readable name
- `description`: Purpose and content type
- `createdAt`: ISO 8601 timestamp
- `updatedAt`: ISO 8601 timestamp

### AI Responses

All AI response fixtures follow the expected response format from the AI client.

#### filing_high_confidence.json
Classification response with 0.95 confidence score. Includes:
- `collection_id`: Target collection
- `confidence`: 0.95 (above threshold)
- `reasoning`: Explanation of classification
- `search_terms`: Keywords for search enhancement
- `alternatives`: Empty (high confidence)

#### filing_low_confidence.json
Classification response with 0.55 confidence score (below 0.7 threshold). Includes:
- `collection_id`: Primary suggestion
- `confidence`: 0.55 (below threshold)
- `reasoning`: Explanation of uncertainty
- `search_terms`: Keywords for search enhancement
- `alternatives`: Array of alternative collections with their confidence scores

#### qna_answer.json
Question answering response with citations. Includes:
- `answer`: Formatted answer text with markdown
- `confidence`: Confidence in answer accuracy
- `sources`: Array of source documents with titles and URLs
- `context_used`: Number of context documents used

#### summary.json
Summary generation response. Includes:
- `summary`: 2-3 sentence summary text
- `key_topics`: Array of main topics covered
- `confidence`: Confidence in summary quality

## Usage in Tests

### Loading Fixtures

```go
import (
    "encoding/json"
    "os"
    "testing"
)

func loadDocumentFixture(t *testing.T, filename string) *outline.Document {
    data, err := os.ReadFile("test/fixtures/documents/" + filename)
    if err != nil {
        t.Fatalf("Failed to load fixture: %v", err)
    }

    var doc outline.Document
    if err := json.Unmarshal(data, &doc); err != nil {
        t.Fatalf("Failed to parse fixture: %v", err)
    }

    return &doc
}

func TestCommandDetection(t *testing.T) {
    doc := loadDocumentFixture(t, "with_commands.json")

    detector := command.NewRegexDetector()
    commands, err := detector.DetectCommands(context.Background(), doc)

    assert.NoError(t, err)
    assert.Len(t, commands, 3)
}
```

### Testing Command Detection

Use `with_commands.json` to test:
- Command pattern matching
- Argument extraction
- Multiple commands in one document
- Line range calculation

### Testing Classification

Use these documents for classification tests:
- `technical_doc.json` - Should classify with high confidence to Engineering
- `marketing_doc.json` - Should classify with high confidence to Marketing
- `ambiguous_doc.json` - Should classify with low confidence, triggering guidance loop

### Testing Idempotency

Use `with_existing_summary.json` to test:
- Detection of existing AI-generated content
- Marker recognition (HTML comments)
- Clean replacement of existing summaries
- Handling of user-edited content between markers

### Testing AI Integration

Mock AI client responses using fixtures from `ai_responses/`:
```go
func setupMockAIClient(t *testing.T) *ai.MockClient {
    highConfResp := loadAIResponseFixture(t, "filing_high_confidence.json")

    mock := &ai.MockClient{}
    mock.On("ClassifyDocument", mock.Anything, mock.Anything).
        Return(highConfResp, nil)

    return mock
}
```

## Customizing Fixtures

To create new fixtures:

1. **Documents**: Follow the Outline API Document model structure
2. **Collections**: Follow the Collection model structure
3. **AI Responses**: Match the expected AI client response format
4. **Timestamps**: Use ISO 8601 format (`2024-01-15T10:30:00Z`)
5. **IDs**: Use descriptive IDs like `doc-tech-001` for readability

## Fixture Maintenance

When updating fixtures:
- Keep them realistic and representative of actual usage
- Update this README if adding new fixture types
- Ensure all required fields are present
- Use consistent formatting (2-space indentation)
- Include comments in JSON where helpful (remove before parsing)

## Related Documentation

- [High-Level Design](/docs/01_ARCHITECTURE/2026-01-19_01_HLD.md)
- [Command System LLD](/docs/01_ARCHITECTURE/lld/09_command_system.md)
- [Outline API Client LLD](/docs/01_ARCHITECTURE/lld/04_outline_api_client.md)
