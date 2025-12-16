#!/bin/bash

# =============================================================================
# Batch Ingest Polling Test Script
# =============================================================================
# This script tests the complete batch ingest flow:
# 1. Login to get auth token
# 2. Start batch ingest job
# 3. Poll job status until completion
# 4. Verify final state
# =============================================================================

set -e

# Configuration
API_BASE_URL="${API_BASE_URL:-http://localhost:3001}"
TEST_EMAIL="${TEST_EMAIL:-test@example.com}"
TEST_PASSWORD="${TEST_PASSWORD:-password123}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

echo "=============================================="
echo "  Batch Ingest Polling Test"
echo "=============================================="
echo "API URL: $API_BASE_URL"
echo "Test Email: $TEST_EMAIL"
echo "=============================================="

# Step 1: Login to get auth token
log_info "Step 1: Logging in..."

LOGIN_RESPONSE=$(curl -s -X POST "$API_BASE_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\": \"$TEST_EMAIL\", \"password\": \"$TEST_PASSWORD\"}")

# Extract token from response
ACCESS_TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.data.access_token // .access_token // empty')

if [ -z "$ACCESS_TOKEN" ] || [ "$ACCESS_TOKEN" == "null" ]; then
  log_error "Failed to login. Response:"
  echo "$LOGIN_RESPONSE" | jq .
  
  log_warning "Trying to register a new user..."
  
  REGISTER_RESPONSE=$(curl -s -X POST "$API_BASE_URL/api/v1/auth/register" \
    -H "Content-Type: application/json" \
    -d "{\"email\": \"$TEST_EMAIL\", \"password\": \"$TEST_PASSWORD\", \"name\": \"Test User\"}")
  
  echo "$REGISTER_RESPONSE" | jq .
  
  # Try login again
  LOGIN_RESPONSE=$(curl -s -X POST "$API_BASE_URL/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"email\": \"$TEST_EMAIL\", \"password\": \"$TEST_PASSWORD\"}")
  
  ACCESS_TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.data.access_token // .access_token // empty')
  
  if [ -z "$ACCESS_TOKEN" ] || [ "$ACCESS_TOKEN" == "null" ]; then
    log_error "Still cannot login. Please check credentials."
    exit 1
  fi
fi

log_success "Logged in successfully"
echo "Token: ${ACCESS_TOKEN:0:50}..."

# Step 2: Get a subject to use for testing
log_info "Step 2: Finding a subject..."

SUBJECTS_RESPONSE=$(curl -s -X GET "$API_BASE_URL/api/v1/subjects?limit=1" \
  -H "Authorization: Bearer $ACCESS_TOKEN")

SUBJECT_ID=$(echo "$SUBJECTS_RESPONSE" | jq -r '.data.subjects[0].id // .data[0].id // empty')

if [ -z "$SUBJECT_ID" ] || [ "$SUBJECT_ID" == "null" ]; then
  log_warning "No subjects found. Checking courses..."
  
  COURSES_RESPONSE=$(curl -s -X GET "$API_BASE_URL/api/v1/courses" \
    -H "Authorization: Bearer $ACCESS_TOKEN")
  
  echo "Courses response:"
  echo "$COURSES_RESPONSE" | jq '.data[:2]'
  
  log_error "Please create a subject first, or set SUBJECT_ID environment variable"
  echo "Example: SUBJECT_ID=1 ./test_batch_ingest_polling.sh"
  exit 1
fi

log_success "Using Subject ID: $SUBJECT_ID"

# Step 3: Start batch ingest
log_info "Step 3: Starting batch ingest..."

# Use a real PDF URL for testing (or a mock server)
PDF_URL="${PDF_URL:-https://www.w3.org/WAI/WCAG21/Techniques/pdf/img/table-word.pdf}"

BATCH_REQUEST='{
  "papers": [
    {
      "pdf_url": "'"$PDF_URL"'",
      "title": "Test-Paper-Dec-2024",
      "year": 2024,
      "month": "December",
      "exam_type": "End Semester",
      "source_name": "Test Script"
    }
  ]
}'

echo "Request body:"
echo "$BATCH_REQUEST" | jq .

BATCH_RESPONSE=$(curl -s -X POST "$API_BASE_URL/api/v1/subjects/$SUBJECT_ID/pyqs/batch-ingest" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d "$BATCH_REQUEST")

echo "Batch ingest response:"
echo "$BATCH_RESPONSE" | jq .

JOB_ID=$(echo "$BATCH_RESPONSE" | jq -r '.data.job_id // empty')

if [ -z "$JOB_ID" ] || [ "$JOB_ID" == "null" ]; then
  log_error "Failed to start batch ingest"
  exit 1
fi

log_success "Job started: ID=$JOB_ID"

# Step 4: Poll job status
log_info "Step 4: Polling job status..."

MAX_POLLS=60
POLL_INTERVAL=2
POLL_COUNT=0

while [ $POLL_COUNT -lt $MAX_POLLS ]; do
  POLL_COUNT=$((POLL_COUNT + 1))
  
  JOB_STATUS_RESPONSE=$(curl -s -X GET "$API_BASE_URL/api/v1/indexing-jobs/$JOB_ID" \
    -H "Authorization: Bearer $ACCESS_TOKEN")
  
  STATUS=$(echo "$JOB_STATUS_RESPONSE" | jq -r '.data.status // empty')
  PROGRESS=$(echo "$JOB_STATUS_RESPONSE" | jq -r '.data.progress // 0')
  COMPLETED=$(echo "$JOB_STATUS_RESPONSE" | jq -r '.data.completed_items // 0')
  FAILED=$(echo "$JOB_STATUS_RESPONSE" | jq -r '.data.failed_items // 0')
  TOTAL=$(echo "$JOB_STATUS_RESPONSE" | jq -r '.data.total_items // 0')
  
  echo -e "${BLUE}[Poll $POLL_COUNT]${NC} Status: $STATUS | Progress: $PROGRESS% | Completed: $COMPLETED | Failed: $FAILED | Total: $TOTAL"
  
  # Check if job is complete
  if [ "$STATUS" == "completed" ] || [ "$STATUS" == "failed" ] || [ "$STATUS" == "partially_completed" ] || [ "$STATUS" == "cancelled" ]; then
    log_success "Job completed with status: $STATUS"
    break
  fi
  
  sleep $POLL_INTERVAL
done

if [ $POLL_COUNT -ge $MAX_POLLS ]; then
  log_warning "Polling timed out after $MAX_POLLS attempts"
fi

# Step 5: Get final job status
log_info "Step 5: Final job status..."

FINAL_STATUS=$(curl -s -X GET "$API_BASE_URL/api/v1/indexing-jobs/$JOB_ID" \
  -H "Authorization: Bearer $ACCESS_TOKEN")

echo "Final job status:"
echo "$FINAL_STATUS" | jq .

# Step 6: Check notifications
log_info "Step 6: Checking notifications..."

NOTIFICATIONS=$(curl -s -X GET "$API_BASE_URL/api/v1/notifications?limit=5" \
  -H "Authorization: Bearer $ACCESS_TOKEN")

echo "Recent notifications:"
echo "$NOTIFICATIONS" | jq '.data.notifications[:3]'

# Summary
echo ""
echo "=============================================="
echo "  TEST SUMMARY"
echo "=============================================="
echo "Job ID: $JOB_ID"
echo "Final Status: $(echo "$FINAL_STATUS" | jq -r '.data.status')"
echo "Completed Items: $(echo "$FINAL_STATUS" | jq -r '.data.completed_items')"
echo "Failed Items: $(echo "$FINAL_STATUS" | jq -r '.data.failed_items')"
echo "Total Items: $(echo "$FINAL_STATUS" | jq -r '.data.total_items')"
echo "=============================================="

# Check if test passed
FINAL_JOB_STATUS=$(echo "$FINAL_STATUS" | jq -r '.data.status')
if [ "$FINAL_JOB_STATUS" == "completed" ]; then
  log_success "TEST PASSED - All items ingested successfully"
  exit 0
elif [ "$FINAL_JOB_STATUS" == "partially_completed" ]; then
  log_warning "TEST PARTIALLY PASSED - Some items failed"
  exit 0
else
  log_error "TEST FAILED - Job status: $FINAL_JOB_STATUS"
  exit 1
fi
