#!/bin/bash
# Script to find broken responses in production logs
# Usage: ./find_broken_responses.sh [search_term] [context_lines]
#
# Examples:
#   ./find_broken_responses.sh "List all PYQ" 100
#   ./find_broken_responses.sh "Scanner error" 50
#   ./find_broken_responses.sh                      # defaults to searching for errors

SEARCH_TERM="${1:-Scanner error|context deadline exceeded|timeout|failed to stream}"
CONTEXT_LINES="${2:-100}"
OUTPUT_DIR="/tmp/study-woods-logs"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
SSH_KEY="$HOME/.ssh/study-in-woods"
SSH_HOST="root@167.71.233.64"
DOCKER_CONTAINER="study-woods-api"

# Create output directory
mkdir -p "$OUTPUT_DIR"

echo "=============================================="
echo "Study-in-Woods Production Log Search"
echo "=============================================="
echo "Search Term: $SEARCH_TERM"
echo "Context Lines: $CONTEXT_LINES (before and after)"
echo "Output Directory: $OUTPUT_DIR"
echo "Timestamp: $TIMESTAMP"
echo "=============================================="

# Function to search logs
search_logs() {
    local pattern="$1"
    local output_file="$2"
    local ctx="${3:-100}"
    
    echo "Searching for: $pattern"
    echo "Saving to: $output_file"
    
    ssh -i "$SSH_KEY" "$SSH_HOST" \
        "docker logs $DOCKER_CONTAINER 2>&1 | grep -i -E '$pattern' -B $ctx -A $ctx" \
        > "$output_file" 2>&1
    
    local lines=$(wc -l < "$output_file" 2>/dev/null || echo "0")
    echo "Found $lines lines"
    echo ""
}

# 1. Search for the specific term provided
OUTPUT_FILE_MAIN="$OUTPUT_DIR/search_${TIMESTAMP}.txt"
search_logs "$SEARCH_TERM" "$OUTPUT_FILE_MAIN" "$CONTEXT_LINES"

# 2. Search for all streaming errors
OUTPUT_FILE_ERRORS="$OUTPUT_DIR/streaming_errors_${TIMESTAMP}.txt"
search_logs "Scanner error|stream error|failed to|context deadline" "$OUTPUT_FILE_ERRORS" 50

# 3. Search for "List all PYQ" specifically
OUTPUT_FILE_PYQ="$OUTPUT_DIR/list_all_pyq_${TIMESTAMP}.txt"
search_logs "List all [Pp][Yy][Qq]" "$OUTPUT_FILE_PYQ" 150

# 4. Extract unique error types
OUTPUT_FILE_SUMMARY="$OUTPUT_DIR/error_summary_${TIMESTAMP}.txt"
echo "Extracting error summary..."
ssh -i "$SSH_KEY" "$SSH_HOST" \
    "docker logs $DOCKER_CONTAINER 2>&1 | grep -i -E 'error|failed|timeout|panic' | sort | uniq -c | sort -rn | head -50" \
    > "$OUTPUT_FILE_SUMMARY" 2>&1

echo "=============================================="
echo "Output Files Created:"
echo "=============================================="
echo "1. Main search results:    $OUTPUT_FILE_MAIN"
echo "2. Streaming errors:       $OUTPUT_FILE_ERRORS"
echo "3. 'List all PYQ' queries: $OUTPUT_FILE_PYQ"
echo "4. Error summary:          $OUTPUT_FILE_SUMMARY"
echo ""
echo "=============================================="
echo "Quick Analysis:"
echo "=============================================="

# Show error counts
echo ""
echo "Top errors found:"
head -20 "$OUTPUT_FILE_SUMMARY" 2>/dev/null || echo "No summary available"

echo ""
echo "=============================================="
echo "To view full logs:"
echo "=============================================="
echo "cat $OUTPUT_FILE_MAIN"
echo "cat $OUTPUT_FILE_PYQ"
echo "less $OUTPUT_FILE_ERRORS"
echo ""
