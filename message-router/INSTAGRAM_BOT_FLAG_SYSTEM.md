# Instagram Bot Flag System

## Overview
This system provides 100% reliable detection of bot vs human agent messages on Instagram by using a flag-based approach instead of relying on Instagram's inconsistent `app_id` values.

## How It Works

### For Instagram Messages:
1. **Dify sets flag first**: Before generating a response, Dify calls the message-router API to set a bot flag
2. **Message-router stores flag**: The flag is stored in memory for that specific conversation
3. **Instagram webhook arrives**: When the echo message is received
4. **Flag check**: Instead of checking `app_id`, the system checks if a bot flag exists
   - **Flag exists** → Bot message → Don't disable bot → Clear flag and continue
   - **No flag** → Human agent message → Disable bot for 6 hours

### For Facebook Messages:
- **No changes**: Facebook messages continue to use the existing `app_id` and sender ID detection
- **Same reliability**: Facebook's `app_id` system works reliably, so no changes needed

## API Endpoint

### POST /api/mark-bot-response
**Purpose**: Allows Dify to mark a conversation as having a bot response pending

**Parameters**:
- `conversation_id` (query parameter): The conversation ID in format `pageID-userID`

**Example**:
```
POST /api/mark-bot-response?conversation_id=123456789-987654321
```

**Response**:
```json
{
  "status": "success",
  "conversation_id": "123456789-987654321",
  "message": "Bot flag set"
}
```

## Dify Integration

### Workflow Setup:
1. **HTTP Request Node**: Add this as the first node in your Dify workflow
   - URL: `https://your-message-router.com/api/mark-bot-response?conversation_id={{conversation_id}}`
   - Method: POST
   - Wait for 200 OK response before proceeding

2. **LLM Node**: Only executes after the flag is successfully set

3. **Response Node**: Sends the actual message to Instagram

### Example Dify Workflow:
```
User Message → Set Bot Flag → Wait for Confirmation → Generate LLM Response → Send to Instagram
```

## Benefits

1. **100% Accuracy**: No more false positives from Instagram's inconsistent `app_id=0` values
2. **Platform-Specific**: Instagram uses flag system, Facebook keeps existing reliable logic
3. **Race Condition Free**: Flag is guaranteed to be set before message is sent
4. **Simple Integration**: Single API call from Dify workflow
5. **Memory Efficient**: Flags are automatically cleared after use

## Technical Details

### Conversation ID Format:
- Format: `{pageID}-{userID}`
- Example: `123456789-987654321`
- Same format used throughout the system for consistency

### Thread Safety:
- Uses `sync.RWMutex` for concurrent access
- Safe for multiple simultaneous conversations

### Memory Management:
- Flags are automatically cleared after being checked
- No memory leaks from accumulated flags
- Lightweight in-memory storage

## Migration Impact

### What Changed:
- Instagram echo messages now use bot flag detection
- New API endpoint added for Dify integration
- Enhanced logging for Instagram message processing

### What Stayed the Same:
- Facebook message processing (unchanged)
- Database schema (no changes needed)
- Existing conversation management
- Bot enable/disable logic (same 6-hour timeout)

## Monitoring

### Log Messages:
- `🤖 Bot flag SET for conversation: {id}` - Flag was set by Dify
- `🔍 Bot flag CHECK for conversation {id}: {result}` - Flag was checked
- `🗑️ Bot flag CLEARED for conversation: {id}` - Flag was removed after use
- `📱 Instagram echo message - checking bot flag` - Instagram message processing
- `👤 Instagram human agent message detected (no bot flag)` - Human agent detected

### Health Check:
The system includes the new endpoint in the startup logs:
```
📍 Registered routes:
   - POST /api/mark-bot-response (Instagram Bot Flag)
```