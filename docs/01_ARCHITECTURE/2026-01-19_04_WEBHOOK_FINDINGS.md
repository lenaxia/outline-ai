# Outline Webhooks - Findings from Source Code Analysis

## Summary
✅ Outline **fully supports** webhooks with `documents.update` events, making real-time command/question detection possible without polling.

## Key Discoveries

### 1. Available Document Events
From `shared/utils/EventHelper.ts`, the complete list of document-related AUDIT_EVENTS:

```typescript
"documents.create",      // New document created
"documents.publish",     // Document published
"documents.update",      // ✅ Document edited - THIS IS KEY!
"documents.archive",     // Document archived
"documents.unarchive",   // Document unarchived
"documents.move",        // Document moved to different collection
"documents.delete",      // Document soft-deleted
"documents.permanent_delete", // Document permanently deleted
"documents.restore",     // Document restored from trash
"documents.add_user",    // User added to document
"documents.remove_user", // User removed from document
```

### 2. Webhook Subscription Mechanism
From `plugins/webhooks/server/processors/WebhookProcessor.ts`:

- Webhooks listen to **ALL events** (`["*"]`)
- Individual subscriptions filter events using `validForEvent()` method
- Can subscribe to:
  - **Specific events**: `["documents.update", "documents.create"]`
  - **Event patterns**: `["documents."]` matches all document events
  - **All events**: `["*"]` (not recommended - too noisy)

### 3. Event Payload Structure
From webhook documentation and source code:

```json
{
  "id": "uuid-delivery-id",
  "webhookSubscriptionId": "uuid-subscription-id",
  "createdAt": "2024-01-19T12:00:00.000Z",
  "event": "documents.update",
  "actorId": "uuid-user-id",
  "model": {
    "id": "document-uuid",
    "title": "Document Title",
    "text": "Full document content...",
    // ... other document properties
  }
}
```

**Key Fields for Our Use Case:**
- `event`: Event type (filter for `documents.update`)
- `model.id`: Document ID
- `model.text`: Full document content (check for commands/questions)
- `actorId`: User who made the change

### 4. Security Requirements
From webhook documentation:

- **Signature Validation**: `Outline-Signature` header contains SHA-256 HMAC
  - Calculated as: `HMAC-SHA256(request_body, webhook_secret)`
  - Must validate before processing to ensure authenticity
- **Response Timeout**: Must respond with HTTP 200 within 5 seconds
- **Long Operations**: Use background jobs, return 200 immediately

### 5. Reliability Features
From webhook documentation:

- **Retries**: Failed deliveries retry with exponential backoff
- **Auto-disable**: After 25 consecutive failures, webhook auto-disables
- **Notifications**: Webhook creator receives email on failures
- **Delivery Log**: Track webhook delivery attempts

## Implementation Strategy

### Recommended Approach
1. **Primary**: Webhook receiver on `/webhooks` endpoint
   - Subscribe to `["documents.update", "documents.create"]`
   - Fast, real-time response (< 1 second after user edit)
   - Minimal API usage (only when documents change)

2. **Fallback**: Polling mode (optional)
   - For local development (no public URL)
   - If webhooks disabled or failing
   - Poll every 60s (much less frequent than without webhooks)

### Webhook Setup Process
```bash
# Using Outline API to create webhook subscription
POST https://app.getoutline.com/api/webhookSubscriptions.create
{
  "name": "Outline AI Assistant",
  "url": "https://your-service.com/webhooks",
  "secret": "your-generated-secret",
  "events": ["documents.update", "documents.create"]
}
```

### Event Processing Flow
```
1. Outline sends POST to /webhooks
2. Validate Outline-Signature header
3. Parse event payload
4. Check event type (documents.update?)
5. Extract document content from model.text
6. Scan for command markers (/ai-file, /ai, ?!?)
7. If found, queue for worker pool
8. Return HTTP 200 (within 5 seconds)
9. Worker processes asynchronously
```

## Performance Benefits

### Webhook vs Polling Comparison

**Polling (30s interval):**
- API calls per hour: 120 (regardless of activity)
- Response time: 0-30 seconds (average 15s)
- Cost: High (constant API usage)

**Webhooks:**
- API calls per hour: Only when documents edited (1-10 typically)
- Response time: < 1 second
- Cost: Minimal (only pay for document fetches if needed)

**Improvement:**
- 99% reduction in API calls
- 15x faster response time
- Near-zero cost for idle periods

## Code References

All findings from the Outline repository at `~/personal/temp/outline`:

1. **Event definitions**: `shared/utils/EventHelper.ts` (line 40)
2. **Webhook processor**: `plugins/webhooks/server/processors/WebhookProcessor.ts`
3. **Webhook model**: `server/models/WebhookSubscription.ts`
4. **Event filtering**: `WebhookSubscription.validForEvent()` method
5. **Webhook API**: `plugins/webhooks/server/api/webhookSubscriptions.ts`

## Next Steps for Implementation

1. **Setup Phase**:
   - Generate webhook secret
   - Configure public URL for webhook endpoint
   - Create webhook subscription via Outline API
   - Test signature validation

2. **Development Phase**:
   - Implement `/webhooks` POST handler
   - Add signature validation middleware
   - Parse and filter events
   - Queue for worker pool
   - Ensure < 5 second response

3. **Monitoring Phase**:
   - Track webhook delivery success rate
   - Log signature validation failures
   - Monitor for auto-disable notifications
   - Implement fallback to polling if needed

4. **Local Development**:
   - Use ngrok or similar for local webhook testing
   - OR disable webhooks and use polling mode
   - Test signature validation with known secrets

## Conclusion

Outline's webhook system is **production-ready** and perfectly suited for our use case. The `documents.update` event provides exactly what we need to detect commands and questions in real-time, with minimal API overhead and sub-second response times.

**Recommendation**: Implement webhooks as primary method, with optional polling fallback for local development only.
