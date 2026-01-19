# Interactive Guidance Feature for /ai-file Command

## Overview
Enhanced the `/ai-file` command with optional user guidance and an interactive feedback loop for uncertain filing decisions.

## Feature Components

### 1. Optional User Guidance
Users can provide hints to help the AI make better filing decisions:

```markdown
/ai-file                              # No guidance
/ai-file technical documentation      # Guide toward technical collections
/ai-file customer-facing              # Guide toward customer/product collections
/ai-file backend API implementation   # Very specific guidance
```

**How it works:**
- Guidance is parsed from the command line after `/ai-file`
- Passed to AI along with document content and taxonomy
- AI considers guidance in addition to content analysis
- Improves confidence and accuracy

### 2. Interactive Feedback Loop

**Low Confidence Scenario:**

When AI confidence < threshold (default 0.7):
1. System converts `/ai-file` → `?ai-file` in the document
2. Adds a comment explaining the uncertainty:
   ```
   ⚠️ Unable to file with confidence. Uncertain between:
   - Engineering (API implementation details)
   - Product (mobile app features)

   To help me decide:
   - Edit this line to: /ai-file engineering focus
   - Or add a new line: /ai-file product documentation

   [AI Confidence: 55%]
   ```
3. User reviews and either:
   - Updates `?ai-file` to `/ai-file <better guidance>`
   - OR adds a new `/ai-file <guidance>` line
4. Next polling cycle detects the updated command
5. Reprocesses with new guidance
6. If successful, removes BOTH markers

### 3. Marker Semantics

**`/ai-file [guidance]`** - Active command
- Will be processed in next polling cycle
- Optional guidance helps AI
- Removed after successful filing

**`?ai-file [guidance]`** - Uncertain marker
- Indicates previous filing attempt had low confidence
- Not processed (waits for user to convert back to `/ai-file`)
- Shows user where AI got stuck
- Preserves any original guidance user provided

**Both markers present:**
- Process the `/ai-file` command
- Remove BOTH markers on successful filing
- Handles case where user adds new `/ai-file` without removing `?ai-file`

## AI Response Format

Enhanced to include alternatives for low confidence scenarios:

```json
{
  "collection_id": "engineering_123",
  "confidence": 0.55,
  "reasoning": "Could be Engineering (API implementation) or Product (mobile features)",
  "alternatives": [
    {
      "collection_id": "product_456",
      "collection_name": "Product",
      "reason": "Mobile app features and user-facing documentation"
    }
  ],
  "search_terms": ["API", "mobile", "documentation"]
}
```

**Fields:**
- `collection_id`: Primary recommendation
- `confidence`: 0.0-1.0 score
- `reasoning`: Explanation for primary choice
- `alternatives`: Other possible collections (only when confidence < threshold)
- `search_terms`: Keywords regardless of confidence

## Configuration Options

```yaml
commands:
  filing:
    include_alternatives: true     # Show alternative collections in comment
    max_alternatives: 3            # Limit alternatives shown
    success_comment: true          # Comment on successful filing
    uncertainty_comment: true      # Comment explaining low confidence
```

## User Experience Flows

### Flow 1: High Confidence (No Guidance Needed)
```
User: [creates document, adds /ai-file]
     ↓
System: [analyzes, confidence = 0.95]
     ↓
System: [files to Engineering, removes /ai-file]
     ↓
System: [adds comment: "✓ Filed to Engineering (confidence: 95%)"]
```

### Flow 2: Low Confidence → User Provides Guidance
```
User: [creates document, adds /ai-file]
     ↓
System: [analyzes, confidence = 0.55]
     ↓
System: [converts to ?ai-file, adds comment explaining uncertainty]
     ↓
User: [reads comment, updates: "?ai-file" → "/ai-file backend focused"]
     ↓
System: [analyzes with guidance, confidence = 0.92]
     ↓
System: [files to Engineering, removes both markers]
     ↓
System: [adds comment: "✓ Filed to Engineering (confidence: 92%) - Thank you for the guidance!"]
```

### Flow 3: Preemptive Guidance (User Knows It's Ambiguous)
```
User: [creates document, adds "/ai-file customer success focused"]
     ↓
System: [analyzes with guidance, confidence = 0.88]
     ↓
System: [files to Customer Success, removes /ai-file]
     ↓
System: [adds comment: "✓ Filed to Customer Success (confidence: 88%) - Used your guidance"]
```

## Benefits

### For Users
1. **Transparency**: See when AI is uncertain
2. **Control**: Can guide AI to correct destination
3. **Learning**: Understand what content is ambiguous
4. **No Surprises**: Documents don't disappear to wrong collections
5. **Flexibility**: Can provide guidance upfront or wait for AI to ask

### For System
1. **Higher Accuracy**: User guidance improves filing decisions
2. **Fewer Mistakes**: Don't file when uncertain
3. **User Trust**: Transparent about limitations
4. **Iterative Improvement**: Learn from user feedback over time

### Compared to Alternatives
**Better than automatic guessing:**
- No silent failures
- No documents in wrong places
- No need to search and re-file

**Better than requiring guidance always:**
- Friction-free for clear cases
- Only asks when needed
- Progressive disclosure of complexity

**Better than leaving in place silently:**
- User knows filing was attempted
- Clear next steps provided
- Easy to provide missing context

## Implementation Considerations

### Command Detection
- Must detect both `/ai-file` and `?ai-file` markers
- Parse guidance text after command
- Handle both markers in same document

### Document Editing
- Convert `/ai-file` → `?ai-file` (in-place edit)
- Preserve any existing guidance
- Remove both markers on success

### Comment Generation
- Format uncertainty comment clearly
- Include specific alternatives from AI
- Provide actionable guidance examples
- Show confidence score for transparency

### State Management
- Don't need persistent state (markers are in document)
- Re-process if `/ai-file` detected (even if `?ai-file` exists)
- Clean up both markers on success

## Testing Scenarios

1. **High confidence, no guidance**: Files successfully
2. **High confidence, with guidance**: Files successfully, acknowledges guidance
3. **Low confidence, no guidance**: Converts to `?ai-file`, asks for help
4. **Low confidence, user adds guidance**: Retries with guidance, files successfully
5. **Both markers present**: Processes new command, cleans up both
6. **User edits `?ai-file` to `/ai-file`**: Detects change, reprocesses
7. **Invalid guidance**: AI still makes best effort, may ask for clarification again
8. **User ignores `?ai-file`**: Stays as question marker, doesn't re-process until user acts

## Future Enhancements

- **Learning from feedback**: Remember user corrections to improve future suggestions
- **Smart defaults per collection**: If user always files "API" docs to Engineering, suggest that
- **Confidence calibration**: Adjust threshold based on user acceptance rate
- **Bulk operations**: `/ai-file-all <guidance>` to file multiple documents with same hint
- **Template guidance**: Common guidance phrases saved as shortcuts
