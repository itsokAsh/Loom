#!/bin/bash

# Test script for email node implementation
# This script verifies that all components are working correctly

set -e

echo "=== Email Node Implementation Test ==="
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if .env file exists
if [ ! -f .env ]; then
    echo -e "${YELLOW}Warning: .env file not found. Copy .env.example to .env and add your SendGrid API key.${NC}"
    exit 1
fi

# Source .env file
source .env

# Check if SENDGRID_API_KEY is set
if [ -z "$SENDGRID_API_KEY" ]; then
    echo -e "${RED}Error: SENDGRID_API_KEY not set in .env file${NC}"
    exit 1
fi

echo -e "${GREEN}✓${NC} SendGrid API key found"

# Build all services
echo ""
echo "Building services..."
echo ""

echo "Building node-worker-pool..."
cd node-worker-pool
go build ./...
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓${NC} node-worker-pool builds successfully"
else
    echo -e "${RED}✗${NC} node-worker-pool build failed"
    exit 1
fi
cd ..

echo "Building orchestration-engine..."
cd orchestration-engine
go build ./...
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓${NC} orchestration-engine builds successfully"
else
    echo -e "${RED}✗${NC} orchestration-engine build failed"
    exit 1
fi
cd ..

echo "Building trigger-api..."
cd trigger-api
go build ./...
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓${NC} trigger-api builds successfully"
else
    echo -e "${RED}✗${NC} trigger-api build failed"
    exit 1
fi
cd ..

# Check if required files exist
echo ""
echo "Checking implementation files..."
echo ""

FILES=(
    "node-worker-pool/internal/executor/hardened_http_engine.go"
    "node-worker-pool/internal/executor/ip_blocklist.go"
    "node-worker-pool/internal/nodes/email.go"
    "node-worker-pool/internal/secrets/store.go"
    "orchestration-engine/migrations/004_email_counter.up.sql"
    "orchestration-engine/migrations/004_email_counter.down.sql"
    "examples/user_onboarding_workflow.json"
)

for file in "${FILES[@]}"; do
    if [ -f "$file" ]; then
        echo -e "${GREEN}✓${NC} $file exists"
    else
        echo -e "${RED}✗${NC} $file missing"
        exit 1
    fi
done

echo ""
echo -e "${GREEN}=== All checks passed! ===${NC}"
echo ""
echo "Next steps:"
echo "1. Start services: docker-compose up -d"
echo "2. Wait for services to be healthy"
echo "3. Create a workflow using examples/user_onboarding_workflow.json"
echo "4. Trigger the workflow with a webhook"
echo "5. Check that email is sent successfully"
echo ""
echo "To test email sending:"
echo "  1. Ensure your SendGrid API key is valid"
echo "  2. Verify the 'from' email domain in SendGrid"
echo "  3. Check SendGrid Activity dashboard after triggering workflow"
