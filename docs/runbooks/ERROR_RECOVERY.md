# Error Recovery Runbook

**Document Type:** Operational Runbook
**Last Updated:** 2026-01-19
**Audience:** Operations, DevOps, Support Engineers

## Table of Contents

1. [Overview](#overview)
2. [Common Failure Scenarios](#common-failure-scenarios)
3. [Worker Pool Failures](#worker-pool-failures)
4. [Command System Failures](#command-system-failures)
5. [Webhook System Failures](#webhook-system-failures)
6. [Database Issues](#database-issues)
7. [Monitoring and Alerts](#monitoring-and-alerts)
8. [Prevention Strategies](#prevention-strategies)

## Overview

This runbook provides step-by-step procedures for recovering from common error scenarios in the Outline AI Assistant. Each section includes:

- **Symptoms**: How to identify the issue
- **Diagnosis**: SQL queries and log checks
- **Recovery**: Step-by-step recovery procedures
- **Prevention**: How to avoid the issue in the future

### Quick Reference: Severity Levels

- **P0 (Critical)**: Service completely down or data loss imminent
- **P1 (High)**: Major functionality broken, affecting multiple users
- **P2 (Medium)**: Isolated failures, workarounds available
- **P3 (Low)**: Minor issues, no immediate impact

## Common Failure Scenarios

### Scenario 1: Service Won't Start After Downtime

**Severity:** P0
**Symptoms:**
- Service fails to start with database lock errors
- Log shows "database is locked" or "unable to open database file"

**Diagnosis:**

```bash
# Check if SQLite database is locked
lsof /path/to/outline-ai.db

# Check database integrity
sqlite3 /path/to/outline-ai.db "PRAGMA integrity_check;"

# Check for WAL files
ls -la /path/to/outline-ai.db*
```

**Recovery Steps:**

1. **Stop all instances of the service**
```bash
# Find all processes
ps aux | grep outline-ai

# Kill any hung processes
sudo killall -9 outline-ai
```

2. **Checkpoint the WAL and release locks**
```bash
sqlite3 /path/to/outline-ai.db "PRAGMA wal_checkpoint(TRUNCATE);"
```

3. **Verify database integrity**
```bash
sqlite3 /path/to/outline-ai.db "PRAGMA integrity_check;"
# Should return: ok
```

4. **Restart the service**
```bash
sudo systemctl restart outline-ai
# or
./outline-ai
```

5. **Verify startup**
```bash
# Check health endpoint
curl http://localhost:8080/health

# Check logs
tail -f /var/log/outline-ai/service.log
```

**Prevention:**
- Implement proper graceful shutdown handling
- Use WAL mode with proper checkpoint intervals
- Set connection pool size to 1 for SQLite (already configured)

---

### Scenario 2: Worker Pool Exhausted / All Workers Stuck

**Severity:** P1
**Symptoms:**
- Health check shows no active workers
- Queue is full but nothing is processing
- Log shows workers haven't sent heartbeat in > 5 minutes

**Diagnosis:**

```sql
-- Check dead letter queue for stuck tasks
SELECT
    task_type,
    document_id,
    failure_reason,
    attempt_count,
    last_failure
FROM dead_letter_queue
WHERE last_failure > datetime('now', '-1 hour')
ORDER BY last_failure DESC
LIMIT 20;

-- Check for orphaned task checkpoints
SELECT
    task_id,
    document_id,
    created_at,
    updated_at
FROM task_checkpoints
WHERE updated_at < datetime('now', '-30 minutes')
ORDER BY updated_at;
```

**Check Logs:**
```bash
# Look for worker stall warnings
grep "Worker appears to be stalled" /var/log/outline-ai/service.log | tail -20

# Check for long-running tasks
grep "Executing task" /var/log/outline-ai/service.log | \
  awk '{print $1, $2, $NF}' | \
  sort | uniq -c | sort -rn | head -20
```

**Recovery Steps:**

1. **Immediate: Restart the service** (forces worker pool restart)
```bash
sudo systemctl restart outline-ai
```

2. **Clear orphaned task state**
```sql
-- Delete checkpoints older than 1 hour
DELETE FROM task_checkpoints
WHERE updated_at < datetime('now', '-1 hour');

-- Verify deletion
SELECT COUNT(*) FROM task_checkpoints;
```

3. **Retry failed tasks from dead letter queue**

Using the management API (if implemented):
```bash
# List failed tasks
curl http://localhost:8080/admin/dlq/list

# Retry specific task
curl -X POST http://localhost:8080/admin/dlq/retry/123
```

Or using SQL:
```sql
-- Get tasks from DLQ
SELECT id, task_id, document_id, task_type
FROM dead_letter_queue
WHERE last_failure > datetime('now', '-24 hours')
LIMIT 10;

-- To manually reprocess, use the manual recovery tool
-- (requires running outline-ai-admin CLI tool)
```

4. **Monitor recovery**
```bash
# Watch logs for task processing
tail -f /var/log/outline-ai/service.log | grep "Task completed"

# Check health endpoint
watch -n 5 'curl -s http://localhost:8080/health | jq .'
```

**Prevention:**
- Implement worker health monitoring (already in design)
- Set task timeouts (5 minutes per task)
- Add alerting on high DLQ size
- Configure proper worker count for load (default: 3)

---

### Scenario 3: Command Markers Not Being Removed

**Severity:** P2
**Symptoms:**
- Documents have command markers (/ai-file, /ai, etc.) that aren't being processed
- Same commands are being processed repeatedly
- Users report commands "not working"

**Diagnosis:**

```bash
# Check if webhooks are being received
curl http://localhost:8080/health | jq '.webhook_stats'

# Check command processing logs
grep "Command detected" /var/log/outline-ai/service.log | tail -20
grep "Command marker removed" /var/log/outline-ai/service.log | tail -20

# Check for partial failures
grep "Failed to remove command marker" /var/log/outline-ai/service.log | tail -20
```

```sql
-- Check command state table
SELECT
    command_type,
    status,
    COUNT(*) as count,
    MAX(last_attempt) as last_attempt
FROM command_state
GROUP BY command_type, status
ORDER BY last_attempt DESC;

-- Find stuck commands (processing for > 30 minutes)
SELECT
    command_id,
    document_id,
    command_type,
    status,
    attempt_count,
    last_attempt,
    last_error
FROM command_state
WHERE status = 'processing'
  AND last_attempt < datetime('now', '-30 minutes')
ORDER BY last_attempt;
```

**Recovery Steps:**

1. **Identify stuck documents**
```sql
-- Get document IDs with stuck commands
SELECT document_id, command_type, last_attempt, last_error
FROM command_state
WHERE status = 'processing'
  AND last_attempt < datetime('now', '-30 minutes');
```

2. **Manually inspect documents in Outline**
- Open each document in Outline UI
- Check if command marker is still present
- Verify document is accessible and not deleted

3. **Clean up stale command state**
```sql
-- Mark old processing commands as failed
UPDATE command_state
SET status = 'failed',
    last_error = 'Timeout - marked as failed by admin',
    updated_at = CURRENT_TIMESTAMP
WHERE status = 'processing'
  AND last_attempt < datetime('now', '-1 hour');

-- Verify update
SELECT COUNT(*) FROM command_state WHERE status = 'processing';
```

4. **Trigger manual reprocessing**

Option A: Update document in Outline (triggers webhook):
- Remove the command marker
- Re-add the command marker
- This triggers a new webhook event

Option B: Use admin tool (if implemented):
```bash
# Reprocess specific document
curl -X POST http://localhost:8080/admin/commands/reprocess \
  -H "Content-Type: application/json" \
  -d '{"document_id": "abc123"}'
```

Option C: Manually remove markers:
- Edit document in Outline
- Remove command marker manually
- Add a comment explaining the issue

**Prevention:**
- Implement command timeout cleanup (runs every hour)
- Add alerting on stuck commands
- Ensure dual marker cleanup (/ai-file + ?ai-file)
- Log marker removal failures more visibly

---

### Scenario 4: Webhook Events Being Dropped

**Severity:** P1
**Symptoms:**
- Commands added to documents aren't being processed
- No recent events in webhook stats
- Queue overflow errors in logs

**Diagnosis:**

```bash
# Check webhook receiver health
curl http://localhost:8080/health | jq '.webhook_stats'

# Expected output:
# {
#   "total_received": 1234,
#   "valid_signatures": 1234,
#   "invalid_signatures": 0,
#   "processed_successfully": 1200,
#   "processing_failed": 34,
#   "last_event_time": "2026-01-19T10:30:00Z",
#   "queue_size": 5,
#   "queue_capacity": 1000
# }

# Check for queue overflow
grep "Event queue overflow" /var/log/outline-ai/service.log | wc -l

# Check for webhook delivery failures in Outline
# (requires access to Outline admin panel)
```

```sql
-- Check overflow events table
SELECT
    event_type,
    model_id,
    stored_at
FROM overflow_events
ORDER BY stored_at DESC
LIMIT 20;

-- Count overflow events
SELECT COUNT(*) as overflow_count
FROM overflow_events;

-- Check failed webhook events
SELECT
    event_type,
    model_id,
    failure_reason,
    retry_count,
    next_retry,
    strategy
FROM failed_webhook_events
WHERE strategy != 'skip'
ORDER BY last_failure DESC
LIMIT 20;
```

**Recovery Steps:**

1. **Check webhook configuration in Outline**
- Log into Outline admin panel
- Navigate to Settings → Webhooks
- Verify webhook URL is correct
- Check webhook status (should be "Active", not "Disabled")
- Check delivery statistics

2. **Process overflow events from database**
```bash
# Trigger overflow event processing
# (this runs automatically every 5 minutes, but can be forced)

# Check if overflow processor is running
grep "Processing overflow events from database" /var/log/outline-ai/service.log | tail -5
```

3. **If webhook is disabled in Outline**

Webhooks auto-disable after 25 consecutive failures. To re-enable:

a. Fix the root cause (service was down, URL changed, etc.)
b. In Outline admin: Delete old webhook, create new one
c. Update webhook secret in config
d. Restart service with new config

4. **Perform catch-up for missed events**
```bash
# Trigger manual catch-up (if implemented)
curl -X POST http://localhost:8080/admin/catchup/run

# This will:
# - Scan for documents with command markers
# - Process any missed commands
# - Update catch-up state
```

```sql
-- Check catch-up state
SELECT
    last_processed_time,
    last_catchup_duration_ms,
    documents_processed,
    updated_at
FROM catchup_state;

-- If catch-up state is stale, reset it
UPDATE catchup_state
SET last_processed_time = datetime('now', '-1 hour'),
    updated_at = CURRENT_TIMESTAMP
WHERE id = 1;
```

5. **Monitor recovery**
```bash
# Watch for new webhook events
tail -f /var/log/outline-ai/service.log | grep "Received webhook event"

# Check queue size
watch -n 2 'curl -s http://localhost:8080/health | jq .webhook_stats.queue_size'
```

**Prevention:**
- Increase queue size if overflow is frequent (default: 1000)
- Add alerting on queue utilization > 80%
- Implement catch-up on startup (already in design)
- Monitor webhook status in Outline

---

## Worker Pool Failures

### Task Retry Exhaustion

**Symptoms:**
- High number of entries in dead_letter_queue table
- Same document IDs appearing in DLQ repeatedly
- Alerts on DLQ size > 50

**SQL Diagnostics:**

```sql
-- View dead letter queue summary
SELECT
    task_type,
    COUNT(*) as count,
    AVG(attempt_count) as avg_attempts,
    MAX(last_failure) as most_recent
FROM dead_letter_queue
GROUP BY task_type
ORDER BY count DESC;

-- Find documents with multiple DLQ entries
SELECT
    document_id,
    COUNT(*) as failure_count,
    GROUP_CONCAT(task_type) as failed_task_types,
    MAX(last_failure) as last_failure
FROM dead_letter_queue
GROUP BY document_id
HAVING COUNT(*) > 1
ORDER BY failure_count DESC;

-- Examine specific DLQ entries
SELECT
    id,
    task_id,
    task_type,
    document_id,
    failure_reason,
    attempt_count,
    error_details,
    checkpoint
FROM dead_letter_queue
WHERE document_id = 'doc_abc123';
```

**Recovery Procedure:**

1. **Analyze failure patterns**
```sql
-- Get failure reason frequency
SELECT
    CASE
        WHEN failure_reason LIKE '%timeout%' THEN 'timeout'
        WHEN failure_reason LIKE '%auth%' THEN 'auth'
        WHEN failure_reason LIKE '%not found%' THEN 'not_found'
        WHEN failure_reason LIKE '%rate limit%' THEN 'rate_limit'
        ELSE 'other'
    END as failure_category,
    COUNT(*) as count
FROM dead_letter_queue
GROUP BY failure_category
ORDER BY count DESC;
```

2. **Fix underlying issues** (based on analysis)

- **Timeouts**: Increase task timeout or reduce document size
- **Auth errors**: Check API key configuration
- **Not found**: Documents may have been deleted, skip these
- **Rate limits**: Reduce worker count or increase rate limit

3. **Retry recoverable tasks**

```sql
-- Get recoverable tasks (not auth/not-found errors)
SELECT id, task_id, document_id, task_type
FROM dead_letter_queue
WHERE failure_reason NOT LIKE '%auth%'
  AND failure_reason NOT LIKE '%not found%'
  AND failure_reason NOT LIKE '%404%'
ORDER BY last_failure DESC;

-- Mark for retry (reset to overflow_events for reprocessing)
-- This requires moving data between tables - use admin tool instead
```

Using admin tool:
```bash
# List DLQ entries
./outline-ai-admin dlq list --limit 50

# Retry specific entry
./outline-ai-admin dlq retry --id 123

# Retry all with specific failure reason
./outline-ai-admin dlq retry-pattern --pattern "timeout"

# Skip non-recoverable entries
./outline-ai-admin dlq skip --id 456 --reason "Document deleted"
```

4. **Clean up old DLQ entries**
```sql
-- Archive old DLQ entries (> 7 days)
CREATE TABLE IF NOT EXISTS dead_letter_archive AS
SELECT * FROM dead_letter_queue
WHERE last_failure < datetime('now', '-7 days');

-- Delete archived entries from DLQ
DELETE FROM dead_letter_queue
WHERE last_failure < datetime('now', '-7 days');

-- Verify
SELECT COUNT(*) as remaining_dlq FROM dead_letter_queue;
SELECT COUNT(*) as archived FROM dead_letter_archive;
```

**Prevention:**
- Set appropriate retry limits (default: 3)
- Implement error classification (transient vs permanent)
- Alert on DLQ size > 50 entries
- Regular DLQ review (weekly)

---

### Partial Task Failures

**Symptoms:**
- Document updated but not moved to collection
- Search terms added but markers not removed
- Inconsistent document state

**SQL Diagnostics:**

```sql
-- Check task checkpoints (indicates partial completion)
SELECT
    task_id,
    document_id,
    document_updated,
    search_terms_added,
    document_moved,
    markers_removed,
    comment_posted,
    updated_at
FROM task_checkpoints
ORDER BY updated_at DESC
LIMIT 20;

-- Find checkpoints with partial completion
SELECT
    task_id,
    document_id,
    CASE
        WHEN document_updated AND NOT document_moved THEN 'updated_not_moved'
        WHEN document_moved AND NOT markers_removed THEN 'moved_markers_remain'
        WHEN markers_removed AND NOT comment_posted THEN 'markers_removed_no_comment'
        ELSE 'unknown'
    END as partial_state,
    updated_at
FROM task_checkpoints
WHERE NOT (document_updated AND search_terms_added AND document_moved AND markers_removed AND comment_posted)
ORDER BY updated_at DESC;
```

**Recovery Procedure:**

1. **Identify documents with partial state**
```sql
-- Get document IDs needing recovery
SELECT document_id, task_id, updated_at
FROM task_checkpoints
WHERE updated_at > datetime('now', '-24 hours')
  AND NOT document_moved;  -- Or other incomplete condition
```

2. **Manual recovery for each document**

For each document ID:

a. **Check document in Outline** (via UI or API)
```bash
# Get current document state
curl -H "Authorization: Bearer $OUTLINE_API_KEY" \
  https://app.getoutline.com/api/documents.info \
  -d "id=doc_abc123" | jq .
```

b. **Complete the remaining steps manually**

Example: Document updated but not moved
```bash
# Move document to collection
curl -X POST \
  -H "Authorization: Bearer $OUTLINE_API_KEY" \
  https://app.getoutline.com/api/documents.move \
  -d "id=doc_abc123" \
  -d "collectionId=collection_xyz"

# Remove command marker (edit document text)
# Update document via API or Outline UI
```

c. **Clean up checkpoint**
```sql
-- Delete checkpoint after manual recovery
DELETE FROM task_checkpoints
WHERE document_id = 'doc_abc123';
```

3. **Batch recovery for similar issues**
```sql
-- Get all documents with same partial state
SELECT document_id, task_id
FROM task_checkpoints
WHERE document_updated = 1
  AND document_moved = 0;

-- Use admin tool to retry from checkpoint
```

**Prevention:**
- Implement checkpoint-based recovery (already in design)
- Make non-critical steps optional (comment posting)
- Add compensation logic for rollback
- Monitor partial completion rates

---

## Command System Failures

### Dual Marker Scenarios

**Symptoms:**
- Documents have both /ai-file and ?ai-file markers
- Commands processed multiple times
- Conflicting comments on document

**SQL Diagnostics:**

```sql
-- Find commands with multiple attempts
SELECT
    document_id,
    command_type,
    COUNT(*) as attempts,
    MAX(last_attempt) as last_attempt,
    GROUP_CONCAT(status) as statuses
FROM command_state
GROUP BY document_id, command_type
HAVING COUNT(*) > 1
ORDER BY last_attempt DESC;
```

**Manual Inspection:**

For each document with dual markers:

1. Open document in Outline UI
2. Search for both markers:
   - `/ai-file` (active command)
   - `?ai-file` (uncertain marker)
3. Review comments to understand history

**Recovery Procedure:**

1. **If command succeeded** (document is in correct collection):
```bash
# Edit document, remove BOTH markers
# Can be done via Outline UI or API
```

2. **If command failed** (document still needs filing):
```bash
# Keep /ai-file, remove ?ai-file
# Or provide guidance: /ai-file [guidance]
```

3. **Clean up command state**
```sql
-- Remove old command state entries
DELETE FROM command_state
WHERE document_id = 'doc_abc123'
  AND status IN ('completed', 'failed')
  AND last_attempt < datetime('now', '-24 hours');
```

**Prevention:**
- Implement dual marker cleanup (already in design)
- Single transaction for marker removal
- Idempotent command processing

---

### Comment Posting Failures

**Symptoms:**
- Command executed but no confirmation comment
- Users don't see AI responses
- Silent failures in logs

**SQL Diagnostics:**

```sql
-- Check for failed comments (if tracked)
SELECT
    document_id,
    content,
    timestamp,
    retries
FROM failed_comments
WHERE timestamp > datetime('now', '-24 hours')
ORDER BY timestamp DESC;
```

**Log Analysis:**
```bash
# Find comment posting failures
grep "Failed to post.*comment" /var/log/outline-ai/service.log | tail -20

# Check for auth errors
grep "comment" /var/log/outline-ai/service.log | grep "401\|403" | tail -10
```

**Recovery Procedure:**

1. **Verify API permissions**
```bash
# Test comment creation
curl -X POST \
  -H "Authorization: Bearer $OUTLINE_API_KEY" \
  https://app.getoutline.com/api/comments.create \
  -d "documentId=test_doc" \
  -d "data={\"type\":\"doc\",\"content\":[{\"type\":\"paragraph\",\"content\":[{\"type\":\"text\",\"text\":\"Test comment\"}]}]}"
```

2. **If auth issue**: Update API key in config and restart

3. **Retry failed comments**
```sql
-- Get failed comments to retry
SELECT id, document_id, content
FROM failed_comments
WHERE retries < 3
ORDER BY timestamp;
```

For each failed comment:
```bash
# Use admin tool or manually post via Outline UI
./outline-ai-admin comments retry --id 123
```

4. **Document which tasks lack comments**
```sql
-- Find completed commands without comments
-- (This query assumes comment posting is tracked)
SELECT
    c.document_id,
    c.command_type,
    c.last_attempt
FROM command_state c
LEFT JOIN failed_comments fc ON c.document_id = fc.document_id
WHERE c.status = 'completed'
  AND fc.id IS NULL
  AND c.last_attempt > datetime('now', '-24 hours');
```

**Prevention:**
- Make comment posting optional (don't fail command)
- Implement retry logic for comments (already in design)
- Store failed comments for background retry
- Alert on high comment failure rate

---

## Webhook System Failures

### Long Downtime Recovery (Days of Outage)

**Severity:** P0
**Symptoms:**
- Service was down for > 24 hours
- Many commands were added during downtime
- No webhook events received

**Recovery Procedure:**

1. **Assess downtime duration**
```sql
-- Check last processed time
SELECT
    last_processed_time,
    documents_processed,
    updated_at
FROM catchup_state;

-- Calculate downtime
SELECT
    julianday('now') - julianday(last_processed_time) as downtime_days,
    datetime('now') as current_time,
    last_processed_time
FROM catchup_state;
```

2. **Start the service** (with catch-up enabled)
```bash
# Ensure catch-up is enabled in config
cat config.yaml | grep -A 5 fallback_polling

# Start service
sudo systemctl start outline-ai

# Monitor catch-up progress
tail -f /var/log/outline-ai/service.log | grep -E "catch-up|Searching for command marker"
```

3. **Monitor catch-up progress**
```bash
# Check catch-up status
curl http://localhost:8080/admin/catchup/status | jq .

# Example output:
# {
#   "in_progress": true,
#   "started_at": "2026-01-19T10:00:00Z",
#   "markers_checked": 6,
#   "documents_found": 45,
#   "documents_processed": 23,
#   "errors": 2
# }
```

4. **Verify catch-up completion**
```sql
-- Check updated catch-up state
SELECT
    last_processed_time,
    documents_processed,
    last_catchup_duration_ms,
    updated_at
FROM catchup_state;

-- Should show last_processed_time close to current time
```

5. **Verify documents were processed**

Sample check:
```bash
# Search for remaining command markers in Outline
# (via UI or API)
# Should find fewer unprocessed commands
```

6. **Handle edge cases manually**
```sql
-- Find documents that might have been missed
-- (This requires cross-referencing with Outline)
SELECT document_id, command_type, last_attempt
FROM command_state
WHERE status != 'completed'
  AND last_attempt < (SELECT last_processed_time FROM catchup_state);
```

**Prevention:**
- Enable catch-up on startup (default: enabled)
- Regular service health monitoring
- Redundant deployment (if critical)
- Alert on service down > 10 minutes

---

### Webhook Signature Validation Failures

**Severity:** P1
**Symptoms:**
- All webhooks rejected with "Invalid signature"
- High invalid_signatures count in stats
- CRITICAL log messages about signature validation

**Diagnosis:**

```bash
# Check webhook stats
curl http://localhost:8080/health | jq '.webhook_stats'

# Look for signature validation errors
grep "signature validation failed" /var/log/outline-ai/service.log | wc -l

# Check for CRITICAL alerts
grep "CRITICAL.*signature" /var/log/outline-ai/service.log | tail -5
```

```sql
-- Check signature failure log (if implemented)
SELECT
    timestamp,
    received_signature,
    body_hash,
    ip_address
FROM signature_failures
ORDER BY timestamp DESC
LIMIT 20;
```

**Common Causes:**

1. **Webhook secret mismatch**
2. **Secret rotation without updating config**
3. **Webhook secret exposed/regenerated in Outline**

**Recovery Procedure:**

1. **Verify webhook secret**

a. Get current secret from Outline:
- Log into Outline admin panel
- Navigate to Settings → Webhooks
- Click on webhook to view details
- Copy the secret

b. Compare with config:
```bash
# Check current config
cat config.yaml | grep webhook_secret
# Or check environment variable
echo $OUTLINE_WEBHOOK_SECRET
```

2. **If secrets don't match**: Update config

```bash
# Update config file
vim config.yaml
# Set: outline.webhook_secret: "correct_secret_here"

# Or update environment variable
export OUTLINE_WEBHOOK_SECRET="correct_secret_here"

# Restart service
sudo systemctl restart outline-ai
```

3. **If secret was rotated**: Implement grace period

```bash
# Config supports previous secret for rotation
# outline.webhook_secret_previous: "old_secret"

# Service will accept both secrets for 24 hours
```

4. **Verify recovery**
```bash
# Trigger a test webhook by editing any document in Outline
# Check logs
tail -f /var/log/outline-ai/service.log | grep "Received webhook event"

# Check stats
curl http://localhost:8080/health | jq '.webhook_stats.valid_signatures'
```

**Prevention:**
- Secure secret storage (environment variables, secrets manager)
- Document secret rotation procedure
- Implement grace period for rotation (already in design)
- Alert on signature validation failure rate > 10%

---

### Event Queue Overflow

**Severity:** P1
**Symptoms:**
- "Event queue full" errors in logs
- Webhook responses with 503 status
- overflow_events table growing

**Diagnosis:**

```bash
# Check queue utilization
curl http://localhost:8080/health | jq '.webhook_stats | {queue_size, queue_capacity}'

# Count overflow events
sqlite3 /path/to/outline-ai.db "SELECT COUNT(*) FROM overflow_events;"
```

```sql
-- View overflow events
SELECT
    event_type,
    model_type,
    model_id,
    stored_at
FROM overflow_events
ORDER BY stored_at DESC
LIMIT 50;

-- Overflow rate over time
SELECT
    date(stored_at) as date,
    COUNT(*) as overflow_count
FROM overflow_events
GROUP BY date(stored_at)
ORDER BY date DESC;
```

**Recovery Procedure:**

1. **Immediate: Process overflow events**

The overflow processor runs automatically every 5 minutes, but you can force it:

```bash
# Restart service to trigger immediate processing
sudo systemctl restart outline-ai

# Or use admin endpoint
curl -X POST http://localhost:8080/admin/overflow/process
```

2. **Increase worker pool capacity** (if load is sustained)

```yaml
# Edit config.yaml
service:
  max_concurrent_workers: 5  # Increase from 3
```

```bash
# Restart service
sudo systemctl restart outline-ai
```

3. **Increase queue size** (if bursts are common)

```yaml
# Edit config.yaml
webhooks:
  queue_size: 2000  # Increase from 1000
```

4. **Monitor until overflow clears**
```bash
# Watch overflow count decrease
watch -n 5 'echo "SELECT COUNT(*) FROM overflow_events;" | sqlite3 /path/to/outline-ai.db'

# Watch queue utilization
watch -n 2 'curl -s http://localhost:8080/health | jq .webhook_stats.queue_size'
```

**Prevention:**
- Right-size queue capacity for expected load
- Alert on queue utilization > 80%
- Scale worker count with load
- Implement overflow handling (already in design)

---

## Database Issues

### Database Lock / Corruption

**Severity:** P0
**Symptoms:**
- "database is locked" errors
- Service won't start
- PRAGMA integrity_check fails

**Diagnosis:**

```bash
# Check for lock file
ls -la /path/to/outline-ai.db-shm
ls -la /path/to/outline-ai.db-wal

# Check processes holding lock
lsof /path/to/outline-ai.db

# Test database access
sqlite3 /path/to/outline-ai.db "SELECT COUNT(*) FROM question_state;"

# Check integrity
sqlite3 /path/to/outline-ai.db "PRAGMA integrity_check;"
```

**Recovery Procedure:**

1. **Stop all processes**
```bash
sudo systemctl stop outline-ai
pkill -9 outline-ai
```

2. **Checkpoint WAL**
```bash
sqlite3 /path/to/outline-ai.db "PRAGMA wal_checkpoint(TRUNCATE);"
```

3. **If checkpoint fails**: Recovery mode
```bash
# Backup current database
cp /path/to/outline-ai.db /path/to/outline-ai.db.backup.$(date +%Y%m%d_%H%M%S)

# Try to recover
sqlite3 /path/to/outline-ai.db ".recover" > recovery.sql

# Create new database from recovery
mv /path/to/outline-ai.db /path/to/outline-ai.db.corrupt
sqlite3 /path/to/outline-ai.db < recovery.sql

# Verify
sqlite3 /path/to/outline-ai.db "PRAGMA integrity_check;"
```

4. **If corruption is severe**: Restore from backup
```bash
# List backups
ls -lht /path/to/backups/

# Restore latest good backup
cp /path/to/backups/state-20260119-080000.db /path/to/outline-ai.db

# Verify
sqlite3 /path/to/outline-ai.db "PRAGMA integrity_check;"
```

5. **Restart service**
```bash
sudo systemctl start outline-ai
```

**Data Loss Assessment:**
```sql
-- Check last question answered
SELECT MAX(processed_at) as last_question
FROM question_state;

-- Check last command processed
SELECT MAX(executed_at) as last_command
FROM command_log;

-- Gap between backup and corruption
-- (Manually determine based on backup timestamp and current time)
```

**Prevention:**
- Automated daily backups (configure in config.yaml)
- WAL mode with automatic checkpoints
- Single writer configuration (already set)
- Regular integrity checks (weekly cron job)

---

### Database Size Growing Rapidly

**Severity:** P2
**Symptoms:**
- Database file > 100MB (much larger than expected)
- Disk space alerts
- Slow query performance

**Diagnosis:**

```bash
# Check database size
du -h /path/to/outline-ai.db*

# Check table sizes
sqlite3 /path/to/outline-ai.db "
SELECT
    name,
    SUM(pgsize) as size_bytes,
    COUNT(*) as page_count
FROM dbstat
GROUP BY name
ORDER BY size_bytes DESC;
"

# Count rows in large tables
sqlite3 /path/to/outline-ai.db "
SELECT 'question_state' as table_name, COUNT(*) as row_count FROM question_state
UNION ALL
SELECT 'command_log', COUNT(*) FROM command_log
UNION ALL
SELECT 'dead_letter_queue', COUNT(*) FROM dead_letter_queue
UNION ALL
SELECT 'overflow_events', COUNT(*) FROM overflow_events;
"
```

**Recovery Procedure:**

1. **Clean up old data**
```sql
-- Remove old question state (> 30 days)
DELETE FROM question_state
WHERE processed_at < datetime('now', '-30 days');

-- Remove old command logs (> 90 days)
DELETE FROM command_log
WHERE executed_at < datetime('now', '-90 days');

-- Archive and remove old DLQ entries
INSERT INTO dead_letter_archive
SELECT * FROM dead_letter_queue
WHERE last_failure < datetime('now', '-7 days');

DELETE FROM dead_letter_queue
WHERE last_failure < datetime('now', '-7 days');

-- Remove processed overflow events
DELETE FROM overflow_events
WHERE stored_at < datetime('now', '-1 days');
```

2. **Vacuum database** (reclaim space)
```bash
# This may take a while and locks the database
sqlite3 /path/to/outline-ai.db "VACUUM;"

# Check new size
du -h /path/to/outline-ai.db
```

3. **Optimize database**
```bash
sqlite3 /path/to/outline-ai.db "
PRAGMA optimize;
ANALYZE;
"
```

**Prevention:**
- Automated cleanup (already in design):
  - Question state: 30 day retention
  - Command logs: 90 day retention (optional logging)
  - DLQ: 7 day retention
  - Overflow events: 24 hour retention
- Regular VACUUM (monthly cron job)
- Monitor database size (alert if > 50MB)

---

## Monitoring and Alerts

### Essential Metrics to Monitor

**Service Health:**
```bash
# Health check endpoint
curl http://localhost:8080/health | jq .

# Key metrics:
# - status: "healthy"
# - worker_pool.active_workers: > 0
# - worker_pool.queued_tasks: < queue_size * 0.8
# - webhook_stats.last_event_time: < 10 minutes ago
# - webhook_stats.queue_size: < queue_capacity * 0.8
```

**Database Metrics:**
```sql
-- Key table sizes
SELECT
    (SELECT COUNT(*) FROM question_state) as questions,
    (SELECT COUNT(*) FROM command_log) as commands,
    (SELECT COUNT(*) FROM dead_letter_queue) as dlq_size,
    (SELECT COUNT(*) FROM overflow_events) as overflow_size,
    (SELECT COUNT(*) FROM failed_webhook_events) as failed_webhooks;

-- Recent activity
SELECT
    COUNT(*) as questions_last_hour,
    AVG(answer_delivered) as answer_rate
FROM question_state
WHERE processed_at > datetime('now', '-1 hour');
```

### Recommended Alerts

**Critical (P0):**
- Service down for > 5 minutes
- Health check failing
- Database corruption detected
- All webhook signature validations failing

**High (P1):**
- Worker pool stuck (no activity > 10 minutes)
- Queue overflow events > 10 in 5 minutes
- DLQ size > 50 entries
- No webhooks received in > 30 minutes (during business hours)

**Medium (P2):**
- Queue utilization > 80%
- Task failure rate > 10%
- Comment posting failure rate > 20%
- Database size > 50MB

**Low (P3):**
- Average task execution time > 2 minutes
- Command state stuck > 1 hour
- Signature validation failure rate > 5%

### Prometheus Metrics (If Implemented)

```
# Service health
outline_ai_service_up{} 1

# Worker pool
outline_ai_worker_pool_size{} 3
outline_ai_worker_pool_active{} 2
outline_ai_worker_pool_queued{} 5
outline_ai_tasks_completed_total{type="ai-file"} 145
outline_ai_tasks_failed_total{type="ai-file"} 12

# Webhooks
outline_ai_webhooks_received_total{} 1234
outline_ai_webhooks_invalid_signature_total{} 0
outline_ai_webhooks_processed_total{result="success"} 1200
outline_ai_webhooks_processed_total{result="failed"} 34
outline_ai_webhook_queue_size{} 5

# Database
outline_ai_db_size_bytes{} 25165824
outline_ai_dlq_size{} 3
```

### Log Patterns to Monitor

```bash
# Errors to alert on
grep -E "FATAL|CRITICAL|panic" /var/log/outline-ai/service.log

# Task failures
grep "Task execution failed" /var/log/outline-ai/service.log | wc -l

# Worker stalls
grep "Worker appears to be stalled" /var/log/outline-ai/service.log

# Database issues
grep -E "database.*locked|database.*corrupt" /var/log/outline-ai/service.log

# Webhook issues
grep -E "signature validation failed|queue full|overflow" /var/log/outline-ai/service.log
```

---

## Prevention Strategies

### 1. Regular Maintenance

**Daily:**
- Check health endpoint
- Review error logs
- Monitor disk space

**Weekly:**
- Review DLQ entries
- Check database size
- Verify webhook delivery
- Review failed command states

**Monthly:**
- VACUUM database
- Archive old logs
- Review and optimize worker count
- Test backup restoration

### 2. Configuration Best Practices

```yaml
# Recommended production config
service:
  max_concurrent_workers: 3  # Adjust based on load
  health_check_port: 8080

webhooks:
  enabled: true
  queue_size: 1000  # Increase if overflow is frequent
  signature_validation: true

  fallback_polling:
    enabled: true  # Safety net
    interval: 60s

processing:
  max_retries: 3
  retry_backoff_base: 30s
  retry_backoff_max: 5m

logging:
  level: "info"  # Use "debug" only for troubleshooting
  format: "json"

# Automated cleanup
cleanup:
  question_state_retention: 30d
  command_log_retention: 90d  # Or disable command logging
  dead_letter_retention: 7d
  cleanup_interval: 1h

# Automated backups
backups:
  enabled: true
  interval: 24h
  directory: /var/backups/outline-ai
  max_backups: 7  # Keep 1 week
```

### 3. Operational Runbook Checklist

**Before Deployment:**
- [ ] Backup current database
- [ ] Test new version in staging
- [ ] Review configuration changes
- [ ] Update runbook if needed

**After Deployment:**
- [ ] Verify service starts successfully
- [ ] Check health endpoint
- [ ] Monitor logs for errors
- [ ] Test key functionality (add command, verify processing)
- [ ] Check webhook stats
- [ ] Verify no spike in errors

**During Incident:**
- [ ] Check service health
- [ ] Review recent logs
- [ ] Check database integrity
- [ ] Review DLQ and overflow tables
- [ ] Document incident timeline
- [ ] Perform root cause analysis
- [ ] Update runbook with learnings

### 4. Disaster Recovery Plan

**RPO (Recovery Point Objective):** 24 hours (daily backups)
**RTO (Recovery Time Objective):** 30 minutes

**Backup Strategy:**
- Automated daily backups (via config)
- Keep 7 days of backups
- Store backups on separate disk/volume
- Test restoration monthly

**Recovery Procedure:**
1. Stop service
2. Restore database from backup
3. Start service with catch-up enabled
4. Monitor catch-up process
5. Verify functionality

**Data Loss Acceptable:**
- Question state: Yes (will reprocess questions)
- Command state: Yes (will redetect commands)
- Command logs: Yes (optional audit trail)

**Data Loss Not Acceptable:**
- None (all data is reproducible from Outline)

---

## Appendix: SQL Queries

### Useful Diagnostic Queries

```sql
-- Overall system health
SELECT
    'Questions Answered' as metric,
    COUNT(*) as count,
    MAX(processed_at) as last_activity
FROM question_state
WHERE answer_delivered = 1

UNION ALL

SELECT
    'Commands Processed',
    COUNT(*),
    MAX(executed_at)
FROM command_log
WHERE status = 'success'

UNION ALL

SELECT
    'Tasks in DLQ',
    COUNT(*),
    MAX(last_failure)
FROM dead_letter_queue

UNION ALL

SELECT
    'Overflow Events',
    COUNT(*),
    MAX(stored_at)
FROM overflow_events;

-- Activity over last 24 hours
SELECT
    strftime('%Y-%m-%d %H:00', processed_at) as hour,
    COUNT(*) as questions_answered
FROM question_state
WHERE processed_at > datetime('now', '-24 hours')
  AND answer_delivered = 1
GROUP BY strftime('%Y-%m-%d %H:00', processed_at)
ORDER BY hour DESC;

-- Error frequency
SELECT
    CASE
        WHEN failure_reason LIKE '%timeout%' THEN 'Timeout'
        WHEN failure_reason LIKE '%auth%' THEN 'Auth Error'
        WHEN failure_reason LIKE '%not found%' THEN 'Not Found'
        WHEN failure_reason LIKE '%rate%' THEN 'Rate Limit'
        ELSE 'Other'
    END as error_type,
    COUNT(*) as count
FROM dead_letter_queue
GROUP BY error_type
ORDER BY count DESC;

-- Webhook processing stats
SELECT
    strftime('%Y-%m-%d', stored_at) as date,
    COUNT(*) as overflow_events
FROM overflow_events
WHERE stored_at > datetime('now', '-7 days')
GROUP BY date
ORDER BY date DESC;

-- Documents needing attention
SELECT
    d.document_id,
    d.task_type,
    d.failure_reason,
    d.attempt_count,
    c.command_type,
    c.status
FROM dead_letter_queue d
LEFT JOIN command_state c ON d.document_id = c.document_id
ORDER BY d.last_failure DESC
LIMIT 20;
```

---

## Document Version History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2026-01-19 | AI Design Team | Initial runbook creation |

---

**End of Runbook**

For questions or updates to this runbook, please contact the operations team or update this document in the repository.
