# Database Schema Documentation

## Overview

The Neurocrow Message Router uses a multi-tenant PostgreSQL database designed to support multiple clients, each with their own social media pages and isolated conversation data. The schema implements proper foreign key relationships and includes legacy columns for backward compatibility during system migration.

## Entity Relationship Diagram

```
clients (1) ────┬─── (many) social_pages
                │    social_pages (1) ────┬─── (many) conversations
                │                         │    conversations (1) ───── (many) messages
                └──── (many) users        │
                                          └─── (many) messages
```

## Table Definitions

### clients
Top-level client organizations that own social media pages and users.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | uuid | PRIMARY KEY, DEFAULT uuid_generate_v4() | Unique client identifier |
| name | text | NOT NULL | Client organization name |
| email | text | UNIQUE | Contact email for the client |
| facebook_user_id | text | UNIQUE | Facebook user ID (if applicable) |
| created_at | timestamptz | DEFAULT now() | Client creation timestamp |

**Relationships:**
- One-to-many with `social_pages`
- One-to-many with `users`
- One-to-many with `messages` (via client_id)

---

### social_pages
Facebook and Instagram pages with API credentials and AI integration settings.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | uuid | PRIMARY KEY, DEFAULT uuid_generate_v4() | Internal page identifier |
| client_id | uuid | NOT NULL, FK → clients.id | Owner client |
| platform | text | NOT NULL, CHECK ('facebook', 'instagram') | Social media platform |
| page_id | text | NOT NULL | Facebook/Instagram page ID |
| page_name | text | NOT NULL | Display name of the page |
| access_token | text | NOT NULL | Facebook Graph API access token |
| status | text | | Page status (active/inactive) |
| dify_api_key | text | | Dify AI API key (format: app-xxxxx...) |
| activated_at | timestamptz | | Page activation timestamp |
| created_at | timestamptz | DEFAULT now() | Record creation timestamp |

**Key Features:**
- **Multi-tenant AI**: Each page has its own `dify_api_key` for isolated AI responses
- **Platform Support**: Supports both Facebook Messenger and Instagram Direct Messages
- **Access Control**: Page-specific access tokens for Facebook Graph API operations

**Relationships:**
- Many-to-one with `clients`
- One-to-many with `conversations`
- One-to-many with `messages`

---

### conversations
User conversation threads with bot control state management.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| thread_id | text | PRIMARY KEY | User ID from Facebook/Instagram |
| page_id | uuid | NOT NULL, FK → social_pages.id | Associated social media page |
| platform | text | NOT NULL | Platform identifier |
| bot_enabled | boolean | DEFAULT true | Controls whether the bot processes messages for this conversation |
| latest_message_at | timestamptz | NOT NULL | Timestamp of most recent message |
| message_count | integer | DEFAULT 0 | Total messages in conversation |
| first_message_at | timestamptz | | Timestamp of first message |
| social_user_name | varchar | | User's display name from platform |
| profile_picture_url | text | DEFAULT placeholder | User's profile picture URL |
| unread_count | integer | DEFAULT 0 | Unread messages count |
| last_message_content | text | | Preview of last message |
| last_message_sender | text | | Source of last message |
| dify_conversation_id | text | | Dify AI conversation ID for context |
| created_at | timestamptz | DEFAULT CURRENT_TIMESTAMP | Record creation |
| updated_at | timestamptz | DEFAULT CURRENT_TIMESTAMP | Last modification |

#### Additional Timestamp Columns
| Column | Type | Description |
|--------|------|-------------|
| last_bot_message_at | timestamptz | Last bot response timestamp |
| last_human_message_at | timestamptz | Last human agent message - used for 12-hour bot reactivation logic |
| last_user_message_at | timestamptz | Last user message timestamp |

**Bot Control System:**
The service uses the simple `bot_enabled` boolean flag to control bot behavior:
- `true`: Bot processes messages and sends automated responses
- `false`: Bot is disabled, messages are logged but no responses sent
- Automatic reactivation after 12 hours of human agent inactivity

**Relationships:**
- Many-to-one with `social_pages`
- One-to-many with `messages`

---

### messages
Individual messages with routing metadata and source tracking.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | uuid | PRIMARY KEY, DEFAULT uuid_generate_v4() | Unique message identifier |
| client_id | uuid | FK → clients.id | Owner client (can be NULL) |
| page_id | uuid | FK → social_pages.id | Associated page |
| thread_id | text | FK → conversations.thread_id | Conversation thread |
| platform | text | NOT NULL | Platform identifier |
| content | text | NOT NULL | Message text content |
| from_user | text | NOT NULL | Message sender identifier |
| source | text | NOT NULL, CHECK ('bot', 'human', 'user', 'system') | Message origin |
| requires_attention | boolean | DEFAULT false | Needs human review |
| internal | boolean | DEFAULT false | Internal system message |
| read | boolean | DEFAULT false | Message read status |
| timestamp | timestamptz | DEFAULT now() | Message timestamp |

**Source Types:**
- **`user`**: Messages from end users (Facebook/Instagram users)
- **`bot`**: Automated responses from AI chatbots
- **`human`**: Messages from human agents
- **`system`**: Internal system messages (state changes, logs)

**Multi-tenant Notes:**
- `client_id` can be NULL for system-level messages
- Messages are associated with both client and page for proper isolation
- Thread control state changes are logged as system messages

**Relationships:**
- Many-to-one with `clients`
- Many-to-one with `social_pages`  
- Many-to-one with `conversations`

---

### users
Application users (human agents and administrators).

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | uuid | PRIMARY KEY, DEFAULT uuid_generate_v4() | User identifier |
| email | varchar | NOT NULL, UNIQUE | Login email |
| password_hash | varchar | NOT NULL | Hashed password |
| client_id | uuid | FK → clients.id | Associated client |
| role | varchar | DEFAULT 'client' | User role |
| created_at | timestamptz | DEFAULT CURRENT_TIMESTAMP | Account creation |
| updated_at | timestamptz | DEFAULT CURRENT_TIMESTAMP | Last modification |

**Relationships:**
- Many-to-one with `clients`

## Key Design Patterns

### Multi-tenant Architecture
- **Client Isolation**: Each client has separate data through `client_id` foreign keys
- **Page-specific Configuration**: Individual Dify API keys and access tokens per page
- **Flexible Associations**: System messages can exist without client association

### Bot Control Management
- **Simple Flag System**: Uses `bot_enabled` boolean for conversation control
- **State Transitions**: Bot enable/disable events logged as system messages
- **Automatic Reactivation**: Database function reactivates bots after 12 hours

### Message Source Tracking
- **Clear Attribution**: Every message tagged with source (user/bot/human/system)
- **Attention Flags**: Automatic flagging of messages requiring human review
- **Audit Trail**: Complete conversation history with state change logs

### Performance Considerations
- **Indexed Lookups**: Primary keys on UUIDs for fast joins
- **Timestamp Ordering**: Efficient message ordering and conversation updates
- **Row-level Locking**: Prevents race conditions during state updates

## Migration Notes

### Botpress → Dify Migration
- **API Integration**: Moved from Botpress webhooks to direct Dify API calls
- **Conversation Context**: Now maintained via `dify_conversation_id` field
- **Per-tenant Keys**: Each page has individual Dify API key for isolation

### Bot Control System
- **Simple Implementation**: Uses `bot_enabled` boolean flag for control
- **Human Agent Detection**: Echo message analysis to detect agent intervention
- **Auto-reactivation**: 12-hour timer system via database function

### Data Retention
- **Historical Data**: Existing conversations maintain full history
- **Message Archives**: Complete audit trail of all conversation interactions

## Database Functions

### reenable_disabled_bots()
Automatically reactivates bots disabled for 12+ hours due to human agent inactivity.

**Returns:** Number of bots reactivated
**Usage:** Called during message processing to implement auto-reactivation rule

## Indexes and Performance

### Key Indexes
- `conversations.thread_id` (PRIMARY KEY)
- `social_pages.page_id` for webhook lookups
- `messages.thread_id` for conversation queries
- `messages.timestamp` for message ordering

### Connection Pooling
- **Max Connections**: 25 concurrent connections
- **Max Idle**: 25 idle connections  
- **Connection Lifetime**: 5 minutes

## Security Considerations

### Access Control
- **Client Isolation**: Strict foreign key relationships prevent cross-client access
- **Token Security**: Facebook access tokens stored encrypted at rest
- **API Key Management**: Dify API keys handled securely per tenant

### Data Protection
- **Prepared Statements**: All queries use parameterized statements
- **Row-level Security**: Future enhancement for additional access control
- **Audit Logging**: Complete message and state change history