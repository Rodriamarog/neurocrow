I'll create a comprehensive refactoring plan to migrate from your custom bot disable system to Facebook's native Handover Protocol. This is a significant architectural change that will simplify your codebase considerably.

## Update Todos

* [ ] Analyze current system architecture and Facebook Handover Protocol requirements
* [ ] Design new database schema changes needed for handover protocol
* [ ] Plan Facebook Graph API integration for thread control
* [ ] Design new webhook handling for handover events
* [ ] Plan sentiment analysis response refactoring
* [ ] Create step-by-step implementation roadmap

> ✻ **Thinking…**
>
> Let me work through this systematically to create a comprehensive refactoring plan.
>
> First, let me understand what will change and what can be removed/simplified.

# Comprehensive Refactoring Plan: Custom Bot System → Facebook Handover Protocol

## 🎯 Overview & Benefits

### Current Problems Solved

- Eliminates race conditions in bot state management
- Removes complex 6-hour timer logic and background workers
- Simplifies echo message detection
- Reduces database complexity
- Native Facebook integration = more reliable

### Key Principle

Let Facebook handle thread control instead of custom state management.

---
## 📊 Phase 1: Database Schema Evolution

### Current Schema Issues to Address

```sql
-- PROBLEMATIC FIELDS (in conversations table):
bot_enabled          boolean    -- Custom state, will be replaced
last_human_message_at timestamp -- Manual tracking, Facebook handles this
-- Complex reactivation logic dependent on these fields
```

### New Schema Design

```sql
-- ADD to conversations table:
thread_control_app_id     bigint        -- Which app controls thread (bot vs inbox)
thread_control_status     varchar(20)   -- 'bot_control', 'human_control', 'unknown'  
handover_requested_at     timestamp     -- When handover was requested
handover_reason           text          -- 'user_request', 'frustrated', 'error'

-- OPTIONAL - Keep for analytics but don't use for logic:
bot_enabled              boolean       -- Deprecated, keep for historical data
last_human_message_at    timestamp     -- Deprecated, keep for historical data
```

### Migration Strategy

1.  **Phase 1A:** Add new columns with defaults
2.  **Phase 1B:** Populate existing data (set `thread_control_status = 'unknown'`)
3.  **Phase 1C:** Update application logic to use new fields
4.  **Phase 1D:** Stop using old fields (but keep for rollback)

---
## 🔧 Phase 2: Facebook Graph API Integration

### New API Functions to Add

```go
// facebook.go - New functions needed
func passThreadControl(ctx context.Context, pageAccessToken, recipientID string, targetAppID int64, metadata string) error
func takeThreadControl(ctx context.Context, pageAccessToken, recipientID string, metadata string) error
func getThreadOwner(ctx context.Context, pageAccessToken, recipientID string) (*ThreadControlInfo, error)
```

### App ID Configuration

```go
// Add to Config struct
type Config struct {
    // ... existing fields
    FacebookPageInboxAppID int64 // App ID for Facebook Page Inbox (usually 263902037430900)
    FacebookBotAppID       int64 // Your bot's app ID
}
```

### Required Facebook Permissions

- `pages_messaging` (you already have this)
- `pages_messaging_subscriptions` (you already have this)
- No additional permissions needed for handover protocol

---
## 📨 Phase 3: Webhook Handling Refactor

### New Webhook Events to Handle

```
// Current: messaging -> messages, delivery, echoes
// ADD: messaging -> messaging_handovers
```

```go
type HandoverEvent struct {
    Sender struct {
        ID string `json:"id"`
    } `json:"sender"`
    Recipient struct {
        ID string `json:"id"`
    } `json:"recipient"`
    Timestamp int64 `json:"timestamp"`
    PassThreadControl *struct {
        NewOwnerAppID int64  `json:"new_owner_app_id"`
        PreviousOwnerAppID int64 `json:"previous_owner_app_id"`
        Metadata string `json:"metadata"`
    } `json:"pass_thread_control,omitempty"`
    TakeThreadControl *struct {
        PreviousOwnerAppID int64 `json:"previous_owner_app_id"`
        Metadata string `json:"metadata"`
    } `json:"take_thread_control,omitempty"`
}
```

### Updated Webhook Processing Logic

```go
// handlers.go modifications needed:

// REMOVE: Complex echo message detection logic (lines 137-192)
// REMOVE: updateConversationForHumanMessage calls
// ADD: Handle messaging_handovers events
// SIMPLIFY: Message processing (no more bot_enabled checks)
```

---
## 🧠 Phase 4: Sentiment Analysis Response Refactor

### Current Logic (`handlers.go:245-330`)

```go
// PROBLEMATIC - Custom bot disable
if analysis.Status == "need_human" {
    updateConversationForHumanMessage(ctx, entry.ID, msg.Sender.ID, platform)
    continue // Skip Dify processing
}

if analysis.Status == "frustrated" {
    // Send empathy but continue with bot - WRONG!
}
```

### New Logic with Handover Protocol

```go
// IMPROVED - Use Facebook's native handover
if analysis.Status == "need_human" {
    // Send handoff message  
    handoffMsg := "Te conectaré con un agente humano para ayudarte mejor."
    sendPlatformResponse(ctx, pageInfo, msg.Sender.ID, handoffMsg)

    // Pass control to Facebook Page Inbox
    err := passThreadControl(ctx, pageInfo.AccessToken, msg.Sender.ID,
        config.FacebookPageInboxAppID, "User requested human assistance")
    if err != nil {
        log.Printf("❌ Failed to pass thread control: %v", err)
        // Fallback: continue with bot
    } else {
        updateThreadControlStatus(ctx, msg.Sender.ID, "human_control", "user_request")
        continue // Don't process with Dify
    }
}

if analysis.Status == "frustrated" {
    // DECISION POINT: Pass to human OR send empathy + continue?
    // Option A: Pass to human immediately
    // Option B: Send empathy, continue with bot, but lower threshold for future handoff

    // Recommended: Option A for better user experience
    empathyMsg := "Entiendo tu frustración. Te conectaré con un agente para ayudarte mejor."
    sendPlatformResponse(ctx, pageInfo, msg.Sender.ID, empathyMsg)

    err := passThreadControl(ctx, pageInfo.AccessToken, msg.Sender.ID,
        config.FacebookPageInboxAppID, "User appears frustrated")
    if err == nil {
        updateThreadControlStatus(ctx, msg.Sender.ID, "human_control", "frustrated")
        continue
    }
    // If handoff fails, continue with bot
}
```

---
## 🗑️ Phase 5: Code Removal & Simplification

### Files/Functions to REMOVE

- **`main.go`**
  - `startBotReactivationWorker()` // Lines 228-248
  - `runBotReactivationCheck()`    // Lines 250-270

- **`conversation.go`**
  - `updateConversationForHumanMessage()` // Lines 248-380
  - `isRecentHumanActivity()`             // Lines 382-397
  - Complex `bot_enabled` logic

- **`handlers.go`**
  - Echo message detection logic    // Lines 137-192
  - Bot state re-checking logic    // Lines 304-314
  - `updateConversationForHumanMessage` calls

- **`bot_reactivation_migration.sql`**
  - All trigger and function definitions (entire file can be deprecated)

### Database Functions to REMOVE

```sql
-- These become unnecessary:
DROP FUNCTION IF EXISTS reactivate_idle_bots();
DROP FUNCTION IF EXISTS run_bot_reactivation_check();
DROP FUNCTION IF EXISTS check_bot_reactivation_on_message();
DROP TRIGGER IF EXISTS trigger_check_bot_reactivation ON messages;
```

---
## 🚀 Phase 6: Implementation Roadmap

### Step 1: Preparation (1-2 days)

1. Add Facebook handover API functions (don't use yet)
2. Add new database columns
3. Add webhook handling for `messaging_handovers` (log only, don't act)
4. Test webhook reception

### Step 2: Parallel Implementation (2-3 days)

1. Create new sentiment response logic (feature-flagged)
2. Add thread control status tracking
3. Test handover API calls in staging
4. Update webhook processing to handle both old and new logic

### Step 3: Gradual Migration (1-2 days)

1. Enable new logic for test conversations
2. Monitor handover events and thread control
3. Verify human agents can respond through Facebook inbox
4. Test bot regaining control

### Step 4: Full Cutover (1 day)

1. Switch all conversations to new logic
2. Disable old bot reactivation worker
3. Remove complex echo message detection
4. Monitor for issues

### Step 5: Cleanup (1-2 days)

1. Remove deprecated code
2. Remove database triggers and functions
3. Update documentation
4. Performance testing

---
## ⚠️ Risk Mitigation & Rollback Plan

### Key Risks

1. **Facebook API reliability** - Handover calls might fail
2. **Webhook delivery** - Handover events might be missed
3. **Human agent workflow** - Agents need to use Facebook Page Inbox
4. **Thread control confusion** - Multiple apps trying to control same thread

### Mitigation Strategies

1. **Graceful degradation** - If handover fails, continue with bot
2. **Fallback logic** - Keep simplified bot disable as emergency backup
3. **Monitoring** - Track handover success/failure rates
4. **Documentation** - Clear instructions for human agents

### Rollback Plan

1. **Phase 1-3:** Can rollback easily (new code feature-flagged)
2. **Phase 4+:** Requires re-enabling old workers and logic
3. **Database:** Old columns preserved for rollback
4. **Emergency:** Feature flag to disable handover and use old system

---
## 📈 Expected Benefits After Refactor

### Code Complexity Reduction

- Remove ~500 lines of complex state management code
- Eliminate race conditions in bot enable/disable logic
- Remove background workers and periodic tasks
- Simplify webhook processing by 60-70%

### Reliability Improvements

- Native Facebook integration = more reliable than custom solution
- Eliminates timer-based bugs (6-hour reactivation issues)
- Better human/bot coordination through Facebook's proven system
- Cleaner error handling (either handover works or bot continues)

### Operational Benefits

- Human agents use familiar Facebook interface
- No custom dashboard needed for human responses
- Automatic conversation context preserved by Facebook
- Better user experience (seamless handoff)

This refactor transforms your system from a complex custom solution to a simple, reliable integration with Facebook's native capabilities. The complexity reduction alone will
eliminate most of the logic issues you're currently experiencing.

### Somehow subscribe pages automatically to messaging_handovers