#!/bin/bash

# Test script for syllabus extraction performance
# Tests the optimized chunked extraction with performance metrics

set -e

echo "🧪 Testing Syllabus Extraction Performance"
echo "=========================================="
echo ""

# Configuration
API_URL="http://localhost:8080"
EMAIL="${ADMIN_EMAIL:-admin@example.com}"
PASSWORD="${ADMIN_PASSWORD:-ChangeMe123!}"
PDF_FILE="${1:-frm_download_file.pdf}"

# Check if PDF exists
if [ ! -f "$PDF_FILE" ]; then
    echo "❌ Error: PDF file not found: $PDF_FILE"
    echo "Usage: $0 [pdf_file_path]"
    exit 1
fi

echo "📄 PDF File: $PDF_FILE"
echo ""

# Step 1: Login and get token
echo "🔐 Step 1: Authenticating..."
LOGIN_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\": \"$EMAIL\", \"password\": \"$PASSWORD\"}")

TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.data.access_token')

if [ "$TOKEN" == "null" ] || [ -z "$TOKEN" ]; then
    echo "❌ Authentication failed!"
    echo "Response: $LOGIN_RESPONSE"
    exit 1
fi

echo "✅ Authenticated successfully"
echo ""

# Step 2: Upload PDF
echo "📤 Step 2: Uploading PDF..."
UPLOAD_START=$(date +%s)

UPLOAD_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/semesters/1/syllabus/upload" \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@$PDF_FILE")

UPLOAD_END=$(date +%s)
UPLOAD_TIME=$((UPLOAD_END - UPLOAD_START))

DOCUMENT_ID=$(echo "$UPLOAD_RESPONSE" | jq -r '.data.id')

if [ "$DOCUMENT_ID" == "null" ] || [ -z "$DOCUMENT_ID" ]; then
    echo "❌ Upload failed!"
    echo "Response: $UPLOAD_RESPONSE"
    exit 1
fi

echo "✅ PDF uploaded successfully (Document ID: $DOCUMENT_ID)"
echo "⏱️  Upload time: ${UPLOAD_TIME}s"
echo ""

# Step 3: Extract syllabus
echo "🤖 Step 3: Extracting syllabus (this may take 1-2 minutes)..."
EXTRACT_START=$(date +%s)

EXTRACT_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/documents/$DOCUMENT_ID/extract-syllabus" \
  -H "Authorization: Bearer $TOKEN")

EXTRACT_END=$(date +%s)
EXTRACT_TIME=$((EXTRACT_END - EXTRACT_START))

# Check if extraction succeeded
SUCCESS=$(echo "$EXTRACT_RESPONSE" | jq -r '.success')

if [ "$SUCCESS" != "true" ]; then
    echo "❌ Extraction failed!"
    echo "Response: $EXTRACT_RESPONSE"
    exit 1
fi

echo "✅ Extraction completed successfully"
echo "⏱️  Extraction time: ${EXTRACT_TIME}s"
echo ""

# Step 4: Get extraction statistics
echo "📊 Step 4: Gathering statistics..."

# Count subjects
SUBJECT_COUNT=$(echo "$EXTRACT_RESPONSE" | jq -r '.data | length')

# Get detailed stats from database
DB_STATS=$(docker exec study-woods-postgres psql -U postgres -d study_in_woods -t -c "
SELECT 
    COUNT(DISTINCT s.id) as subjects,
    COUNT(DISTINCT su.id) as units,
    COUNT(DISTINCT st.id) as topics,
    COUNT(DISTINCT br.id) as books
FROM syllabuses s
LEFT JOIN syllabus_units su ON s.id = su.syllabus_id
LEFT JOIN syllabus_topics st ON su.id = st.unit_id
LEFT JOIN book_references br ON s.id = br.syllabus_id
WHERE s.document_id = $DOCUMENT_ID;
")

SUBJECTS=$(echo "$DB_STATS" | awk '{print $1}')
UNITS=$(echo "$DB_STATS" | awk '{print $3}')
TOPICS=$(echo "$DB_STATS" | awk '{print $5}')
BOOKS=$(echo "$DB_STATS" | awk '{print $7}')

echo ""
echo "📈 PERFORMANCE RESULTS"
echo "======================"
echo "⏱️  Total Time: ${EXTRACT_TIME}s"
echo "📄 PDF File: $PDF_FILE"
echo ""
echo "📊 Extraction Statistics:"
echo "   • Subjects: $SUBJECTS"
echo "   • Units: $UNITS"
echo "   • Topics: $TOPICS"
echo "   • Books: $BOOKS"
echo ""

# Calculate topics per subject
if [ "$SUBJECTS" -gt 0 ]; then
    TOPICS_PER_SUBJECT=$((TOPICS / SUBJECTS))
    echo "   • Avg Topics/Subject: $TOPICS_PER_SUBJECT"
fi

echo ""

# Step 5: Sample topic quality check
echo "🔍 Step 5: Topic Quality Sample (first 10 topics)..."
docker exec study-woods-postgres psql -U postgres -d study_in_woods -c "
SELECT 
    s.subject_code,
    su.unit_number,
    st.topic_number,
    st.title
FROM syllabus_topics st
JOIN syllabus_units su ON st.unit_id = su.id
JOIN syllabuses s ON su.syllabus_id = s.id
WHERE s.document_id = $DOCUMENT_ID
ORDER BY s.id, su.unit_number, st.topic_number
LIMIT 10;
"

echo ""
echo "✅ Test completed successfully!"
echo ""
echo "💡 Performance Benchmarks:"
echo "   • Target: < 90s total extraction time"
echo "   • Target: > 30 topics per syllabus"
echo "   • Your result: ${EXTRACT_TIME}s, $TOPICS topics total"
echo ""

# Performance evaluation
if [ "$EXTRACT_TIME" -lt 90 ]; then
    echo "🎉 EXCELLENT: Extraction time is under 90 seconds!"
else
    echo "⚠️  WARNING: Extraction time exceeded 90 seconds"
fi

if [ "$TOPICS" -gt 30 ]; then
    echo "🎉 EXCELLENT: Topic extraction quality is high!"
else
    echo "⚠️  WARNING: Low topic count, check extraction quality"
fi

echo ""
