# Low-Level Design: AI Prompt Templates

**Domain:** AI Integration
**Status:** Design
**Last Updated:** 2026-01-19
**Target Deployment:** Homelab/SOHO

## Purpose

Define production-ready, versioned AI prompt templates for all document operations including classification, Q&A, content enhancement, and related document discovery. Each template includes complete prompt text, Go implementation code, example inputs/outputs, and token management strategies.

## Design Principles

1. **Structured Responses**: All prompts return JSON with strict schemas
2. **Versioned Templates**: Support prompt evolution without breaking changes
3. **Token Awareness**: Explicit token limits and truncation strategies
4. **Clear Instructions**: Unambiguous formatting and behavioral guidelines
5. **Confidence Scoring**: All responses include confidence levels
6. **Production-Ready**: Real-world examples with edge cases
7. **Provider Agnostic**: Works with OpenAI, Claude, local models

## Prompt Versioning Strategy

### Version Format

```
v{major}.{minor}
- Major: Breaking changes to response schema or behavior
- Minor: Prompt improvements, clarifications, new optional fields
```

### Version Management

```go
package prompts

type PromptVersion string

const (
    // Classification prompts
    ClassifyDocumentV1 PromptVersion = "classify-v1.0"
    ClassifyDocumentV2 PromptVersion = "classify-v2.0" // Future: with embeddings

    // Q&A prompts
    AnswerQuestionV1 PromptVersion = "answer-v1.0"

    // Content enhancement prompts
    GenerateSummaryV1    PromptVersion = "summary-v1.0"
    EnhanceTitleV1       PromptVersion = "title-v1.0"
    GenerateSearchTermsV1 PromptVersion = "searchterms-v1.0"
    RelatedDocumentsV1   PromptVersion = "related-v1.0"
)

type PromptTemplate struct {
    Version     PromptVersion
    SystemPrompt string
    UserPromptBuilder func(interface{}) string
    MaxTokens    int
    Temperature  float32
}

var PromptRegistry = map[PromptVersion]PromptTemplate{
    ClassifyDocumentV1: {
        Version:     ClassifyDocumentV1,
        SystemPrompt: classifySystemPromptV1,
        UserPromptBuilder: buildClassifyUserPromptV1,
        MaxTokens:    1000,
        Temperature:  0.3,
    },
    // ... more templates
}
```

### Using Versioned Prompts

```go
package prompts

func GetPrompt(version PromptVersion) (PromptTemplate, error) {
    template, exists := PromptRegistry[version]
    if !exists {
        return PromptTemplate{}, fmt.Errorf("prompt version %s not found", version)
    }
    return template, nil
}

// Usage in AI client
func (c *OpenAIClient) ClassifyDocument(ctx context.Context, req *ClassificationRequest) (*ClassificationResponse, error) {
    template, err := prompts.GetPrompt(prompts.ClassifyDocumentV1)
    if err != nil {
        return nil, err
    }

    systemPrompt := template.SystemPrompt
    userPrompt := template.UserPromptBuilder(req)

    responseText, err := c.makeCompletionRequest(ctx, systemPrompt, userPrompt, template.MaxTokens, template.Temperature)
    // ... rest of implementation
}
```

## Token Limit Management

### Token Counting Strategy

```go
package prompts

import (
    "github.com/tiktoken-go/tokenizer"
)

type TokenCounter struct {
    encoder *tokenizer.Encoder
}

func NewTokenCounter(model string) (*TokenCounter, error) {
    encoder, err := tokenizer.Get(model)
    if err != nil {
        return nil, err
    }
    return &TokenCounter{encoder: encoder}, nil
}

func (tc *TokenCounter) CountTokens(text string) int {
    tokens, _ := tc.encoder.Encode(text)
    return len(tokens)
}

func (tc *TokenCounter) EstimateTokens(text string) int {
    // Rough estimate: 1 token ≈ 4 characters
    return len(text) / 4
}
```

### Content Truncation

```go
package prompts

type TruncationStrategy string

const (
    TruncateHead   TruncationStrategy = "head"   // Keep beginning
    TruncateTail   TruncationStrategy = "tail"   // Keep end
    TruncateMiddle TruncationStrategy = "middle" // Keep start + end
)

type TruncationConfig struct {
    MaxTokens int
    Strategy  TruncationStrategy
    Marker    string // e.g., "... (content truncated) ..."
}

func TruncateContent(content string, cfg TruncationConfig, counter *TokenCounter) string {
    tokens := counter.CountTokens(content)
    if tokens <= cfg.MaxTokens {
        return content
    }

    switch cfg.Strategy {
    case TruncateHead:
        return truncateHead(content, cfg.MaxTokens, cfg.Marker, counter)
    case TruncateTail:
        return truncateTail(content, cfg.MaxTokens, cfg.Marker, counter)
    case TruncateMiddle:
        return truncateMiddle(content, cfg.MaxTokens, cfg.Marker, counter)
    default:
        return truncateTail(content, cfg.MaxTokens, cfg.Marker, counter)
    }
}

func truncateTail(content string, maxTokens int, marker string, counter *TokenCounter) string {
    // Binary search for optimal character length
    left, right := 0, len(content)
    for left < right {
        mid := (left + right + 1) / 2
        truncated := content[:mid] + marker
        if counter.CountTokens(truncated) <= maxTokens {
            left = mid
        } else {
            right = mid - 1
        }
    }
    return content[:left] + marker
}

// Similar implementations for truncateHead and truncateMiddle
```

## Prompt Template 1: Document Classification

### Purpose

Classify documents into collections with confidence scoring, alternatives for low confidence, and extracted search terms.

### Response Schema

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["collection_id", "confidence", "reasoning", "search_terms"],
  "properties": {
    "collection_id": {
      "type": "string",
      "description": "ID of the best matching collection"
    },
    "confidence": {
      "type": "number",
      "minimum": 0.0,
      "maximum": 1.0,
      "description": "Confidence score (0.0-1.0)"
    },
    "reasoning": {
      "type": "string",
      "description": "Brief explanation (1-2 sentences) of classification decision"
    },
    "alternatives": {
      "type": "array",
      "description": "Alternative collections (only if confidence < 0.9)",
      "items": {
        "type": "object",
        "required": ["collection_id", "confidence", "reasoning"],
        "properties": {
          "collection_id": {"type": "string"},
          "confidence": {"type": "number"},
          "reasoning": {"type": "string"}
        }
      },
      "maxItems": 3
    },
    "search_terms": {
      "type": "array",
      "description": "5-10 relevant search terms",
      "items": {"type": "string"},
      "minItems": 5,
      "maxItems": 10
    }
  }
}
```

### System Prompt

```go
const classifySystemPromptV1 = `You are a document classification assistant for a knowledge management system. Your task is to analyze documents and determine which collection they belong to based on their content, title, and optional user guidance.

RESPONSE FORMAT:
You MUST respond with a valid JSON object matching this exact structure:
{
  "collection_id": "the ID of the best matching collection",
  "confidence": 0.85,
  "reasoning": "brief explanation of why this collection fits (1-2 sentences)",
  "alternatives": [
    {
      "collection_id": "alternative_collection_id",
      "confidence": 0.65,
      "reasoning": "why this is also potentially relevant"
    }
  ],
  "search_terms": ["term1", "term2", "term3", "term4", "term5"]
}

CLASSIFICATION GUIDELINES:
1. Analyze the document title, content, and collection descriptions
2. If user guidance is provided, it should HEAVILY influence your decision
3. Return confidence between 0.0 (completely uncertain) and 1.0 (completely certain)
4. Include up to 3 alternatives ONLY if your primary confidence is below 0.9
5. Generate 5-10 relevant search terms that capture key topics and concepts
6. Be concise but clear in your reasoning

CONFIDENCE SCORING:
- 0.9-1.0: Very clear match, obvious collection
- 0.7-0.9: Good match, some minor ambiguity
- 0.5-0.7: Uncertain, multiple collections could fit
- Below 0.5: Highly ambiguous, requires user guidance

USER GUIDANCE:
- User guidance is natural language input like "this is about pricing" or "file with engineering docs"
- Treat user guidance as the PRIMARY signal, overriding content-based classification if they conflict
- If guidance is vague or unhelpful, rely more on content analysis

IMPORTANT:
- You must choose from the provided collection IDs only
- Do not make up collection IDs
- Do not include explanations outside the JSON structure
- Ensure valid JSON syntax (proper quotes, commas, brackets)`
```

### User Prompt Builder

```go
func buildClassifyUserPromptV1(req interface{}) string {
    classReq := req.(*ai.ClassificationRequest)
    var sb strings.Builder

    sb.WriteString("DOCUMENT TO CLASSIFY:\n")
    sb.WriteString(fmt.Sprintf("Title: %s\n\n", classReq.DocumentTitle))

    // Truncate content if needed
    content := classReq.DocumentContent
    if len(content) > 8000 { // ~2000 tokens
        content = truncateTail(content, 2000, "\n\n... (content truncated for length) ...\n\n", tokenCounter)
    }
    sb.WriteString(fmt.Sprintf("Content:\n%s\n\n", content))

    if classReq.UserGuidance != "" {
        sb.WriteString("USER GUIDANCE (IMPORTANT - prioritize this):\n")
        sb.WriteString(fmt.Sprintf("%s\n\n", classReq.UserGuidance))
    }

    sb.WriteString("AVAILABLE COLLECTIONS:\n")
    for i, col := range classReq.Taxonomy.Collections {
        sb.WriteString(fmt.Sprintf("\n[%d] ID: %s\n", i+1, col.ID))
        sb.WriteString(fmt.Sprintf("    Name: %s\n", col.Name))
        sb.WriteString(fmt.Sprintf("    Description: %s\n", col.Description))

        if len(col.SampleDocuments) > 0 {
            sb.WriteString(fmt.Sprintf("    Sample documents: %s\n",
                strings.Join(col.SampleDocuments, ", ")))
        }
    }

    sb.WriteString("\nProvide your classification in JSON format as specified.")

    return sb.String()
}
```

### Example Usage

```go
package main

import (
    "context"
    "fmt"
    "github.com/yourusername/outline-ai/internal/ai"
)

func ExampleClassifyDocument() {
    client := ai.NewOpenAIClient(endpoint, apiKey, "gpt-4", 30*time.Second)

    req := &ai.ClassificationRequest{
        DocumentTitle:   "Q4 Sales Performance Analysis",
        DocumentContent: "This quarter saw a 23% increase in revenue compared to Q3. Key drivers include: new enterprise customers (12), expansion in EMEA market (+45%), and improved conversion rates on our freemium tier (8.2% → 11.3%). Challenges: Customer acquisition cost increased by 15%, primarily due to increased ad spend. Churn rate remained stable at 2.1%.",
        UserGuidance:    "",
        Taxonomy: &ai.TaxonomyContext{
            Collections: []ai.TaxonomyCollection{
                {
                    ID:          "col_finance_001",
                    Name:        "Finance & Revenue",
                    Description: "Financial reports, revenue analysis, budget planning",
                    SampleDocuments: []string{"Q3 Budget Review", "Annual Financial Summary"},
                },
                {
                    ID:          "col_sales_001",
                    Name:        "Sales & Marketing",
                    Description: "Sales performance, marketing campaigns, customer acquisition",
                    SampleDocuments: []string{"Sales Kickoff 2025", "Marketing Campaign Results"},
                },
                {
                    ID:          "col_product_001",
                    Name:        "Product & Engineering",
                    Description: "Product roadmaps, technical documentation, feature specs",
                    SampleDocuments: []string{"API Documentation", "Product Roadmap Q1"},
                },
            },
        },
    }

    resp, err := client.ClassifyDocument(context.Background(), req)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Classification: %s (confidence: %.2f)\n", resp.CollectionID, resp.Confidence)
    fmt.Printf("Reasoning: %s\n", resp.Reasoning)
}
```

### Example Response: High Confidence

```json
{
  "collection_id": "col_sales_001",
  "confidence": 0.92,
  "reasoning": "The document focuses on sales performance metrics including revenue growth, customer acquisition, and conversion rates, which clearly aligns with the Sales & Marketing collection.",
  "alternatives": [
    {
      "collection_id": "col_finance_001",
      "confidence": 0.75,
      "reasoning": "Contains revenue analysis and financial metrics, but primary focus is on sales performance rather than financial planning."
    }
  ],
  "search_terms": ["Q4", "sales performance", "revenue growth", "customer acquisition", "conversion rate", "EMEA market", "churn rate", "enterprise customers"]
}
```

### Example Response: Low Confidence

```json
{
  "collection_id": "col_product_001",
  "confidence": 0.58,
  "reasoning": "Document discusses technical implementation but could also fit in engineering workflows or API documentation collections.",
  "alternatives": [
    {
      "collection_id": "col_eng_workflows_001",
      "confidence": 0.55,
      "reasoning": "Contains deployment and CI/CD information which fits engineering workflow documentation."
    },
    {
      "collection_id": "col_api_docs_001",
      "confidence": 0.48,
      "reasoning": "Includes API endpoint documentation that could belong in API-specific collection."
    }
  ],
  "search_terms": ["API", "deployment", "CI/CD", "authentication", "endpoints", "technical implementation"]
}
```

### With User Guidance

```go
// Low confidence without guidance
req1 := &ai.ClassificationRequest{
    DocumentTitle:   "Meeting Notes",
    DocumentContent: "Discussed project timeline and deliverables...",
    UserGuidance:    "",
}
// Response: confidence 0.45, multiple alternatives

// High confidence with guidance
req2 := &ai.ClassificationRequest{
    DocumentTitle:   "Meeting Notes",
    DocumentContent: "Discussed project timeline and deliverables...",
    UserGuidance:    "this is about the product roadmap review",
}
// Response: confidence 0.88, clear collection match
```

## Prompt Template 2: Question Answering with Context

### Purpose

Answer questions based on provided document context, with citations and confidence scoring.

### Response Schema

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["answer", "citations", "confidence"],
  "properties": {
    "answer": {
      "type": "string",
      "description": "Markdown-formatted answer to the question"
    },
    "citations": {
      "type": "array",
      "description": "Source documents used to construct the answer",
      "items": {
        "type": "object",
        "required": ["document_title", "document_url"],
        "properties": {
          "document_title": {"type": "string"},
          "document_url": {"type": "string"}
        }
      }
    },
    "confidence": {
      "type": "number",
      "minimum": 0.0,
      "maximum": 1.0,
      "description": "Confidence in answer accuracy based on available context"
    }
  }
}
```

### System Prompt

```go
const answerQuestionSystemPromptV1 = `You are a helpful knowledge base assistant that answers questions based on provided context documents. Your goal is to provide accurate, well-sourced answers using ONLY the information available in the context.

RESPONSE FORMAT:
You MUST respond with a valid JSON object matching this exact structure:
{
  "answer": "Your detailed answer here, formatted in markdown",
  "citations": [
    {
      "document_title": "Title of source document",
      "document_url": "URL from the context"
    }
  ],
  "confidence": 0.9
}

ANSWERING GUIDELINES:
1. Base your answer ONLY on the provided context documents
2. Cite all sources by including them in the citations array
3. Format your answer in markdown (use headers, lists, bold, etc. for clarity)
4. If the context doesn't contain enough information, say so explicitly
5. Do not make assumptions or add information not present in the context
6. If multiple documents provide relevant information, synthesize them coherently

CITATION REQUIREMENTS:
- Include ALL documents that contributed to your answer
- Use the exact document_title and document_url from the context
- If you cannot answer from context, set confidence low and explain why

CONFIDENCE SCORING:
- 0.9-1.0: Complete answer with comprehensive context coverage
- 0.7-0.9: Good answer with some minor gaps in context
- 0.5-0.7: Partial answer, significant information missing
- Below 0.5: Cannot adequately answer from provided context

ANSWER FORMATTING:
- Use markdown headers (##, ###) for structure
- Use bullet points or numbered lists for clarity
- Use **bold** for emphasis on key points
- Use code blocks (\`\`\`) for technical content
- Keep answers concise but complete (2-4 paragraphs typical)

WHEN CONTEXT IS INSUFFICIENT:
If you cannot answer the question from context, respond like this:
{
  "answer": "I don't have enough information in the provided documents to answer this question. The context documents discuss X and Y, but don't cover Z which is needed to answer your question.",
  "citations": [],
  "confidence": 0.2
}

IMPORTANT:
- Do not fabricate information not in the context
- Do not cite documents that weren't actually used
- Ensure valid JSON syntax (proper quotes, commas, brackets)`
```

### User Prompt Builder

```go
func buildAnswerQuestionUserPromptV1(req interface{}) string {
    qnaReq := req.(*ai.QuestionRequest)
    var sb strings.Builder

    sb.WriteString(fmt.Sprintf("QUESTION: %s\n\n", qnaReq.Question))
    sb.WriteString("CONTEXT DOCUMENTS:\n\n")

    for i, doc := range qnaReq.ContextDocs {
        sb.WriteString(fmt.Sprintf("--- Document %d ---\n", i+1))
        sb.WriteString(fmt.Sprintf("Title: %s\n", doc.Title))
        sb.WriteString(fmt.Sprintf("URL: %s\n\n", doc.URL))

        // Truncate excerpt if needed
        excerpt := doc.Excerpt
        if len(excerpt) > 4000 { // ~1000 tokens per document
            excerpt = truncateTail(excerpt, 1000, "\n\n... (excerpt truncated) ...\n\n", tokenCounter)
        }
        sb.WriteString(fmt.Sprintf("Content:\n%s\n\n", excerpt))
    }

    sb.WriteString("Provide your answer in JSON format as specified, using only the context above.")

    return sb.String()
}
```

### Example Usage

```go
func ExampleAnswerQuestion() {
    client := ai.NewOpenAIClient(endpoint, apiKey, "gpt-4", 30*time.Second)

    req := &ai.QuestionRequest{
        Question: "What are our current API rate limits and how can customers request increases?",
        ContextDocs: []ai.ContextDocument{
            {
                Title:   "API Documentation - Rate Limits",
                URL:     "https://docs.example.com/api/rate-limits",
                Excerpt: "Our API implements rate limiting to ensure fair usage across all customers. Free tier: 100 requests/hour. Pro tier: 1,000 requests/hour. Enterprise tier: Custom limits negotiated with sales team. Rate limits reset at the top of each hour.",
            },
            {
                Title:   "API Rate Limit Increase Process",
                URL:     "https://docs.example.com/support/rate-limit-increase",
                Excerpt: "To request a rate limit increase: 1) Log into your dashboard, 2) Navigate to Settings > API, 3) Click 'Request Limit Increase', 4) Provide your use case and expected traffic. Our team typically responds within 2 business days. Enterprise customers should contact their account manager directly.",
            },
        },
    }

    resp, err := client.AnswerQuestion(context.Background(), req)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Answer: %s\n", resp.Answer)
    fmt.Printf("Confidence: %.2f\n", resp.Confidence)
    fmt.Printf("Citations: %d\n", len(resp.Citations))
}
```

### Example Response: High Confidence

```json
{
  "answer": "## API Rate Limits\n\nYour API rate limits depend on your subscription tier:\n\n- **Free tier**: 100 requests/hour\n- **Pro tier**: 1,000 requests/hour\n- **Enterprise tier**: Custom limits negotiated with sales team\n\nAll rate limits reset at the top of each hour.\n\n## Requesting a Rate Limit Increase\n\nTo request a rate limit increase:\n\n1. Log into your dashboard\n2. Navigate to Settings > API\n3. Click 'Request Limit Increase'\n4. Provide your use case and expected traffic\n\nThe team typically responds within 2 business days. Enterprise customers should contact their account manager directly for limit adjustments.",
  "citations": [
    {
      "document_title": "API Documentation - Rate Limits",
      "document_url": "https://docs.example.com/api/rate-limits"
    },
    {
      "document_title": "API Rate Limit Increase Process",
      "document_url": "https://docs.example.com/support/rate-limit-increase"
    }
  ],
  "confidence": 0.95
}
```

### Example Response: Insufficient Context

```json
{
  "answer": "I don't have enough information in the provided documents to answer your question about webhook retry policies. The context documents discuss rate limits and authentication, but don't contain information about webhook retry behavior or configuration.",
  "citations": [],
  "confidence": 0.15
}
```

## Prompt Template 3: Summary Generation

### Purpose

Generate concise 2-3 sentence summaries of documents for enhanced discoverability.

### Response Schema

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["summary"],
  "properties": {
    "summary": {
      "type": "string",
      "description": "2-3 sentence summary of the document"
    }
  }
}
```

### System Prompt

```go
const generateSummarySystemPromptV1 = `You are a document summarization assistant. Your task is to create concise, informative summaries that help users quickly understand document content.

RESPONSE FORMAT:
You MUST respond with a valid JSON object:
{
  "summary": "Your 2-3 sentence summary here."
}

SUMMARIZATION GUIDELINES:
1. Create a 2-3 sentence summary (50-100 words typical)
2. Capture the main topic, key points, and purpose of the document
3. Write in clear, professional language
4. Focus on what the document IS rather than what it CONTAINS
5. Make summaries useful for search and discovery

GOOD SUMMARY EXAMPLES:
- "This document outlines the Q4 product roadmap, focusing on three major features: advanced analytics, mobile app redesign, and API v3 launch. It includes timelines, resource allocation, and success metrics for each initiative."
- "Technical specification for implementing OAuth 2.0 authentication in our API. Covers authorization flows, token management, security considerations, and code examples for common scenarios."

BAD SUMMARY EXAMPLES:
- "This document contains information about sales." (too vague)
- "Here is a document that talks about the features we are planning to build and when we will build them and who will work on them." (too wordy, poor structure)
- "Sales increased. Marketing did well. More customers signed up." (too brief, lacks context)

IMPORTANT:
- Do not use phrases like "This document discusses..." or "The author talks about..."
- Get straight to the content
- Ensure valid JSON syntax`
```

### User Prompt Builder

```go
func buildSummaryUserPromptV1(req interface{}) string {
    summReq := req.(*ai.SummaryRequest)

    // Truncate content for summary (don't need full document)
    content := summReq.DocumentContent
    if len(content) > 6000 { // ~1500 tokens
        content = truncateTail(content, 1500, "\n\n... (content truncated) ...\n\n", tokenCounter)
    }

    return fmt.Sprintf("Document Title: %s\n\nDocument Content:\n%s\n\nProvide a 2-3 sentence summary in JSON format.",
        summReq.DocumentTitle, content)
}
```

### Example Usage

```go
func ExampleGenerateSummary() {
    client := ai.NewOpenAIClient(endpoint, apiKey, "gpt-4", 30*time.Second)

    req := &ai.SummaryRequest{
        DocumentTitle:   "Docker Deployment Guide",
        DocumentContent: "This guide covers deploying our application using Docker containers. It includes Dockerfile configuration, docker-compose setup for local development, environment variable management, and best practices for production deployments. We also cover common troubleshooting scenarios and performance optimization tips.",
    }

    resp, err := client.GenerateSummary(context.Background(), req)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Summary: %s\n", resp.Summary)
}
```

### Example Responses

```json
{
  "summary": "Comprehensive guide for deploying the application using Docker, covering Dockerfile configuration, docker-compose for local development, and production deployment best practices. Includes environment variable management, troubleshooting scenarios, and performance optimization techniques."
}
```

```json
{
  "summary": "Q4 OKR planning document that defines objectives and key results for the engineering team. Sets ambitious goals around feature delivery (5 major releases), performance improvements (50% latency reduction), and team growth (hire 8 engineers)."
}
```

## Prompt Template 4: Title Enhancement

### Purpose

Improve vague or generic document titles to make them more descriptive and discoverable.

### Response Schema

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["suggested_title", "confidence"],
  "properties": {
    "suggested_title": {
      "type": "string",
      "description": "Improved, descriptive title"
    },
    "confidence": {
      "type": "number",
      "minimum": 0.0,
      "maximum": 1.0,
      "description": "Confidence that the suggested title is better than the original"
    }
  }
}
```

### System Prompt

```go
const enhanceTitleSystemPromptV1 = `You are a title enhancement assistant. Your task is to improve vague or generic document titles to make them more descriptive, specific, and discoverable.

RESPONSE FORMAT:
You MUST respond with a valid JSON object:
{
  "suggested_title": "Your improved title here",
  "confidence": 0.85
}

TITLE ENHANCEMENT GUIDELINES:
1. Make titles specific and descriptive
2. Include key topics or concepts
3. Keep titles concise (50-80 characters ideal, max 100)
4. Use title case (capitalize major words)
5. Avoid generic words like "Document", "Notes", "Untitled"
6. If current title is already good, return it unchanged with high confidence

CONFIDENCE SCORING:
- 0.9-1.0: Current title is already excellent, no change needed
- 0.7-0.9: Significant improvement made
- 0.5-0.7: Moderate improvement, some uncertainty
- Below 0.5: Content unclear, hard to suggest better title

GOOD TITLE IMPROVEMENTS:
- "Meeting Notes" → "Q4 Product Roadmap Planning Meeting - Jan 15, 2026"
- "Untitled" → "Customer Onboarding Checklist for Enterprise Accounts"
- "Doc" → "API Authentication Implementation Guide (OAuth 2.0)"
- "Project Plan" → "Website Redesign Project Plan: Timeline, Budget & Resources"

NO CHANGE NEEDED:
- "Q4 Sales Performance Analysis" → same (already descriptive)
- "Docker Deployment Guide for Production Environments" → same (clear and specific)

BAD TITLE IMPROVEMENTS:
- "Notes" → "Important Document" (still too vague)
- "Report" → "A Report About Sales and Marketing Metrics for Q4 2025 Including Revenue Growth and Customer Acquisition Costs" (way too long)

IMPORTANT:
- Only suggest changes when there's clear improvement
- Base suggestions on document content, not assumptions
- Ensure valid JSON syntax`
```

### User Prompt Builder

```go
func buildEnhanceTitleUserPromptV1(req interface{}) string {
    titleReq := req.(*ai.TitleRequest)

    // Only need beginning of content for title generation
    content := titleReq.DocumentContent
    if len(content) > 4000 { // ~1000 tokens
        content = truncateHead(content, 1000, "\n\n... (remaining content omitted) ...", tokenCounter)
    }

    return fmt.Sprintf("Current Title: %s\n\nDocument Content (excerpt):\n%s\n\nProvide an improved title in JSON format.",
        titleReq.CurrentTitle, content)
}
```

### Example Usage

```go
func ExampleEnhanceTitle() {
    client := ai.NewOpenAIClient(endpoint, apiKey, "gpt-4", 30*time.Second)

    req := &ai.TitleRequest{
        CurrentTitle:    "Meeting Notes",
        DocumentContent: "Discussed Q1 marketing budget allocation. Decided to increase social media spend by 30%, reduce print advertising by 50%. Target: 20% more qualified leads. Campaign launches Feb 1st. Sarah owns execution.",
    }

    resp, err := client.EnhanceTitle(context.Background(), req)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Original: %s\n", req.CurrentTitle)
    fmt.Printf("Suggested: %s (confidence: %.2f)\n", resp.SuggestedTitle, resp.Confidence)
}
```

### Example Responses

```json
{
  "suggested_title": "Q1 Marketing Budget Reallocation: Social Media Focus",
  "confidence": 0.88
}
```

```json
{
  "suggested_title": "API Rate Limiting Implementation Guide",
  "confidence": 0.82
}
```

```json
{
  "suggested_title": "Product Requirements Document: Real-Time Analytics Dashboard",
  "confidence": 0.95
}
```

## Prompt Template 5: Search Terms Extraction

### Purpose

Extract 5-10 relevant search terms and keywords to improve document discoverability.

### Response Schema

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["search_terms"],
  "properties": {
    "search_terms": {
      "type": "array",
      "description": "5-10 relevant search terms",
      "items": {"type": "string"},
      "minItems": 5,
      "maxItems": 10
    }
  }
}
```

### System Prompt

```go
const generateSearchTermsSystemPromptV1 = `You are a search term extraction assistant. Your task is to identify 5-10 relevant keywords and phrases that will help users find this document.

RESPONSE FORMAT:
You MUST respond with a valid JSON object:
{
  "search_terms": ["term1", "term2", "term3", "term4", "term5"]
}

EXTRACTION GUIDELINES:
1. Extract 5-10 search terms (aim for 7-8 typically)
2. Include a mix of:
   - Specific technical terms or concepts
   - General topic areas
   - Action-oriented phrases (if applicable)
   - Acronyms or abbreviations (if used in document)
3. Use lowercase for consistency
4. Keep terms concise (1-3 words per term)
5. Avoid overly generic terms like "document", "information", "content"

GOOD SEARCH TERMS:
- Technical: "oauth 2.0", "docker deployment", "REST API"
- Topic areas: "customer onboarding", "quarterly planning", "budget allocation"
- Action phrases: "troubleshooting guide", "best practices", "getting started"
- Specific: "Q4 2025", "enterprise tier", "GDPR compliance"

BAD SEARCH TERMS:
- Too generic: "business", "management", "process"
- Too long: "comprehensive guide to implementing authentication"
- Redundant: "API", "API documentation", "API docs" (pick one)

TERM SELECTION STRATEGY:
- Prioritize terms that appear multiple times in the document
- Include terms from the title
- Think about what users would search for to find this document
- Balance specificity (unique terms) with discoverability (common terms)

IMPORTANT:
- Return exactly 5-10 terms
- Use lowercase
- Ensure valid JSON array syntax`
```

### User Prompt Builder

```go
func buildSearchTermsUserPromptV1(req interface{}) string {
    searchReq := req.(*ai.SearchTermsRequest)

    // Use moderate content length for search term extraction
    content := searchReq.DocumentContent
    if len(content) > 5000 { // ~1250 tokens
        content = truncateTail(content, 1250, "\n\n... (content truncated) ...\n\n", tokenCounter)
    }

    return fmt.Sprintf("Document Title: %s\n\nDocument Content:\n%s\n\nExtract 5-10 search terms in JSON format.",
        searchReq.DocumentTitle, content)
}
```

### Example Usage

```go
func ExampleGenerateSearchTerms() {
    client := ai.NewOpenAIClient(endpoint, apiKey, "gpt-4", 30*time.Second)

    req := &ai.SearchTermsRequest{
        DocumentTitle:   "Kubernetes Production Deployment Checklist",
        DocumentContent: "This checklist covers essential steps for deploying applications to Kubernetes in production. Includes: namespace configuration, resource limits, health checks, monitoring setup, security policies, backup strategies, and rollback procedures. Follows cloud-native best practices.",
    }

    resp, err := client.GenerateSearchTerms(context.Background(), req)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Search terms: %v\n", resp.SearchTerms)
}
```

### Example Responses

```json
{
  "search_terms": [
    "kubernetes",
    "production deployment",
    "cloud-native",
    "resource limits",
    "health checks",
    "monitoring setup",
    "security policies",
    "rollback procedures"
  ]
}
```

```json
{
  "search_terms": [
    "Q4 2025",
    "sales performance",
    "revenue analysis",
    "customer acquisition",
    "conversion rate",
    "EMEA market",
    "churn rate"
  ]
}
```

## Prompt Template 6: Related Documents Discovery

### Purpose

Find semantically related documents from a provided list based on content similarity.

### Response Schema

```json
{
  "$schema": "http://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["related_documents"],
  "properties": {
    "related_documents": {
      "type": "array",
      "description": "Up to 5 related documents, ordered by relevance",
      "items": {
        "type": "object",
        "required": ["title", "relevance", "reason"],
        "properties": {
          "title": {
            "type": "string",
            "description": "Exact title from available documents"
          },
          "relevance": {
            "type": "number",
            "minimum": 0.0,
            "maximum": 1.0,
            "description": "Relevance score"
          },
          "reason": {
            "type": "string",
            "description": "Brief explanation of why this document is related"
          }
        }
      },
      "maxItems": 5
    }
  }
}
```

### System Prompt

```go
const findRelatedDocumentsSystemPromptV1 = `You are a document recommendation assistant. Your task is to find related documents from a provided list that would be relevant to users reading the current document.

RESPONSE FORMAT:
You MUST respond with a valid JSON object:
{
  "related_documents": [
    {
      "title": "Exact title from available documents list",
      "relevance": 0.85,
      "reason": "Brief explanation of relevance (1 sentence)"
    }
  ]
}

RECOMMENDATION GUIDELINES:
1. Identify up to 5 related documents from the provided list
2. Order results by relevance (most relevant first)
3. Only include documents with relevance score >= 0.5
4. Consider multiple types of relationships:
   - Topic similarity (same subject area)
   - Sequential relationship (previous/next in a series)
   - Prerequisite/dependency (foundational knowledge)
   - Cross-references (documents that reference each other)
5. Use the EXACT title from the available documents list

RELEVANCE SCORING:
- 0.9-1.0: Highly related, essential companion document
- 0.7-0.9: Strongly related, recommended reading
- 0.5-0.7: Moderately related, potentially useful
- Below 0.5: Not related enough to recommend

REASON GUIDELINES:
- Keep explanations brief (one sentence, 10-15 words)
- Be specific about the relationship
- Examples:
  - "Covers the prerequisites for understanding this implementation guide."
  - "Provides advanced techniques building on concepts introduced here."
  - "Discusses related authentication methods for comparison."

IMPORTANT:
- Use exact titles from the available documents list
- Do not make up document titles
- Return empty array if no documents are sufficiently related
- Ensure valid JSON syntax`
```

### User Prompt Builder

```go
func buildRelatedDocsUserPromptV1(req interface{}) string {
    relReq := req.(*ai.RelatedDocsRequest)
    var sb strings.Builder

    sb.WriteString("CURRENT DOCUMENT:\n")
    sb.WriteString(fmt.Sprintf("Title: %s\n\n", relReq.DocumentTitle))

    // Moderate content length for comparison
    content := relReq.DocumentContent
    if len(content) > 4000 { // ~1000 tokens
        content = truncateTail(content, 1000, "\n\n... (content truncated) ...\n\n", tokenCounter)
    }
    sb.WriteString(fmt.Sprintf("Content:\n%s\n\n", content))

    sb.WriteString("AVAILABLE DOCUMENTS:\n")
    for i, docTitle := range relReq.AvailableDocs {
        sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, docTitle))
    }

    sb.WriteString("\nFind up to 5 related documents and provide response in JSON format.")

    return sb.String()
}
```

### Example Usage

```go
func ExampleFindRelatedDocuments() {
    client := ai.NewOpenAIClient(endpoint, apiKey, "gpt-4", 30*time.Second)

    req := &ai.RelatedDocsRequest{
        DocumentTitle:   "OAuth 2.0 Implementation Guide",
        DocumentContent: "This guide walks through implementing OAuth 2.0 authentication in your application. Covers authorization code flow, token management, refresh tokens, and security best practices.",
        AvailableDocs: []string{
            "API Rate Limiting Best Practices",
            "JWT Token Structure and Validation",
            "User Authentication Flows Comparison",
            "CORS Configuration for Single Page Apps",
            "Securing API Endpoints with Middleware",
            "OpenID Connect Setup Guide",
            "Database Schema Design Principles",
            "OAuth 2.0 Troubleshooting Common Issues",
        },
    }

    resp, err := client.FindRelatedDocuments(context.Background(), req)
    if err != nil {
        panic(err)
    }

    for _, doc := range resp.RelatedDocuments {
        fmt.Printf("- %s (%.2f): %s\n", doc.Title, doc.Relevance, doc.Reason)
    }
}
```

### Example Response

```json
{
  "related_documents": [
    {
      "title": "OAuth 2.0 Troubleshooting Common Issues",
      "relevance": 0.95,
      "reason": "Directly related troubleshooting guide for OAuth 2.0 implementations."
    },
    {
      "title": "JWT Token Structure and Validation",
      "relevance": 0.88,
      "reason": "Covers token management details essential for OAuth 2.0 implementations."
    },
    {
      "title": "OpenID Connect Setup Guide",
      "relevance": 0.82,
      "reason": "Extends OAuth 2.0 with identity layer, natural next step."
    },
    {
      "title": "Securing API Endpoints with Middleware",
      "relevance": 0.75,
      "reason": "Shows how to use OAuth tokens for endpoint protection."
    },
    {
      "title": "User Authentication Flows Comparison",
      "relevance": 0.68,
      "reason": "Provides broader context comparing OAuth to other authentication methods."
    }
  ]
}
```

## Testing Prompts

### Unit Testing Strategy

```go
package prompts

import (
    "testing"
    "encoding/json"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestPromptVersioning(t *testing.T) {
    tests := []struct {
        name    string
        version PromptVersion
        wantErr bool
    }{
        {"existing version", ClassifyDocumentV1, false},
        {"missing version", PromptVersion("nonexistent-v1.0"), true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := GetPrompt(tt.version)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}

func TestClassificationPromptBuilder(t *testing.T) {
    req := &ai.ClassificationRequest{
        DocumentTitle:   "Test Document",
        DocumentContent: "Test content",
        Taxonomy: &ai.TaxonomyContext{
            Collections: []ai.TaxonomyCollection{
                {ID: "col1", Name: "Collection 1", Description: "Test collection"},
            },
        },
    }

    prompt := buildClassifyUserPromptV1(req)

    assert.Contains(t, prompt, "Test Document")
    assert.Contains(t, prompt, "Test content")
    assert.Contains(t, prompt, "Collection 1")
}

func TestTruncationStrategy(t *testing.T) {
    counter := NewTokenCounter("gpt-4")

    tests := []struct {
        name      string
        content   string
        maxTokens int
        strategy  TruncationStrategy
    }{
        {"no truncation needed", "short content", 100, TruncateTail},
        {"truncate tail", strings.Repeat("word ", 1000), 50, TruncateTail},
        {"truncate middle", strings.Repeat("word ", 1000), 50, TruncateMiddle},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cfg := TruncationConfig{
                MaxTokens: tt.maxTokens,
                Strategy:  tt.strategy,
                Marker:    "...",
            }

            result := TruncateContent(tt.content, cfg, counter)
            tokens := counter.CountTokens(result)

            assert.LessOrEqual(t, tokens, tt.maxTokens)
        })
    }
}

func TestResponseSchemaValidation(t *testing.T) {
    tests := []struct {
        name     string
        response string
        wantErr  bool
    }{
        {
            name: "valid classification response",
            response: `{
                "collection_id": "col1",
                "confidence": 0.9,
                "reasoning": "Test reasoning",
                "search_terms": ["term1", "term2", "term3", "term4", "term5"]
            }`,
            wantErr: false,
        },
        {
            name: "missing required field",
            response: `{
                "collection_id": "col1",
                "confidence": 0.9
            }`,
            wantErr: true,
        },
        {
            name: "invalid confidence range",
            response: `{
                "collection_id": "col1",
                "confidence": 1.5,
                "reasoning": "Test",
                "search_terms": ["term1"]
            }`,
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            var resp ai.ClassificationResponse
            err := json.Unmarshal([]byte(tt.response), &resp)

            if tt.wantErr {
                assert.Error(t, err)
            } else {
                require.NoError(t, err)
                // Validate additional constraints
                assert.GreaterOrEqual(t, resp.Confidence, 0.0)
                assert.LessOrEqual(t, resp.Confidence, 1.0)
            }
        })
    }
}
```

### Integration Testing with Mock AI

```go
package prompts_test

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestPromptIntegration_Classification(t *testing.T) {
    // Use a mock AI client that returns predictable responses
    mockClient := &MockAIClient{
        ResponseFunc: func(system, user string) (string, error) {
            return `{
                "collection_id": "col1",
                "confidence": 0.92,
                "reasoning": "Document clearly discusses engineering topics",
                "search_terms": ["engineering", "architecture", "design"]
            }`, nil
        },
    }

    req := &ai.ClassificationRequest{
        DocumentTitle:   "System Architecture Design",
        DocumentContent: "This document describes our microservices architecture...",
        Taxonomy: &ai.TaxonomyContext{
            Collections: []ai.TaxonomyCollection{
                {ID: "col1", Name: "Engineering", Description: "Technical docs"},
            },
        },
    }

    resp, err := mockClient.ClassifyDocument(context.Background(), req)

    require.NoError(t, err)
    assert.Equal(t, "col1", resp.CollectionID)
    assert.GreaterOrEqual(t, resp.Confidence, 0.9)
    assert.NotEmpty(t, resp.SearchTerms)
}

func TestPromptIntegration_LowConfidence(t *testing.T) {
    mockClient := &MockAIClient{
        ResponseFunc: func(system, user string) (string, error) {
            return `{
                "collection_id": "col1",
                "confidence": 0.62,
                "reasoning": "Could fit multiple collections",
                "alternatives": [
                    {"collection_id": "col2", "confidence": 0.58, "reasoning": "Also relevant"}
                ],
                "search_terms": ["general", "cross-functional"]
            }`, nil
        },
    }

    resp, err := mockClient.ClassifyDocument(context.Background(), req)

    require.NoError(t, err)
    assert.Less(t, resp.Confidence, 0.9)
    assert.NotEmpty(t, resp.Alternatives)
}
```

### Prompt Quality Testing

```go
package prompts

// Test prompt quality metrics
func TestPromptQualityMetrics(t *testing.T) {
    template := PromptRegistry[ClassifyDocumentV1]

    // System prompt should be comprehensive
    assert.Greater(t, len(template.SystemPrompt), 500, "system prompt too short")
    assert.Contains(t, template.SystemPrompt, "JSON", "must specify JSON format")
    assert.Contains(t, template.SystemPrompt, "confidence", "must mention confidence")

    // Token limits should be reasonable
    assert.Greater(t, template.MaxTokens, 0, "max tokens must be positive")
    assert.Less(t, template.MaxTokens, 4096, "max tokens should be conservative")

    // Temperature should be low for consistency
    assert.LessOrEqual(t, template.Temperature, 0.5, "temperature too high for structured output")
}

func BenchmarkPromptBuilding(b *testing.B) {
    req := &ai.ClassificationRequest{
        DocumentTitle:   "Test Document",
        DocumentContent: strings.Repeat("content ", 1000),
        Taxonomy: &ai.TaxonomyContext{
            Collections: make([]ai.TaxonomyCollection, 50),
        },
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = buildClassifyUserPromptV1(req)
    }
}
```

## Package Structure

```
internal/prompts/
├── registry.go           # Prompt version registry
├── templates.go          # All prompt templates (system & user builders)
├── tokens.go             # Token counting and truncation
├── schemas.go            # JSON schema validation
├── prompts_test.go       # Unit tests
└── integration_test.go   # Integration tests with mock AI
```

### Registry Implementation

```go
// internal/prompts/registry.go
package prompts

import "fmt"

type PromptVersion string

const (
    ClassifyDocumentV1    PromptVersion = "classify-v1.0"
    AnswerQuestionV1      PromptVersion = "answer-v1.0"
    GenerateSummaryV1     PromptVersion = "summary-v1.0"
    EnhanceTitleV1        PromptVersion = "title-v1.0"
    GenerateSearchTermsV1 PromptVersion = "searchterms-v1.0"
    RelatedDocumentsV1    PromptVersion = "related-v1.0"
)

type UserPromptBuilder func(interface{}) string

type PromptTemplate struct {
    Version           PromptVersion
    SystemPrompt      string
    UserPromptBuilder UserPromptBuilder
    MaxTokens         int
    Temperature       float32
    Description       string
}

var PromptRegistry = map[PromptVersion]PromptTemplate{
    ClassifyDocumentV1: {
        Version:           ClassifyDocumentV1,
        SystemPrompt:      classifySystemPromptV1,
        UserPromptBuilder: buildClassifyUserPromptV1,
        MaxTokens:         1000,
        Temperature:       0.3,
        Description:       "Document classification with confidence and alternatives",
    },
    AnswerQuestionV1: {
        Version:           AnswerQuestionV1,
        SystemPrompt:      answerQuestionSystemPromptV1,
        UserPromptBuilder: buildAnswerQuestionUserPromptV1,
        MaxTokens:         1500,
        Temperature:       0.4,
        Description:       "Question answering with citations from context documents",
    },
    GenerateSummaryV1: {
        Version:           GenerateSummaryV1,
        SystemPrompt:      generateSummarySystemPromptV1,
        UserPromptBuilder: buildSummaryUserPromptV1,
        MaxTokens:         300,
        Temperature:       0.4,
        Description:       "2-3 sentence document summary generation",
    },
    EnhanceTitleV1: {
        Version:           EnhanceTitleV1,
        SystemPrompt:      enhanceTitleSystemPromptV1,
        UserPromptBuilder: buildEnhanceTitleUserPromptV1,
        MaxTokens:         200,
        Temperature:       0.3,
        Description:       "Document title enhancement for vague titles",
    },
    GenerateSearchTermsV1: {
        Version:           GenerateSearchTermsV1,
        SystemPrompt:      generateSearchTermsSystemPromptV1,
        UserPromptBuilder: buildSearchTermsUserPromptV1,
        MaxTokens:         200,
        Temperature:       0.3,
        Description:       "Extract 5-10 search terms for document discovery",
    },
    RelatedDocumentsV1: {
        Version:           RelatedDocumentsV1,
        SystemPrompt:      findRelatedDocumentsSystemPromptV1,
        UserPromptBuilder: buildRelatedDocsUserPromptV1,
        MaxTokens:         800,
        Temperature:       0.3,
        Description:       "Find up to 5 related documents from provided list",
    },
}

func GetPrompt(version PromptVersion) (PromptTemplate, error) {
    template, exists := PromptRegistry[version]
    if !exists {
        return PromptTemplate{}, fmt.Errorf("prompt version %s not found in registry", version)
    }
    return template, nil
}

func ListPrompts() []PromptTemplate {
    templates := make([]PromptTemplate, 0, len(PromptRegistry))
    for _, template := range PromptRegistry {
        templates = append(templates, template)
    }
    return templates
}
```

## Token Limit Considerations by Template

| Template | Max Input Tokens | Max Output Tokens | Total Budget | Notes |
|----------|-----------------|-------------------|--------------|-------|
| Classification | ~2500 | 1000 | ~3500 | Truncate document content at 2000 tokens |
| Q&A | ~3500 | 1500 | ~5000 | Limit context docs, ~1000 tokens each |
| Summary | ~1500 | 300 | ~1800 | Only need first portion of document |
| Title | ~1000 | 200 | ~1200 | Only need beginning of content |
| Search Terms | ~1250 | 200 | ~1450 | Moderate content length sufficient |
| Related Docs | ~1500 | 800 | ~2300 | Truncate main document, list all titles |

### Provider-Specific Limits

```go
package prompts

var ProviderLimits = map[string]int{
    "gpt-3.5-turbo":  4096,
    "gpt-4":          8192,
    "gpt-4-32k":      32768,
    "claude-2":       100000,
    "claude-3-opus":  200000,
    "local-llama-2":  4096,
}

func GetProviderLimit(model string) int {
    if limit, exists := ProviderLimits[model]; exists {
        return limit
    }
    return 4096 // Safe default
}
```

## SOHO Deployment Considerations

### Cost Management

```go
package prompts

type CostEstimator struct {
    inputCostPer1K  float64
    outputCostPer1K float64
}

func NewCostEstimator(model string) *CostEstimator {
    // GPT-4 pricing as example
    return &CostEstimator{
        inputCostPer1K:  0.03,
        outputCostPer1K: 0.06,
    }
}

func (e *CostEstimator) EstimateRequestCost(inputTokens, outputTokens int) float64 {
    inputCost := float64(inputTokens) / 1000.0 * e.inputCostPer1K
    outputCost := float64(outputTokens) / 1000.0 * e.outputCostPer1K
    return inputCost + outputCost
}
```

### Local Model Optimization

```go
// For local models (Ollama, etc.), optimize prompts for smaller context windows
const classifySystemPromptV1Local = `You are a document classifier.

OUTPUT JSON:
{"collection_id": "id", "confidence": 0.85, "reasoning": "why", "search_terms": ["term1", "term2"]}

INSTRUCTIONS:
- Analyze document title and content
- User guidance overrides content
- Confidence: 0.0-1.0
- Include alternatives if confidence < 0.9
- Generate 5-10 search terms

Choose from provided collection IDs only.`
```

## Prompt Evolution Strategy

### Adding New Versions

1. **Create new version constant**:
   ```go
   const ClassifyDocumentV2 PromptVersion = "classify-v2.0"
   ```

2. **Add new template to registry**:
   ```go
   PromptRegistry[ClassifyDocumentV2] = PromptTemplate{...}
   ```

3. **Keep old version** for backward compatibility

4. **Update default** in configuration:
   ```yaml
   ai:
     prompts:
       classification_version: "classify-v2.0"
   ```

### Migration Path

```go
package prompts

func MigratePromptVersion(old, new PromptVersion) error {
    // Validation logic
    if !IsCompatible(old, new) {
        return fmt.Errorf("incompatible schema change from %s to %s", old, new)
    }
    return nil
}

func IsCompatible(old, new PromptVersion) bool {
    // Check if response schemas are compatible
    // Allow minor version upgrades automatically
    // Require user confirmation for major version changes
    return extractMajorVersion(old) == extractMajorVersion(new)
}
```

## Configuration Integration

### Example Configuration

```yaml
ai:
  endpoint: "https://api.openai.com/v1"
  api_key: "${OPENAI_API_KEY}"
  model: "gpt-4"
  timeout: 30s

  prompts:
    classification_version: "classify-v1.0"
    qa_version: "answer-v1.0"
    summary_version: "summary-v1.0"
    title_version: "title-v1.0"
    search_terms_version: "searchterms-v1.0"
    related_docs_version: "related-v1.0"

  token_limits:
    classification_max_input: 2000
    qa_max_context_docs: 5
    summary_max_input: 1500

  truncation:
    marker: "\n\n... (content truncated for length) ...\n\n"
    strategy: "tail"  # head, tail, or middle
```

---

**Status:** Ready for implementation
**Complexity:** Medium
**Priority:** High (core AI functionality)
**Dependencies:** LLD-05 (AI Client)
