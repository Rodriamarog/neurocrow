#!/bin/bash

# Base URL - modify this to your endpoint
BASE_URL="https://neurocrow-message-router.onrender.com"

echo "🔍 Testing endpoints..."
echo ""

# Test root endpoint (health check)
echo "Testing health check (GET /)..."
curl -s -w "\nStatus: %{http_code}\n" $BASE_URL
echo ""

# Test webhook endpoint with GET
echo "Testing webhook endpoint (GET /webhook)..."
curl -s -w "\nStatus: %{http_code}\n" $BASE_URL/webhook
echo ""

# Test webhook endpoint with POST
echo "Testing webhook endpoint (POST /webhook)..."
curl -X POST -H "Content-Type: application/json" \
     -d '{"test": "message"}' \
     -s -w "\nStatus: %{http_code}\n" \
     $BASE_URL/webhook
echo ""