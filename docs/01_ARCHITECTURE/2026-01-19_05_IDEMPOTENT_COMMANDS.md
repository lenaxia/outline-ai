# Idempotent Command Design

## Overview
The `/summarize` command and search terms generation are designed to be **idempotent** - running them multiple times produces clean results without duplicating or accumulating content.

## Problem Statement

### Without Idempotency
```markdown
[First run of /summarize]
> **Summary**: This document describes database migrations...

[Content...]

[Second run of /summarize - PROBLEM!]
> **Summary**: This document describes database migrations...
> **Summary**: This document outlines our migration strategy...

[Content...]

[Result: Duplicate summaries, messy document]
```

### With Idempotency
```markdown
[First run of /summarize]
<!-- AI-SUMMARY-START -->
> **Summary**: This document describes database migrations...
<!-- AI-SUMMARY-END -->

[Content...]

[Second run of /summarize - CLEAN!]
<!-- AI-SUMMARY-START -->
> **Summary**: This document outlines our migration strategy...
<!-- AI-SUMMARY-END -->

[Content...]

[Result: Single, updated summary]
```

## Design Pattern: Hidden Markers

### Why HTML Comments?
- **Invisible in rendered markdown**: Users see clean content
- **Preserved in source**: Markdown editors don't strip them
- **Easy to detect**: Simple regex or string search
- **Standard**: Works in all markdown implementations
- **Non-invasive**: Doesn't affect document semantics

### Marker Format

**For Summaries:**
```markdown
<!-- AI-SUMMARY-START -->
> **Summary**: [AI-generated summary text here]
<!-- AI-SUMMARY-END -->
```

**For Search Terms:**
```markdown
---
<!-- AI-SEARCH-TERMS-START -->
**Search Terms**: term1, term2, term3
<!-- AI-SEARCH-TERMS-END -->
```

## Replacement Algorithm

### 1. Check for Existing Markers
```go
func HasExistingSummary(content string) (bool, int, int) {
    startIdx := strings.Index(content, "<!-- AI-SUMMARY-START -->")
    endIdx := strings.Index(content, "<!-- AI-SUMMARY-END -->")

    if startIdx >= 0 && endIdx > startIdx {
        return true, startIdx, endIdx + len("<!-- AI-SUMMARY-END -->")
    }
    return false, -1, -1
}
```

### 2. Replace Between Markers
```go
func ReplaceSummary(content, newSummary string) string {
    hasMarkers, startIdx, endIdx := HasExistingSummary(content)

    if hasMarkers {
        // Replace content between markers
        before := content[:startIdx]
        after := content[endIdx:]

        newSection := fmt.Sprintf(
            "<!-- AI-SUMMARY-START -->\n> **Summary**: %s\n<!-- AI-SUMMARY-END -->",
            newSummary,
        )

        return before + newSection + after
    }

    // No markers - add new summary at top
    return AddNewSummary(content, newSummary)
}
```

### 3. Detect Existing Format Without Markers
```go
func DetectSummaryWithoutMarkers(content string) (bool, int, int) {
    // Look for "> **Summary**:" at start of document
    lines := strings.Split(content, "\n")

    for i, line := range lines {
        if strings.TrimSpace(line) == "" {
            continue // Skip empty lines at start
        }

        // First non-empty line
        if strings.HasPrefix(strings.TrimSpace(line), "> **Summary**:") {
            // Find end of blockquote
            endIdx := i + 1
            for endIdx < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[endIdx]), ">") {
                endIdx++
            }
            return true, i, endIdx
        }

        // First non-empty line doesn't match - no summary
        break
    }

    return false, -1, -1
}
```

## User Ownership Handling

### Scenario 1: User Makes Minor Edits
**User edits summary text but keeps markers:**
```markdown
<!-- AI-SUMMARY-START -->
> **Summary**: This document describes database migrations
> with automated rollback procedures.  <-- User added this line
<!-- AI-SUMMARY-END -->
```

**Next /summarize run:**
- Markers detected ✓
- Content between markers replaced
- User edits overwritten with fresh AI summary
- **Rationale**: Markers present = AI maintains ownership

### Scenario 2: User Takes Ownership
**User removes markers:**
```markdown
> **Summary**: This document describes database migrations
> with automated rollback procedures and manual review steps.
```

**Next /summarize run:**
- No markers found
- `detect_existing_format: true` - finds summary format
- `respect_no_markers: true` - skips update
- **Rationale**: No markers = user ownership, don't interfere

### Scenario 3: User Deletes Everything
**User removes both markers and summary:**
```markdown
[Document starts with regular content...]
```

**Next /summarize run:**
- No markers found
- No summary format detected
- Adds fresh summary with markers at top
- **Rationale**: Clean slate, safe to add

## Configuration Options

### Global Settings
```yaml
enhancement:
  idempotent_updates: true         # Enable marker-based idempotency
  respect_user_ownership: true     # Skip if markers removed
```

### Per-Feature Settings
```yaml
commands:
  summarize:
    use_markers: true              # Add HTML comment markers
    respect_no_markers: true       # Skip if no markers (user ownership)
    detect_existing_format: true   # Try to find summaries without markers

  search_terms:
    use_markers: true
    respect_no_markers: true
```

### Behavior Matrix

| Markers Present | Existing Format | `respect_no_markers` | Action |
|----------------|----------------|---------------------|---------|
| ✓ Yes | N/A | N/A | **Replace** between markers |
| ✗ No | ✓ Yes | `true` | **Skip** (user ownership) |
| ✗ No | ✓ Yes | `false` | **Replace** and add markers |
| ✗ No | ✗ No | N/A | **Add new** with markers |

## Implementation Details

### Marker Detection Performance
- **String search**: O(n) where n = document length
- **Typical overhead**: < 1ms for documents up to 100KB
- **Optimization**: Search only first 5KB for summary markers (summaries are at top)
- **Search terms**: Search only last 10KB (search terms at bottom)

### Edge Cases Handled

1. **Malformed markers**: Missing start or end marker
   - Behavior: Treat as no markers, add fresh section
   - Log warning for debugging

2. **Multiple marker sets**: User accidentally duplicates markers
   - Behavior: Replace first set, log warning
   - Optional: Clean up duplicate markers

3. **Markers in code blocks**: User discussing the system in document
   - Behavior: Only match markers NOT in code blocks (```...```)
   - Use markdown-aware parser

4. **Nested blockquotes**: User has complex quote formatting
   - Behavior: Preserve user formatting, only replace marker content
   - Don't break user's markdown structure

5. **Very long summaries**: AI generates unexpectedly long summary
   - Behavior: Truncate to max length (e.g., 500 chars)
   - Log warning, suggest refinement

## User Communication

### Success Messages (via comment)
```markdown
✓ Summary updated (removed previous AI-generated summary)
```

### User Ownership Detected
```markdown
ℹ️ Summary not updated - markers removed (respecting your edits)
```

### First-Time Addition
```markdown
✓ Summary added at top of document
```

## Benefits

### For Users
1. **Clean documents**: No duplicate content accumulation
2. **Predictable behavior**: Re-running commands is safe
3. **User control**: Can take ownership by removing markers
4. **Transparency**: Comments show what AI maintains

### For System
1. **Simpler logic**: No need to track "last updated" timestamps
2. **Stateless**: Don't need database to remember what we generated
3. **Robust**: Works even if document moved/copied
4. **Debuggable**: Easy to see what AI manages vs user content

### For Maintenance
1. **Future-proof**: New commands can use same pattern
2. **Testable**: Clear input/output for each scenario
3. **Extensible**: Easy to add new marker types
4. **Documented**: Markers self-document AI involvement

## Alternative Approaches Considered

### 1. Timestamp-Based Tracking
**Approach**: Store timestamp of last update in database
**Rejected because**:
- Requires persistent state
- Breaks if document copied/moved
- Can't detect user edits
- More complex error recovery

### 2. Content Hashing
**Approach**: Hash generated content, detect if changed
**Rejected because**:
- Can't distinguish user edits from AI updates
- Requires state storage
- Fails if user makes minor formatting changes
- Complex diff logic needed

### 3. Metadata Section
**Approach**: YAML frontmatter with metadata
**Rejected because**:
- Not all markdown parsers support frontmatter
- Outline may not preserve frontmatter
- Visible to users (clutter)
- Harder to edit

### 4. HTML Data Attributes
**Approach**: `<div data-ai-generated="summary">...</div>`
**Rejected because**:
- Doesn't render as nicely in markdown
- Some markdown parsers strip HTML
- Less natural for users to edit

## Future Enhancements

### 1. Smart Merge
Instead of replacing, merge user edits with new AI content:
- Detect what user changed
- Preserve user's additions
- Update only AI-specific parts

**Complexity**: High - requires diff algorithm
**Value**: Medium - most users prefer clean replacement
**Priority**: Low

### 2. Version History
Track all AI-generated versions:
```markdown
<!-- AI-SUMMARY-VERSION=3 DATE=2026-01-19 -->
```

**Complexity**: Medium
**Value**: Low - Outline has built-in version history
**Priority**: Low

### 3. Marker Types for Different AI Models
```markdown
<!-- AI-SUMMARY-MODEL=gpt-4 CONFIDENCE=0.95 -->
```

**Complexity**: Low
**Value**: Medium - useful for debugging
**Priority**: Medium

### 4. User Feedback Loop
Allow users to rate AI-generated content:
```markdown
<!-- AI-SUMMARY-START -->
> **Summary**: ...
<!-- AI-SUMMARY-END -->
👍 👎 [User can click in Outline UI]
```

**Complexity**: High - requires Outline integration
**Value**: High - improves AI over time
**Priority**: Future (post-v1)

## Testing Strategy

### Unit Tests
```go
func TestReplaceSummary_WithExistingMarkers(t *testing.T)
func TestReplaceSummary_WithoutMarkers(t *testing.T)
func TestReplaceSummary_UserOwnership(t *testing.T)
func TestReplaceSummary_MalformedMarkers(t *testing.T)
func TestReplaceSummary_MarkersInCodeBlock(t *testing.T)
```

### Integration Tests
```go
func TestIdempotency_MultipleSummarizeRuns(t *testing.T)
func TestIdempotency_UserEditsPreserved(t *testing.T)
func TestIdempotency_UserOwnershipRespected(t *testing.T)
```

### Manual Test Cases
1. Run `/summarize` 5 times on same document
2. User edits summary, run `/summarize` again
3. User removes markers, run `/summarize` again
4. User copies document with markers to new location
5. User has nested blockquotes in summary area

## Conclusion

The hidden marker pattern provides:
- ✅ **Idempotent operations** - safe to re-run
- ✅ **User control** - can take ownership
- ✅ **Clean UI** - markers invisible in rendered view
- ✅ **Simple implementation** - string manipulation only
- ✅ **Stateless** - no database tracking needed
- ✅ **Robust** - handles edge cases gracefully

This pattern can be extended to other AI-generated content sections as the system evolves.
