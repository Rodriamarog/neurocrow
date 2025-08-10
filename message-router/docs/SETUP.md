# Development Setup Guide

## Prerequisites

Before setting up the Message Router service, ensure you have:

- **Go 1.23.4+** installed
- **PostgreSQL** database (local or remote)
- **Facebook Developer Account** with a configured app
- **Dify AI account** with API access
- **Fireworks AI account** with API access

## Environment Setup

### 1. Clone and Install Dependencies

```bash
cd /path/to/neurocrow/message-router
go mod download
```

### 2. Configure Environment Variables

The service requires several environment variables. Copy your existing `.env` file or create one with these required variables:

```bash
# Database Connection
DATABASE_URL=postgresql://username:password@localhost:5432/neurocrow_db

# Facebook Integration
FACEBOOK_APP_SECRET=your_facebook_app_secret_here
VERIFY_TOKEN=your_custom_verify_token

# AI Services
FIREWORKS_API_KEY=your_fireworks_api_key
# Note: Dify API keys are stored per-page in the database

# Optional Configuration
PORT=8080
LOG_LEVEL=INFO  # Options: DEBUG, INFO, WARN, ERROR
```

### 3. Database Setup

The service expects a PostgreSQL database with the proper schema. Refer to `docs/DATABASE.md` for the complete schema documentation.

Key tables that must exist:
- `clients` - Top-level client organizations
- `social_pages` - Facebook/Instagram pages with credentials
- `conversations` - User conversation threads
- `messages` - Individual messages

## Facebook App Configuration

### 1. Create Facebook App

1. Go to [Facebook Developers](https://developers.facebook.com/)
2. Create a new app with "Business" type
3. Add "Messenger" and "Webhooks" products

### 2. Configure Webhooks

1. In Facebook App Dashboard → Webhooks
2. Set webhook URL: `https://your-domain.com/webhook`
3. Set verify token (must match `VERIFY_TOKEN` in your .env)
4. Subscribe to these webhook fields:
   - `messages`

### 3. Page Access Tokens

For each Facebook/Instagram page:
1. Generate a page access token
2. Store in `social_pages` table with appropriate `dify_api_key`

## AI Service Setup

### Fireworks AI (Sentiment Analysis)

1. Sign up at [Fireworks AI](https://fireworks.ai/)
2. Get API key from dashboard
3. Set as `FIREWORKS_API_KEY` in environment

### Dify AI (Chatbot Responses)

1. Create account at [Dify](https://dify.ai/)
2. Create separate AI apps for each client/page
3. Get API keys (format: `app-xxxxxxxxxxxxx`)
4. Store in `social_pages.dify_api_key` column per page

## Local Development

### Running the Service

```bash
# With environment variables in .env file
go run .

# Or with explicit environment variables
DATABASE_URL="postgresql://..." FACEBOOK_APP_SECRET="..." go run .

# Build and run binary
go build -o message-router .
./message-router
```

### Testing Webhooks Locally

#### Option 1: Using ngrok (Recommended)

```bash
# Install ngrok
brew install ngrok  # macOS
# or download from https://ngrok.com/

# Start your local service
go run .

# In another terminal, expose local port
ngrok http 8080

# Use the ngrok HTTPS URL for Facebook webhook configuration
# Example: https://abc123.ngrok.io/webhook
```

#### Option 2: Using localhost.run

```bash
# Start service
go run .

# In another terminal
ssh -R 80:localhost:8080 ssh.localhost.run

# Use the provided URL for webhook configuration
```

### Testing Webhook Verification

```bash
# Test webhook verification (replace with your verify token and ngrok URL)
curl "https://your-ngrok-url.ngrok.io/webhook?hub.mode=subscribe&hub.verify_token=your_verify_token&hub.challenge=test123"

# Should return: test123
```

## Development Workflow

### 1. Database Changes

When modifying database schema:
1. Update table definitions
2. Update `docs/DATABASE.md`
3. Run migrations if needed
4. Test with existing data

### 2. Adding New Features

1. Update relevant documentation
2. Add appropriate logging with request correlation
3. Handle errors gracefully with fallback mechanisms
4. Test with different message types and platforms

### 3. Testing Message Processing

```bash
# Set debug logging to see detailed processing
LOG_LEVEL=DEBUG go run .

# Monitor logs for:
# - Webhook signature validation
# - Message filtering and routing
# - Sentiment analysis results
# - Database operations
# - AI API responses
```

## Debugging Common Issues

### Webhook Not Receiving Messages

1. **Check Facebook App Configuration**:
   - Verify webhook URL is correct and accessible
   - Confirm app secret matches environment variable
   - Ensure app has necessary permissions

2. **Verify Signature Validation**:
   - Check logs for signature validation errors
   - Confirm `FACEBOOK_APP_SECRET` is correct
   - Test with Facebook's webhook testing tools

### Database Connection Issues

```bash
# Test database connection
psql $DATABASE_URL -c "SELECT NOW();"

# Check connection pool settings in logs
LOG_LEVEL=DEBUG go run . 2>&1 | grep -i "database\|connection"
```

### AI API Issues

1. **Fireworks AI (Sentiment Analysis)**:
   - Verify API key is valid
   - Check request/response in debug logs
   - Monitor token usage and rate limits

2. **Dify AI (Chatbot)**:
   - Confirm API keys in database are correct format (`app-xxxxx...`)
   - Check individual page configurations
   - Monitor conversation context preservation

### Message Processing Issues

```bash
# Enable detailed message logging
LOG_LEVEL=DEBUG go run .

# Look for these log patterns:
# ✅ - Successful operations
# ❌ - Errors that need attention
# ⚠️ - Warnings or unusual patterns
# 🔍 - Debug information
```

## Monitoring and Observability

### Health Checks

```bash
# Service health check
curl http://localhost:8080/

# Expected response:
# {"status":"healthy","message":"Neurocrow Message Router is running"}
```

### Log Analysis

```bash
# Filter by request ID for tracing
LOG_LEVEL=DEBUG go run . 2>&1 | grep "req_abcd1234"

# Monitor sentiment analysis
go run . 2>&1 | grep "sentiment\|tokens\|cost"

# Watch bot control changes
go run . 2>&1 | grep "bot.*enabled\|bot.*disabled\|Bot.*enabled\|Bot.*disabled"
```

### Performance Monitoring

- Database connection pool usage
- API response times (Fireworks, Dify, Facebook)
- Message processing latency
- Memory and CPU usage

## Production Considerations

### Security

- Use HTTPS for webhook endpoints
- Secure environment variable storage
- Regular rotation of API keys and secrets
- Database connection encryption

### Scaling

- Connection pool optimization
- Horizontal scaling considerations
- Database query optimization
- Caching strategies for user profiles

### Error Handling

- Implement proper alerting for critical errors
- Monitor webhook delivery failures
- Track API rate limits and quota usage
- Set up dead letter queues for failed messages

## Troubleshooting Checklist

- [ ] Environment variables configured correctly
- [ ] Database accessible and schema up to date
- [ ] Facebook webhook verification passing
- [ ] API keys valid and not rate limited
- [ ] Logs showing expected message flow
- [ ] Thread control state transitions working
- [ ] Multi-tenant data isolation functioning

## Getting Help

1. Check service logs with appropriate `LOG_LEVEL`
2. Review `docs/DATABASE.md` for schema questions
3. Verify Facebook app configuration
4. Test AI API connectivity independently
5. Check webhook signature validation

For complex issues, enable `LOG_LEVEL=DEBUG` and trace message flow using request IDs in the logs.