#!/bin/bash

# JWT Token Generator Script
# Generates a JWT token for testing the API

echo "🔑 Generating JWT Token for Import-Export Backend"
echo "================================================="
echo ""

# Generate token using Go
TOKEN_OUTPUT=$(go run cmd/generate-token/main.go)

echo "$TOKEN_OUTPUT"
echo ""

# Extract just the token from the output
TOKEN=$(echo "$TOKEN_OUTPUT" | grep "Bearer " | cut -d' ' -f2)

if [ ! -z "$TOKEN" ]; then
    echo "📋 Quick Test Commands:"
    echo "======================"
    echo ""
    echo "# Test health check"
    echo "curl $BASE_URL/health"
    echo ""
    echo "# Test products endpoint"
    echo "curl -H \"Authorization: Bearer $TOKEN\" $BASE_URL/api/v1/products"
    echo ""
    echo "# Test suppliers endpoint"
    echo "curl -H \"Authorization: Bearer $TOKEN\" $BASE_URL/api/v1/suppliers"
    echo ""
    echo "📝 Postman Environment:"
    echo "======================"
    echo "Update your Postman environment with:"
    echo "auth_token = $TOKEN"
    echo ""
    echo "⏰ Token expires in 24 hours"
else
    echo "❌ Failed to generate token"
    exit 1
fi
