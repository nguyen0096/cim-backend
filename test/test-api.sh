#!/bin/bash

# Import-Export Backend API Test Script
# This script tests all the main API endpoints

BASE_URL="http://localhost:8080"
JWT_TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoidGVzdC11c2VyLXV1aWQiLCJlbWFpbCI6InRlc3RAZXhhbXBsZS5jb20iLCJyb2xlIjoiYWRtaW4iLCJleHAiOjE3NTg3OTMyNDgsIm5iZiI6MTc1ODcwNjg0OCwiaWF0IjoxNzU4NzA2ODQ4fQ.-u0-qasY8hDQ77dOncWewvM2JB39y7VdK2eW2HOlHK8"

echo "🚀 Testing Import-Export Backend API"
echo "===================================="
echo ""

# Test 1: Health Check
echo "1. Testing Health Check..."
curl -s "$BASE_URL/health" | jq .
echo ""

# Test 2: Create Supplier
echo "2. Creating Supplier..."
SUPPLIER_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/suppliers" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Tech Supplier Inc",
    "contact_email": "contact@techsupplier.com",
    "contact_phone": "+1-555-0123",
    "address": "123 Tech Street, Tech City, TC 12345"
  }')

echo "$SUPPLIER_RESPONSE" | jq .
SUPPLIER_ID=$(echo "$SUPPLIER_RESPONSE" | jq -r '.id')
echo "Supplier ID: $SUPPLIER_ID"
echo ""

# Test 3: Create Product
echo "3. Creating Product..."
PRODUCT_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/products" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"Laptop Computer\",
    \"description\": \"High-performance laptop\",
    \"sku\": \"LAPTOP-001\",
    \"supplier_id\": \"$SUPPLIER_ID\",
    \"unit_price\": 999.99,
    \"status\": \"active\"
  }")

echo "$PRODUCT_RESPONSE" | jq .
PRODUCT_ID=$(echo "$PRODUCT_RESPONSE" | jq -r '.id')
echo "Product ID: $PRODUCT_ID"
echo ""

# Test 4: Get Products
echo "4. Getting Products..."
curl -s "$BASE_URL/api/v1/products" \
  -H "Authorization: Bearer $JWT_TOKEN" | jq .
echo ""

# Test 5: Create Purchase Order
echo "5. Creating Purchase Order..."
PURCHASE_ORDER_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/purchase-orders" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"order_number\": \"PO-2024-001\",
    \"supplier_id\": \"$SUPPLIER_ID\",
    \"status\": \"pending\",
    \"total_amount\": 1999.98,
    \"notes\": \"Initial purchase order\"
  }")

echo "$PURCHASE_ORDER_RESPONSE" | jq .
PURCHASE_ORDER_ID=$(echo "$PURCHASE_ORDER_RESPONSE" | jq -r '.id')
echo "Purchase Order ID: $PURCHASE_ORDER_ID"
echo ""

# Test 6: Create Order
echo "6. Creating Customer Order..."
ORDER_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/orders" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "order_number": "ORD-2024-001",
    "customer_name": "John Doe",
    "customer_email": "john@example.com",
    "status": "pending",
    "total_amount": 1999.98,
    "notes": "Rush order"
  }')

echo "$ORDER_RESPONSE" | jq .
ORDER_ID=$(echo "$ORDER_RESPONSE" | jq -r '.id')
echo "Order ID: $ORDER_ID"
echo ""

# Test 7: Get Inventory
echo "7. Getting Inventory..."
curl -s "$BASE_URL/api/v1/inventory" \
  -H "Authorization: Bearer $JWT_TOKEN" | jq .
echo ""

# Test 8: Get Low Stock Items
echo "8. Getting Low Stock Items..."
curl -s "$BASE_URL/api/v1/inventory/low-stock" \
  -H "Authorization: Bearer $JWT_TOKEN" | jq .
echo ""

# Test 9: Get Reports
echo "9. Getting Inventory Summary..."
curl -s "$BASE_URL/api/v1/reports/inventory-summary" \
  -H "Authorization: Bearer $JWT_TOKEN" | jq .
echo ""

echo "✅ API Testing Complete!"
echo ""
echo "📊 Summary:"
echo "- Supplier ID: $SUPPLIER_ID"
echo "- Product ID: $PRODUCT_ID"
echo "- Purchase Order ID: $PURCHASE_ORDER_ID"
echo "- Order ID: $ORDER_ID"
echo ""
echo "🔗 Access Points:"
echo "- API: $BASE_URL"
echo "- Health Check: $BASE_URL/health"
echo "- Database Web UI: http://localhost:8081"
echo ""
echo "🔑 JWT Token (valid for 24 hours):"
echo "$JWT_TOKEN"
