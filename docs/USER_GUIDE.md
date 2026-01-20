# Outline AI Assistant - User Guide

Welcome! This guide will help you use the AI Assistant commands in your Outline documents.

---

## Table of Contents

1. [Introduction](#introduction)
2. [Available Commands](#available-commands)
3. [Interactive Guidance Loop](#interactive-guidance-loop)
4. [Idempotent Operations](#idempotent-operations)
5. [Troubleshooting](#troubleshooting)
6. [FAQ](#faq)
7. [Examples Gallery](#examples-gallery)

---

## Introduction

### What is the AI Assistant?

The AI Assistant is a smart helper that lives inside your Outline workspace. It can:
- File documents to the right collection automatically
- Answer questions using your workspace knowledge
- Generate summaries of long documents
- Improve vague document titles
- Find related documents

### How Does It Work?

It's simple! Just type special commands (like `/ai-file` or `/ai`) in your documents. The AI Assistant detects these commands and takes action within seconds. It reads your document, understands the content, and helps you organize and enhance it.

The magic happens in real-time - as soon as you save your document, the AI springs into action!

### When Should I Use It?

Use the AI Assistant when you:
- Create a new document and aren't sure where it belongs
- Have a question about something in your workspace
- Want a quick summary of a long document
- Need to improve a placeholder title like "Draft" or "Notes"
- Want to find documents related to what you're reading

---

## Available Commands

### `/ai-file` - Smart Document Filing

**What it does:** Analyzes your document and moves it to the most appropriate collection.

**When to use it:**
- You've created a document but aren't sure which collection it belongs in
- You want the AI to organize your document for you
- You're documenting something that could fit multiple categories

**How to use it:**

Simply add the command anywhere in your document:
```markdown
/ai-file
```

Or provide guidance to help the AI:
```markdown
/ai-file engineering documentation
```

**Real Example:**

Let's say you create a document titled "API Rate Limiting Strategy" with technical content about implementing rate limits. Here's what happens:

1. You add `/ai-file` at the top of your document
2. Within seconds, the AI analyzes your content
3. It determines this is engineering documentation (95% confidence)
4. Your document is moved to the "Engineering" collection
5. The `/ai-file` command is removed
6. A comment appears: "Filed to Engineering (confidence: 95%)"

**What happens after:**
- Your document is in the right place
- The command marker is removed automatically
- A comment confirms the action
- Search terms are added to help future discovery

**Tips and best practices:**
- Add the command when you're done writing (not while drafting)
- If you know it's ambiguous, provide guidance: `/ai-file customer-facing`
- One command per document is enough
- The AI won't file your document twice - it's safe!

---

### `/ai` - Ask a Question

**What it does:** Answers your question using knowledge from across your workspace.

**When to use it:**
- You need to find information quickly
- Multiple documents might contain the answer
- You want a synthesized answer, not just search results
- You're new and learning about your organization

**How to use it:**

Type the command followed by your question:
```markdown
/ai What is our deployment process?
```

**Real Example:**

You're working on a project and need to understand the authentication flow:

1. You type: `/ai How does our authentication system work?`
2. The AI searches your workspace for relevant documents
3. It finds docs like "Auth Architecture", "API Security", "Login Flow"
4. Within 3-5 seconds, a comment appears with the answer:

> **AI Answer**
>
> Your authentication system uses JWT tokens with a two-tier approach. Users log in via OAuth2, receive an access token (15 min expiry) and a refresh token (7 day expiry). The API validates tokens using the public key from the auth service.
>
> **Sources:**
> - [Auth Architecture](outline://doc/abc123)
> - [API Security Guide](outline://doc/def456)
>
> *Confidence: 92%*

**What happens after:**
- Your question command is removed
- The answer stays as a comment (you can resolve it later)
- Other team members can see the Q&A
- The question won't be answered again if asked twice

**Tips and best practices:**
- Ask specific questions for better answers
- Include relevant keywords in your question
- You can ask follow-up questions by adding another `/ai` command
- Works great for onboarding new team members

---

### `/summarize` - Generate a Summary

**What it does:** Creates a 2-3 sentence summary and places it at the top of your document.

**When to use it:**
- Your document is long and needs a quick overview
- You want readers to understand the main points quickly
- You've updated content and need a fresh summary
- You're creating documentation and want consistency

**How to use it:**

Add the command anywhere:
```markdown
/summarize
```

**Real Example:**

You have a 3-page document about database migration procedures. You add `/summarize` at the bottom:

**Before:**
```markdown
# Database Migration Procedure

[3 pages of detailed technical content...]

/summarize
```

**After (within seconds):**
```markdown
# Database Migration Procedure

> **Summary**: This document outlines our approach to database migrations using
> zero-downtime techniques and automated rollback procedures. It covers migration
> scripts, testing requirements, and production deployment steps.

[3 pages of detailed technical content...]
```

**What happens after:**
- A summary appears at the top (inside a blockquote)
- The `/summarize` command is removed
- Hidden markers track the summary (more on this below)
- You can edit the summary if needed

**Multiple runs are safe!** If you run `/summarize` again (after updating your document), it cleanly replaces the old summary - no duplicates!

**Tips and best practices:**
- Run `/summarize` after major content changes
- The summary is AI-generated but you can edit it
- If you delete the hidden markers, the summary becomes yours (see Idempotent Operations)
- Great for long technical documents

---

### `/enhance-title` - Improve Document Title

**What it does:** Suggests and applies a better title if yours is too vague.

**When to use it:**
- Your title is generic like "Draft", "Notes", or "Untitled"
- You want a descriptive title but aren't sure what to use
- You've written content but forgot to update the placeholder title

**How to use it:**

Add the command:
```markdown
/enhance-title
```

**Real Example:**

Your document is titled "Notes" but contains detailed content about customer onboarding:

**Before:**
- Title: "Notes"
- Content: [Details about customer onboarding process, steps, checklists...]

**After:**
- Title: "Customer Onboarding Process and Checklist"
- A comment explains: "Title enhanced (confidence: 88%)"

**What happens after:**
- Your document has a clear, descriptive title
- The command is removed
- Other team members can find your document more easily

**Tips and best practices:**
- The AI only changes very vague titles (it won't change good titles)
- If confidence is low (< 70%), the title won't change
- You can always rename manually if you don't like the suggestion

---

### `/related` - Find Related Documents

**What it does:** Finds documents similar to yours and adds links to them.

**When to use it:**
- You want to help readers discover connected content
- Your document references concepts covered elsewhere
- You're building a knowledge hub

**How to use it:**

Add the command:
```markdown
/related
```

**Real Example:**

Your document is about "API Authentication". The AI finds related docs:

**Result added to document:**
```markdown
---
**Related Documents:**
- [API Security Best Practices](outline://doc/xyz)
- [OAuth2 Implementation Guide](outline://doc/abc)
- [JWT Token Management](outline://doc/def)
```

**What happens after:**
- A "Related Documents" section appears
- Links point to semantically similar documents
- Readers can explore connected topics
- The command is removed

**Tips and best practices:**
- Run this after your document is substantially complete
- The AI uses content similarity, not just keywords
- You can manually add/remove links from the list

---

## Interactive Guidance Loop

### What Does `?ai-file` Mean?

When you see `?ai-file` in your document (with a question mark), it means the AI wasn't confident enough to file your document automatically. This is a GOOD thing - it means the AI is being careful!

**Why does this happen?**
- Your content could fit multiple collections equally well
- The document discusses multiple topics
- The context is ambiguous without more information

### How to Provide Better Guidance

When you see `?ai-file`, you'll also see a comment explaining the uncertainty:

> Unable to file with confidence. Uncertain between:
> - Engineering (API implementation details)
> - Product (mobile app features)
>
> To help me decide:
> - Edit this line to: /ai-file engineering focus
> - Or add a new line: /ai-file product documentation
>
> [AI Confidence: 55%]

**What to do:**

1. Read the comment to understand the uncertainty
2. Decide which collection is most appropriate
3. Change `?ai-file` to `/ai-file` with guidance:
   ```markdown
   /ai-file backend engineering
   ```
4. Save the document
5. The AI retries with your guidance and files successfully!

### Examples of Good Guidance

| Ambiguous Situation | Good Guidance |
|---------------------|---------------|
| API documentation could be engineering or product | `/ai-file technical implementation` |
| Customer document could be support or marketing | `/ai-file customer success team` |
| Process doc could be operations or engineering | `/ai-file operations procedures` |
| Meeting notes covering multiple teams | `/ai-file engineering team notes` |

**Guidance principles:**
- Be specific but brief (3-5 words is perfect)
- Focus on the PRIMARY purpose of the document
- Think about who will use this document most
- You can be explicit: "engineering", "customer-facing", "internal"

### How to Resolve Uncertain Filing

**Step-by-step example:**

1. **You create a doc:** "Mobile App API Integration Guide"
2. **You add:** `/ai-file`
3. **AI responds:** Changes it to `?ai-file` with this comment:
   > Uncertain between:
   > - Engineering (API implementation)
   > - Product (mobile features)

4. **You decide:** This is for developers, so Engineering is better
5. **You update:** Change `?ai-file` to `/ai-file backend developer guide`
6. **AI retries:**
   - Confidence: 92%
   - Files to Engineering
   - Removes both `?ai-file` and `/ai-file` markers
   - Comments: "Filed to Engineering (confidence: 92%) - Thank you for the guidance!"

**The result:** Your document is in the right place, and you helped train the AI to understand your organization better!

---

## Idempotent Operations

### What Does "Idempotent" Mean?

In simple terms: **you can run the same command multiple times safely**. The AI won't create duplicates or make a mess.

### Running `/summarize` Multiple Times

**Scenario:** You have a document with a summary, you update the content, and run `/summarize` again.

**What happens:**
1. **First run:** Summary is added at the top
   ```markdown
   > **Summary**: Original summary here...
   ```

2. **You edit your document:** Add more sections, update content

3. **Second run:** You add `/summarize` again

4. **Result:** The old summary is cleanly replaced (not duplicated!)
   ```markdown
   > **Summary**: Updated summary reflecting new content...
   ```

**No duplicate summaries!** The AI tracks what it generated using hidden HTML comment markers.

### How It Works (Behind the Scenes)

When the AI generates content, it adds invisible markers:
```markdown
<!-- AI-SUMMARY-START -->
> **Summary**: Your summary here...
<!-- AI-SUMMARY-END -->
```

These markers:
- Are invisible when you view the document (they're HTML comments)
- Tell the AI "I generated this content"
- Allow clean replacement when you run the command again
- Let you take ownership by removing them

**The same applies to:**
- Summaries (top of document)
- Search terms (bottom of document)

### How to Edit AI-Generated Content

You have two options:

**Option 1: Let the AI keep managing it**
- Edit the summary text as much as you want
- Keep the hidden markers (you won't see them anyway)
- Next time you run `/summarize`, your edits will be replaced with a fresh summary
- Good for: Dynamic content that changes often

**Option 2: Take ownership**
- Switch to raw markdown view
- Find and delete the hidden markers: `<!-- AI-SUMMARY-START -->` and `<!-- AI-SUMMARY-END -->`
- Edit the summary however you like
- The AI won't touch it anymore
- Good for: Content you want permanent control over

### Example: Taking Ownership

Let's say the AI generated this summary:
```markdown
<!-- AI-SUMMARY-START -->
> **Summary**: This document covers database migrations.
<!-- AI-SUMMARY-END -->
```

**To take ownership:**
1. Switch to markdown edit mode
2. Remove the markers:
   ```markdown
   > **Summary**: This document covers database migrations with detailed
   > step-by-step procedures, rollback strategies, and troubleshooting guides.
   ```
3. The AI sees no markers and won't modify your summary anymore

**Why does this matter?** It gives you flexibility:
- Run commands multiple times safely (no duplicates)
- AI updates content when YOU want (on-demand)
- You can always take control when you need to

---

## Troubleshooting

### Command Not Working

**Symptom:** You added `/ai-file` but nothing happened after a minute.

**Possible causes:**
1. Command has a typo (check for extra spaces)
2. Command is inside a code block (move it outside)
3. AI service is temporarily unavailable
4. Document hasn't been saved yet

**How to fix:**
- Check spelling: `/ai-file` (not `/ai file` or `/aifile`)
- Make sure the command is on its own line
- Try refreshing the page and checking again
- Wait 1-2 minutes (sometimes AI takes a bit longer)
- Check for error comments on your document

---

### AI Taking Too Long

**Symptom:** Command has been there for 5+ minutes with no response.

**Possible causes:**
1. Your document is very long (AI needs more time)
2. Workspace has thousands of documents (search takes longer)
3. AI service is experiencing delays
4. Rate limits were hit

**How to fix:**
- Wait a bit longer (up to 5 minutes for long documents)
- Check the document for error comments
- If 10+ minutes pass, remove the command and try again
- Contact your Outline admin if persistent

---

### Wrong Classification

**Symptom:** `/ai-file` moved your document to the wrong collection.

**How to fix:**
1. Manually move the document to the correct collection
2. Next time, provide guidance: `/ai-file [specific area]`
3. Example: `/ai-file customer support documentation`

**Prevention:**
- Add guidance upfront if you suspect ambiguity
- Use 2-3 descriptive words about the document's purpose
- Think about who will USE the document most

---

### No Response

**Symptom:** Command disappeared but nothing happened (no comment, no action).

**Possible causes:**
1. AI determined no action was needed
2. Error occurred and was logged but not commented
3. Command was already processed

**How to check:**
- Look in document version history (AI may have processed it)
- Check if the document was already filed correctly
- For `/ai` questions: Check if a comment exists elsewhere
- For `/summarize`: Check if a summary is already at the top

---

### Multiple Commands Interfering

**Symptom:** Added multiple commands but only one worked.

**How it works:**
- Commands are processed in order
- Some commands may remove others (by design)
- `/ai-file` cleans up `?ai-file` markers

**Best practice:**
- Add one command at a time
- Wait for completion before adding another
- Don't mix `/ai-file` with `?ai-file` manually

---

### Who to Contact

If you're stuck:
1. **Check this guide first** - Most issues are covered here
2. **Ask in your team chat** - Others may have encountered the same thing
3. **Contact your Outline admin** - They can check logs and system status
4. **Create a support ticket** - For persistent or complex issues

Include in your support request:
- Command you used
- What you expected to happen
- What actually happened
- Document title (or ID if possible)
- Approximate timestamp

---

## FAQ

### Can I Use Multiple Commands in One Document?

**Yes!** You can use multiple commands, but add them one at a time:
1. Add first command (e.g., `/ai-file`)
2. Wait for it to complete
3. Add next command (e.g., `/summarize`)
4. Wait for completion

**Don't do this:**
```markdown
/ai-file
/summarize
/enhance-title
```
Add them sequentially instead.

---

### What Collections Can the AI File Documents To?

The AI can file to **any collection in your workspace** except:
- Excluded collections (configured by your admin)
- Personal/private collections (if configured)
- Archived collections

If you're unsure what collections exist:
- Check your workspace sidebar
- Ask your Outline admin
- The AI will comment with collection names when uncertain

---

### How Long Does Processing Take?

**Typical timings:**
- `/ai-file` filing: 2-5 seconds
- `/ai` questions: 3-8 seconds (depends on search)
- `/summarize`: 3-5 seconds
- `/enhance-title`: 2-3 seconds
- `/related`: 5-10 seconds (searches many documents)

**Longer times may occur if:**
- Your document is very long (10+ pages)
- Workspace has thousands of documents
- AI service is under load
- Complex questions require more search

---

### Can I Undo an Action?

**Yes, you have options:**

**For filing (`/ai-file`):**
- Use Outline's built-in "Move document" to relocate it
- The AI won't re-file unless you add the command again

**For summaries (`/summarize`):**
- Use document version history to see previous versions
- Manually edit or delete the summary
- Run `/summarize` again to regenerate

**For titles (`/enhance-title`):**
- Rename the document manually
- Use version history to see the previous title

**For all commands:**
- Document version history is your friend!
- Every AI change is tracked as a document update

---

### Is My Data Private?

**Data handling:**
- Your document content is sent to the AI service for analysis
- The AI service configured by your organization processes requests
- Check with your Outline admin about which AI service is used
- Most organizations use OpenAI, Azure OpenAI, or self-hosted models

**What gets sent:**
- Document title and content (for `/ai-file`, `/summarize`, etc.)
- Your question and workspace excerpts (for `/ai` questions)
- Nothing is sent unless you use a command

**What doesn't get sent:**
- Comments on documents
- Version history
- User information (except as context for the AI)

**For more details:** Contact your Outline admin about your organization's AI privacy policy.

---

### Can I Customize the AI Behavior?

**As a user:** Limited customization through guidance
- Provide specific guidance with `/ai-file [hint]`
- Phrase questions clearly for better `/ai` answers
- Edit AI-generated content as needed

**As an admin:** Extensive customization available
- Confidence thresholds
- Excluded collections
- AI model selection
- Enable/disable specific commands
- Custom prompts

Contact your admin if you need organization-wide changes.

---

### What If I Don't Want AI Help?

**You're in control!**
- Don't add commands - the AI won't do anything
- The AI only acts when you explicitly use a command
- You can always organize documents manually
- AI-generated content can be edited or deleted

**Mixed approach:**
- Use AI for routine organization (filing, summaries)
- Manually handle sensitive or complex documents
- Take ownership of AI content when needed

---

## Examples Gallery

### Example 1: Filing a Technical Document

**Scenario:** You wrote a detailed guide about PostgreSQL connection pooling.

**Your document:**
```markdown
# Database Connection Pooling Best Practices

/ai-file

This guide explains how to configure and optimize PostgreSQL connection
pooling in production environments...

[Several pages of technical content about connection pools, configurations,
performance tuning, monitoring, and troubleshooting...]
```

**What happens:**
1. You save the document
2. Within 3 seconds, the command disappears
3. Document is moved to "Engineering" collection
4. A comment appears:
   > Filed to Engineering (confidence: 95%)
   >
   > Reasoning: Technical database implementation content with code examples
   > and performance tuning details.

5. Search terms are added at the bottom:
   ```markdown
   ---
   **Search Terms**: PostgreSQL, connection pooling, database, performance,
   optimization, configuration
   ```

**Result:** Your technical doc is organized, searchable, and ready to help the team!

---

### Example 2: Asking a Question

**Scenario:** You're new to the team and need to understand the deployment process.

**Your document:**
```markdown
# Deployment Questions

/ai How do we deploy to production?

I'm trying to understand the deployment workflow for our main application.
```

**What happens:**
1. Command is detected
2. AI searches workspace: finds "Deployment Guide", "CI/CD Pipeline", "Production Checklist"
3. After 5 seconds, a comment appears:

> **AI Answer**
>
> The production deployment process uses a three-stage pipeline:
>
> 1. **Staging Deployment**: Code is pushed to staging environment automatically
>    via CI/CD when merging to main branch
> 2. **Automated Testing**: Integration tests and smoke tests run on staging
> 3. **Production Release**: After approval, a release tag triggers production
>    deployment through the release pipeline
>
> Key requirements:
> - All tests must pass in staging
> - Two team members must approve the release
> - Deployments only during maintenance windows (Tue/Thu 10am-12pm)
>
> **Sources:**
> - [Deployment Guide](outline://doc/deploy123)
> - [CI/CD Pipeline Setup](outline://doc/cicd456)
> - [Production Deployment Checklist](outline://doc/prod789)
>
> *Confidence: 94%*

4. The `/ai` command is removed
5. You have a clear answer with sources to read more

**Result:** You learned the deployment process in seconds instead of hunting through multiple docs!

---

### Example 3: Generating a Summary

**Scenario:** You completed a lengthy requirements document and want an executive summary.

**Your document (before):**
```markdown
# Customer Portal Requirements

[5 pages of detailed requirements, user stories, technical specs,
acceptance criteria, mockups, etc...]

/summarize
```

**What happens:**
1. AI reads entire document
2. Generates a concise summary
3. Places it at the top

**Your document (after):**
```markdown
# Customer Portal Requirements

> **Summary**: This document defines requirements for the new customer portal,
> including user authentication, dashboard views, and account management features.
> It covers both functional requirements and non-functional requirements such as
> performance targets and security considerations.

[5 pages of detailed requirements...]
```

4. The `/summarize` command is removed
5. Readers now see the big picture before diving into details

**Result:** Stakeholders can quickly understand the document's purpose!

---

### Example 4: Handling Ambiguous Documents

**Scenario:** You wrote notes from a meeting that covered both product features and technical implementation.

**Your document:**
```markdown
# API v2 Planning Meeting Notes

/ai-file

Discussed new API features for mobile app:
- Push notifications endpoint
- User preferences API
- Performance improvements needed
- Mobile team wants better error messages

Technical considerations:
- Need to update backend services
- Database schema changes required
- Estimated 2 sprint implementation
```

**What happens:**
1. AI analyzes the content
2. Confidence is low (58%) - could be Product OR Engineering
3. Command changes to `?ai-file`
4. Comment appears:

> Unable to file with confidence (58%)
>
> Uncertain between:
> - Product (mobile app features, user-facing functionality)
> - Engineering (backend implementation, database changes)
>
> To help me decide:
> - If these are product requirements: `/ai-file product features`
> - If these are technical implementation notes: `/ai-file engineering planning`
>
> [AI Confidence: 58%]

**You decide:** This is primarily for the engineering team's planning.

**You update:**
```markdown
# API v2 Planning Meeting Notes

/ai-file engineering team planning notes

[rest of content...]
```

**AI retries:**
1. Confidence jumps to 89%
2. Files to Engineering collection
3. Removes both markers
4. Comments: "Filed to Engineering (confidence: 89%) - Thank you for the guidance!"

**Result:** Document is in the right place thanks to your input!

---

### Example 5: Using Guidance Effectively

**Scenario:** You're documenting an API, and you know it could be ambiguous.

**Bad approach (no guidance):**
```markdown
# Payment API Documentation

/ai-file

[API documentation content...]
```
Result: AI might be uncertain between Engineering, Product, or Financial

**Good approach (with guidance):**
```markdown
# Payment API Documentation

/ai-file technical implementation for developers

[API documentation content...]
```
Result: AI files to Engineering with high confidence (91%)

**Another good approach (different intent):**
```markdown
# Payment API Documentation

/ai-file customer-facing integration guide

[API documentation content...]
```
Result: AI files to Product/Documentation with high confidence (88%)

**Key lesson:** Same document, different guidance, different outcomes! Think about:
- WHO will use this document?
- WHAT is its primary purpose?
- WHERE do similar docs live?

---

### Example 6: Iterative Summary Improvement

**Scenario:** Your document evolves over time, and you want the summary to stay current.

**Version 1 - Initial draft:**
```markdown
# Security Incident Response

/summarize

[2 pages about initial incident response procedures...]
```

**Result:** Summary added
> **Summary**: This document outlines initial response procedures for security
> incidents, including detection, escalation, and immediate mitigation steps.

**Version 2 - You add more content:**
```markdown
# Security Incident Response

> **Summary**: [old summary...]

[2 pages of original content...]
[NEW: 3 pages about post-incident analysis, lessons learned, prevention...]

/summarize
```

**Result:** Summary is cleanly replaced (not duplicated!)
> **Summary**: This document provides comprehensive security incident response
> procedures covering detection, escalation, mitigation, post-incident analysis,
> and preventive measures to avoid future incidents.

**Version 3 - More additions:**
```markdown
# Security Incident Response

> **Summary**: [previous summary...]

[All previous content...]
[NEW: Communication templates, stakeholder notification procedures...]

/summarize
```

**Result:** Summary updated again
> **Summary**: This document serves as the complete guide for security incident
> response, including procedures for detection, mitigation, analysis, communication
> with stakeholders, and preventive measures.

**Key lesson:** Run `/summarize` whenever you make major updates. The AI keeps the summary fresh without creating duplicates!

---

## Quick Reference Card

| Command | Purpose | Example |
|---------|---------|---------|
| `/ai-file` | File document to correct collection | `/ai-file` or `/ai-file engineering docs` |
| `/ai` | Ask a question | `/ai What is our coding standard?` |
| `/summarize` | Generate/update summary | `/summarize` |
| `/enhance-title` | Improve vague title | `/enhance-title` |
| `/related` | Find related documents | `/related` |
| `?ai-file` | Uncertain filing marker | (AI adds this, you update to `/ai-file [guidance]`) |

**Remember:**
- Commands work on their own lines
- One command at a time works best
- Guidance helps with ambiguous content
- Running commands multiple times is safe
- You can always edit AI-generated content
- The AI only acts when you use commands

---

## Getting Help

**This guide covers most common scenarios.** For additional help:

1. Check the [FAQ section](#faq)
2. Search for similar issues in your team chat
3. Contact your Outline workspace administrator
4. Review the examples gallery for similar use cases

**Happy organizing!** The AI Assistant is here to make your Outline experience smoother and more efficient.

---

*Last updated: 2026-01-19*
*Version: 1.0*
