#!/bin/bash

# Chat Testing Script for Session 12
# This script sends messages and captures responses for analysis

TOKEN=$(cat /tmp/auth_token)
SESSION_ID=12
API_URL="http://localhost:8080"

# Function to send a message and get response (non-streaming for easier capture)
send_message() {
    local message="$1"
    local turn="$2"
    
    echo "=============================================="
    echo "TURN $turn"
    echo "=============================================="
    echo "USER: $message"
    echo ""
    
    # Send message (non-streaming)
    response=$(curl -s -X POST "$API_URL/api/v1/chat/sessions/$SESSION_ID/messages" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d "{\"content\": \"$message\", \"stream\": false}")
    
    # Extract assistant response
    assistant_content=$(echo "$response" | jq -r '.data.assistant_message.content // .error.message // "ERROR"')
    citations=$(echo "$response" | jq -r '.data.assistant_message.citations // []')
    
    echo "ASSISTANT:"
    echo "$assistant_content" | head -50
    echo ""
    echo "CITATIONS: $(echo "$citations" | jq -r 'length') sources"
    echo ""
    
    # Return response for analysis
    echo "$response"
}

# Main test function
run_test() {
    echo "Starting Chat Test - $(date)"
    echo ""
    
    # Test message
    send_message "$1" "$2"
}

# If called with arguments, run single test
if [ -n "$1" ]; then
    run_test "$1" "${2:-1}"
fi
