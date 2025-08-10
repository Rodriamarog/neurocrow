# Neurocrow Message Router

A high-performance Go service that handles Facebook Messenger and Instagram Direct Message webhooks, performs sentiment analysis, and intelligently routes conversations between users and AI chatbots.

## Overview

The Message Router is the core component of the Neurocrow platform that:

- **Processes Facebook/Instagram Webhooks**: Validates and handles incoming messages from social media platforms
- **Performs Sentiment Analysis**: Uses AI to analyze user messages and determine appropriate routing
- **Manages Bot Control**: Simple enable/disable system based on user requests and human agent activity
- **Routes to AI Chatbots**: Integrates with Dify AI platform for automated responses
- **Multi-tenant Architecture**: Supports multiple clients with isolated configurations

## Architecture

```
User Message (Facebook/Instagram)
         ↓
   Webhook Handler
         ↓
   Signature Validation
         ↓
   Sentiment Analysis (Fireworks AI)
         ↓
   Routing Decision
    ↙        ↘
AI Bot (Dify)  Human Agent
    ↓           ↓
Response → User
```

### Key Components

1. **Webhook Processing**: Validates Facebook signatures and processes message events
2. **Sentiment Analyzer**: Determines if messages require human intervention
3. **Bot Control Manager**: Simple boolean flag system for enabling/disabling bot responses  
4. **Database Layer**: Multi-tenant PostgreSQL storage for conversations and messages
5. **AI Integration**: Dify API integration for chatbot responses

## Quick Start

### Prerequisites

- Go 1.23.4+
- PostgreSQL database
- Facebook App with webhook permissions
- Dify AI API key
- Fireworks AI API key

### Environment Variables

Create a `.env` file in the project directory:

```bash
# Database
DATABASE_URL=postgresql://user:password@host:5432/neurocrow

# Facebook Integration
FACEBOOK_APP_SECRET=your_facebook_app_secret
VERIFY_TOKEN=your_webhook_verify_token

# AI Services
FIREWORKS_API_KEY=your_fireworks_api_key

# Optional
PORT=8080
LOG_LEVEL=INFO  # DEBUG, INFO, WARN, ERROR
```

### Running the Service

```bash
# Install dependencies
go mod download

# Run the service
go run .

# Or build and run
go build -o message-router .
./message-router
```

The service will start on the configured port (default: 8080) and be ready to receive webhooks at:
- `GET/POST /webhook` - Facebook/Instagram webhook endpoint
- `POST /send-message` - Send messages from dashboard
- `GET /` - Health check endpoint

## Database Schema

The service uses a multi-tenant PostgreSQL database with the following key tables:

- **clients**: Top-level client organizations
- **social_pages**: Facebook/Instagram pages with API credentials
- **conversations**: User conversation threads with bot control state
- **messages**: Individual messages with routing metadata

### Key Relationships

```
clients (1) → (many) social_pages
social_pages (1) → (many) conversations  
conversations (1) → (many) messages
```

## Message Processing Flow

### 1. Webhook Receipt
- Validates Facebook signature using HMAC-SHA256
- Parses webhook payload for message events
- Generates request ID for log correlation

### 2. Message Filtering
- Filters out delivery receipts and system messages
- Handles echo messages (distinguishes bot vs human agent responses)
- Validates message content and sender information

### 3. Sentiment Analysis
- Analyzes message text using Fireworks AI
- Categorizes as: `general`, `frustrated`, or `need_human`
- Routes based on sentiment and current thread control status

### 4. Response Generation
- **General messages**: Routes to Dify AI for automated response
- **Frustrated users**: Sends empathy message, escalates to human
- **Human requests**: Connects to human agent immediately

### 5. Bot Control Management
- Simple boolean flag system (`bot_enabled`) for conversation control
- Auto-disables bot when human agents respond or users request human help
- Auto-reactivates bot after 12 hours of human agent inactivity

## Bot Control States

The service manages conversation control using a simple boolean flag system:

- **`bot_enabled = true`**: AI chatbot processes and responds to messages
- **`bot_enabled = false`**: Bot is disabled, messages logged but no automated responses

## Configuration

### Facebook Webhook Setup

1. Configure webhook URL: `https://your-domain.com/webhook`
2. Subscribe to `messages` events
3. Set verify token in environment variables

### Multi-tenant Setup

Each client can have multiple Facebook/Instagram pages, each with:
- Unique Dify API key for isolated AI responses
- Individual access tokens for platform integration
- Separate conversation and message storage

## Logging and Debugging

The service provides structured logging with different levels:

```bash
# Set log level via environment variable
LOG_LEVEL=DEBUG  # Shows detailed request/response data
LOG_LEVEL=INFO   # Standard operational logging (default)
LOG_LEVEL=WARN   # Warnings and errors only
LOG_LEVEL=ERROR  # Errors only
```

Log entries include:
- Request correlation IDs for tracing async operations
- Detailed webhook processing steps
- Sentiment analysis results and token usage
- Database operation status
- Thread control transitions

## Error Handling

The service implements comprehensive error handling:

- **Graceful degradation**: Defaults to bot-enabled on database errors
- **Retry logic**: 3 attempts for external API calls with backoff
- **Transaction safety**: Database operations use row-level locking
- **Webhook resilience**: Always returns 200 OK to Facebook to prevent retries

## Legacy Systems

The service is currently migrating from Botpress to Dify AI integration:

- **Current**: Dify API integration for new conversations
- **Legacy**: Some database columns retained for historical data
- **Thread Control**: Modern Facebook Handover Protocol (deprecates 6-hour timer)

## Monitoring

Health check endpoint provides service status:
```bash
curl http://localhost:8080/
# Response: {"status":"healthy","message":"Neurocrow Message Router is running"}
```

## Development

### Testing Webhooks Locally

1. Use ngrok or similar tool to expose local service
2. Configure Facebook webhook URL to tunnel endpoint  
3. Monitor logs with `LOG_LEVEL=DEBUG` for detailed tracing

### Database Queries

The service provides detailed logging of database operations. Key tables to monitor:
- `conversations.bot_enabled` - Current bot control state (true/false)
- `messages.source` - Message origin tracking (user/bot/human/system)

## Security

- Facebook webhook signature validation using HMAC-SHA256
- Environment variable based secret management
- No hardcoded credentials or API keys
- Database prepared statements to prevent injection

## Performance

- Connection pooling for database operations (25 concurrent connections)
- Async message processing to prevent webhook timeouts
- Request ID correlation for debugging without performance impact
- Efficient sentiment analysis with low token usage

## Support

For development questions or issues:
1. Check logs with appropriate log level
2. Verify webhook configuration and signatures
3. Monitor database connection and query performance
4. Test sentiment analysis API connectivity