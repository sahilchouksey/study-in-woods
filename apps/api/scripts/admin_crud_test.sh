#!/bin/bash

# =============================================================================
# Study In Woods - Admin CRUD Test Script
# =============================================================================
# This script tests all admin CRUD operations for the educational hierarchy:
# Universities -> Courses -> Semesters -> Subjects
#
# Prerequisites:
# - API server running on localhost:8080
# - Database seeded with initial data
# - jq installed for JSON parsing
# =============================================================================

set -e

# Configuration
API_BASE="http://localhost:8080"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@example.com}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-ChangeMe123!}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Counters
TESTS_PASSED=0
TESTS_FAILED=0

# =============================================================================
# Helper Functions
# =============================================================================

print_header() {
    echo ""
    echo -e "${BLUE}=============================================================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}=============================================================================${NC}"
}

print_test() {
    echo -e "${YELLOW}[TEST]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
    ((TESTS_PASSED++))
}

print_failure() {
    echo -e "${RED}[FAIL]${NC} $1"
    ((TESTS_FAILED++))
}

print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

# Make API request and return response
api_request() {
    local method=$1
    local endpoint=$2
    local data=$3
    local token=$4
    
    local headers="-H 'Content-Type: application/json'"
    if [ -n "$token" ]; then
        headers="$headers -H 'Authorization: Bearer $token'"
    fi
    
    if [ -n "$data" ]; then
        eval "curl -s -X $method '$API_BASE$endpoint' $headers -d '$data'"
    else
        eval "curl -s -X $method '$API_BASE$endpoint' $headers"
    fi
}

# Check if response is successful
check_success() {
    local response=$1
    local success=$(echo "$response" | jq -r '.success // false')
    [ "$success" = "true" ]
}

# Extract data from response
get_data() {
    local response=$1
    local field=$2
    echo "$response" | jq -r ".data.$field // .data[0].$field // empty"
}

# Extract ID from response (handles both single and array responses)
get_id() {
    local response=$1
    echo "$response" | jq -r '.data.id // .data[0].id // empty'
}

# =============================================================================
# Step 1: Admin Login
# =============================================================================

print_header "STEP 1: Admin Login"

print_test "Logging in as admin..."

LOGIN_RESPONSE=$(api_request "POST" "/api/v1/auth/login" "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}")

if check_success "$LOGIN_RESPONSE"; then
    ACCESS_TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.data.access_token')
    if [ -n "$ACCESS_TOKEN" ] && [ "$ACCESS_TOKEN" != "null" ]; then
        print_success "Admin login successful"
        print_info "Token: ${ACCESS_TOKEN:0:50}..."
    else
        print_failure "No access token received"
        echo "$LOGIN_RESPONSE" | jq .
        exit 1
    fi
else
    print_failure "Admin login failed"
    echo "$LOGIN_RESPONSE" | jq .
    exit 1
fi

# =============================================================================
# Step 2: List Existing Data
# =============================================================================

print_header "STEP 2: List Existing Data"

# List Universities
print_test "Listing all universities..."
UNIVERSITIES_RESPONSE=$(api_request "GET" "/api/v1/universities" "" "$ACCESS_TOKEN")

if check_success "$UNIVERSITIES_RESPONSE"; then
    UNIVERSITY_COUNT=$(echo "$UNIVERSITIES_RESPONSE" | jq '.data | length')
    print_success "Listed $UNIVERSITY_COUNT universities"
    echo "$UNIVERSITIES_RESPONSE" | jq '.data[] | {id, name, code}'
else
    print_failure "Failed to list universities"
    echo "$UNIVERSITIES_RESPONSE" | jq .
fi

# Get first university ID for further tests
EXISTING_UNIVERSITY_ID=$(echo "$UNIVERSITIES_RESPONSE" | jq -r '.data[0].id // empty')
print_info "Using University ID: $EXISTING_UNIVERSITY_ID"

# List Courses for the university
if [ -n "$EXISTING_UNIVERSITY_ID" ]; then
    print_test "Listing courses for university $EXISTING_UNIVERSITY_ID..."
    COURSES_RESPONSE=$(api_request "GET" "/api/v1/courses?university_id=$EXISTING_UNIVERSITY_ID" "" "$ACCESS_TOKEN")
    
    if check_success "$COURSES_RESPONSE"; then
        COURSE_COUNT=$(echo "$COURSES_RESPONSE" | jq '.data | length')
        print_success "Listed $COURSE_COUNT courses"
        echo "$COURSES_RESPONSE" | jq '.data[] | {id, name, code, duration}'
    else
        print_failure "Failed to list courses"
    fi
    
    EXISTING_COURSE_ID=$(echo "$COURSES_RESPONSE" | jq -r '.data[0].id // empty')
    print_info "Using Course ID: $EXISTING_COURSE_ID"
fi

# List Semesters for the course
if [ -n "$EXISTING_COURSE_ID" ]; then
    print_test "Listing semesters for course $EXISTING_COURSE_ID..."
    SEMESTERS_RESPONSE=$(api_request "GET" "/api/v1/courses/$EXISTING_COURSE_ID/semesters" "" "$ACCESS_TOKEN")
    
    if check_success "$SEMESTERS_RESPONSE"; then
        SEMESTER_COUNT=$(echo "$SEMESTERS_RESPONSE" | jq '.data | length')
        print_success "Listed $SEMESTER_COUNT semesters"
        echo "$SEMESTERS_RESPONSE" | jq '.data[] | {id, number, name}'
    else
        print_failure "Failed to list semesters"
    fi
    
    EXISTING_SEMESTER_ID=$(echo "$SEMESTERS_RESPONSE" | jq -r '.data[0].id // empty')
    EXISTING_SEMESTER_NUMBER=$(echo "$SEMESTERS_RESPONSE" | jq -r '.data[0].number // empty')
    print_info "Using Semester ID: $EXISTING_SEMESTER_ID (Number: $EXISTING_SEMESTER_NUMBER)"
fi

# List Subjects for the semester
if [ -n "$EXISTING_SEMESTER_ID" ]; then
    print_test "Listing subjects for semester $EXISTING_SEMESTER_ID..."
    SUBJECTS_RESPONSE=$(api_request "GET" "/api/v1/semesters/$EXISTING_SEMESTER_ID/subjects" "" "$ACCESS_TOKEN")
    
    if check_success "$SUBJECTS_RESPONSE"; then
        SUBJECT_COUNT=$(echo "$SUBJECTS_RESPONSE" | jq '.data | length')
        print_success "Listed $SUBJECT_COUNT subjects"
        echo "$SUBJECTS_RESPONSE" | jq '.data[] | {id, name, code, credits}'
    else
        print_failure "Failed to list subjects"
    fi
    
    EXISTING_SUBJECT_ID=$(echo "$SUBJECTS_RESPONSE" | jq -r '.data[0].id // empty')
    print_info "Using Subject ID: $EXISTING_SUBJECT_ID"
fi

# =============================================================================
# Step 3: CREATE Operations (Bottom-up to avoid dependency issues)
# =============================================================================

print_header "STEP 3: CREATE Operations"

# 3.1 Create a new University
print_test "Creating new university..."
CREATE_UNI_RESPONSE=$(api_request "POST" "/api/v1/universities" '{
    "name": "Test University",
    "code": "TESTU",
    "location": "Test City, Test State",
    "website": "https://testuniversity.edu"
}' "$ACCESS_TOKEN")

if check_success "$CREATE_UNI_RESPONSE"; then
    NEW_UNIVERSITY_ID=$(get_id "$CREATE_UNI_RESPONSE")
    print_success "Created university with ID: $NEW_UNIVERSITY_ID"
    echo "$CREATE_UNI_RESPONSE" | jq '.data | {id, name, code, location}'
else
    print_failure "Failed to create university"
    echo "$CREATE_UNI_RESPONSE" | jq .
fi

# 3.2 Create a new Course
if [ -n "$NEW_UNIVERSITY_ID" ]; then
    print_test "Creating new course..."
    CREATE_COURSE_RESPONSE=$(api_request "POST" "/api/v1/courses" "{
        \"university_id\": $NEW_UNIVERSITY_ID,
        \"name\": \"Test Bachelor of Science\",
        \"code\": \"TBS\",
        \"description\": \"A test course for validation\",
        \"duration\": 6
    }" "$ACCESS_TOKEN")
    
    if check_success "$CREATE_COURSE_RESPONSE"; then
        NEW_COURSE_ID=$(get_id "$CREATE_COURSE_RESPONSE")
        print_success "Created course with ID: $NEW_COURSE_ID"
        echo "$CREATE_COURSE_RESPONSE" | jq '.data | {id, name, code, duration}'
    else
        print_failure "Failed to create course"
        echo "$CREATE_COURSE_RESPONSE" | jq .
    fi
fi

# 3.3 Create a new Semester
if [ -n "$NEW_COURSE_ID" ]; then
    print_test "Creating new semester..."
    CREATE_SEMESTER_RESPONSE=$(api_request "POST" "/api/v1/courses/$NEW_COURSE_ID/semesters" '{
        "number": 1,
        "name": "First Semester"
    }' "$ACCESS_TOKEN")
    
    if check_success "$CREATE_SEMESTER_RESPONSE"; then
        NEW_SEMESTER_ID=$(get_id "$CREATE_SEMESTER_RESPONSE")
        print_success "Created semester with ID: $NEW_SEMESTER_ID"
        echo "$CREATE_SEMESTER_RESPONSE" | jq '.data | {id, number, name}'
    else
        print_failure "Failed to create semester"
        echo "$CREATE_SEMESTER_RESPONSE" | jq .
    fi
fi

# 3.4 Create a new Subject
if [ -n "$NEW_SEMESTER_ID" ]; then
    print_test "Creating new subject..."
    CREATE_SUBJECT_RESPONSE=$(api_request "POST" "/api/v1/semesters/$NEW_SEMESTER_ID/subjects" '{
        "name": "Test Subject",
        "code": "TS101",
        "description": "A test subject for validation",
        "credits": 4
    }' "$ACCESS_TOKEN")
    
    if check_success "$CREATE_SUBJECT_RESPONSE"; then
        NEW_SUBJECT_ID=$(get_id "$CREATE_SUBJECT_RESPONSE")
        print_success "Created subject with ID: $NEW_SUBJECT_ID"
        echo "$CREATE_SUBJECT_RESPONSE" | jq '.data | {id, name, code, credits}'
    else
        print_failure "Failed to create subject"
        echo "$CREATE_SUBJECT_RESPONSE" | jq .
    fi
fi

# =============================================================================
# Step 4: READ Operations (Verify created data)
# =============================================================================

print_header "STEP 4: READ Operations (Verify Created Data)"

# 4.1 Get the created university
if [ -n "$NEW_UNIVERSITY_ID" ]; then
    print_test "Getting university $NEW_UNIVERSITY_ID..."
    GET_UNI_RESPONSE=$(api_request "GET" "/api/v1/universities/$NEW_UNIVERSITY_ID" "" "$ACCESS_TOKEN")
    
    if check_success "$GET_UNI_RESPONSE"; then
        print_success "Retrieved university"
        echo "$GET_UNI_RESPONSE" | jq '.data | {id, name, code}'
    else
        print_failure "Failed to get university"
    fi
fi

# 4.2 Get the created course
if [ -n "$NEW_COURSE_ID" ]; then
    print_test "Getting course $NEW_COURSE_ID..."
    GET_COURSE_RESPONSE=$(api_request "GET" "/api/v1/courses/$NEW_COURSE_ID" "" "$ACCESS_TOKEN")
    
    if check_success "$GET_COURSE_RESPONSE"; then
        print_success "Retrieved course"
        echo "$GET_COURSE_RESPONSE" | jq '.data | {id, name, code}'
    else
        print_failure "Failed to get course"
    fi
fi

# 4.3 Get semesters for the course
if [ -n "$NEW_COURSE_ID" ]; then
    print_test "Getting semesters for course $NEW_COURSE_ID..."
    GET_SEMESTERS_RESPONSE=$(api_request "GET" "/api/v1/courses/$NEW_COURSE_ID/semesters" "" "$ACCESS_TOKEN")
    
    if check_success "$GET_SEMESTERS_RESPONSE"; then
        print_success "Retrieved semesters"
        echo "$GET_SEMESTERS_RESPONSE" | jq '.data[] | {id, number, name}'
    else
        print_failure "Failed to get semesters"
    fi
fi

# 4.4 Get subjects for the semester
if [ -n "$NEW_SEMESTER_ID" ]; then
    print_test "Getting subjects for semester $NEW_SEMESTER_ID..."
    GET_SUBJECTS_RESPONSE=$(api_request "GET" "/api/v1/semesters/$NEW_SEMESTER_ID/subjects" "" "$ACCESS_TOKEN")
    
    if check_success "$GET_SUBJECTS_RESPONSE"; then
        print_success "Retrieved subjects"
        echo "$GET_SUBJECTS_RESPONSE" | jq '.data[] | {id, name, code}'
    else
        print_failure "Failed to get subjects"
    fi
fi

# =============================================================================
# Step 5: UPDATE Operations
# =============================================================================

print_header "STEP 5: UPDATE Operations"

# 5.1 Update the university
if [ -n "$NEW_UNIVERSITY_ID" ]; then
    print_test "Updating university $NEW_UNIVERSITY_ID..."
    UPDATE_UNI_RESPONSE=$(api_request "PUT" "/api/v1/universities/$NEW_UNIVERSITY_ID" '{
        "name": "Updated Test University",
        "location": "Updated City, Updated State"
    }' "$ACCESS_TOKEN")
    
    if check_success "$UPDATE_UNI_RESPONSE"; then
        print_success "Updated university"
        echo "$UPDATE_UNI_RESPONSE" | jq '.data | {id, name, location}'
    else
        print_failure "Failed to update university"
        echo "$UPDATE_UNI_RESPONSE" | jq .
    fi
fi

# 5.2 Update the course
if [ -n "$NEW_COURSE_ID" ]; then
    print_test "Updating course $NEW_COURSE_ID..."
    UPDATE_COURSE_RESPONSE=$(api_request "PUT" "/api/v1/courses/$NEW_COURSE_ID" '{
        "name": "Updated Test Bachelor of Science",
        "description": "Updated description for the test course",
        "duration": 8
    }' "$ACCESS_TOKEN")
    
    if check_success "$UPDATE_COURSE_RESPONSE"; then
        print_success "Updated course"
        echo "$UPDATE_COURSE_RESPONSE" | jq '.data | {id, name, description, duration}'
    else
        print_failure "Failed to update course"
        echo "$UPDATE_COURSE_RESPONSE" | jq .
    fi
fi

# 5.3 Update the semester
if [ -n "$NEW_COURSE_ID" ]; then
    print_test "Updating semester 1 of course $NEW_COURSE_ID..."
    UPDATE_SEMESTER_RESPONSE=$(api_request "PUT" "/api/v1/courses/$NEW_COURSE_ID/semesters/1" '{
        "name": "Updated First Semester"
    }' "$ACCESS_TOKEN")
    
    if check_success "$UPDATE_SEMESTER_RESPONSE"; then
        print_success "Updated semester"
        echo "$UPDATE_SEMESTER_RESPONSE" | jq '.data | {id, number, name}'
    else
        print_failure "Failed to update semester"
        echo "$UPDATE_SEMESTER_RESPONSE" | jq .
    fi
fi

# 5.4 Update the subject
if [ -n "$NEW_SEMESTER_ID" ] && [ -n "$NEW_SUBJECT_ID" ]; then
    print_test "Updating subject $NEW_SUBJECT_ID..."
    UPDATE_SUBJECT_RESPONSE=$(api_request "PUT" "/api/v1/semesters/$NEW_SEMESTER_ID/subjects/$NEW_SUBJECT_ID" '{
        "name": "Updated Test Subject",
        "description": "Updated description",
        "credits": 5
    }' "$ACCESS_TOKEN")
    
    if check_success "$UPDATE_SUBJECT_RESPONSE"; then
        print_success "Updated subject"
        echo "$UPDATE_SUBJECT_RESPONSE" | jq '.data | {id, name, credits}'
    else
        print_failure "Failed to update subject"
        echo "$UPDATE_SUBJECT_RESPONSE" | jq .
    fi
fi

# =============================================================================
# Step 6: DELETE Operations (Top-down to respect dependencies)
# =============================================================================

print_header "STEP 6: DELETE Operations"

# 6.1 Try to delete course with semesters (should fail)
if [ -n "$NEW_COURSE_ID" ]; then
    print_test "Attempting to delete course with existing semesters (should fail)..."
    DELETE_COURSE_FAIL_RESPONSE=$(api_request "DELETE" "/api/v1/courses/$NEW_COURSE_ID" "" "$ACCESS_TOKEN")
    
    if ! check_success "$DELETE_COURSE_FAIL_RESPONSE"; then
        print_success "Correctly prevented deletion of course with semesters"
        echo "$DELETE_COURSE_FAIL_RESPONSE" | jq '.error'
    else
        print_failure "Should have failed to delete course with semesters"
    fi
fi

# 6.2 Delete subject first
if [ -n "$NEW_SEMESTER_ID" ] && [ -n "$NEW_SUBJECT_ID" ]; then
    print_test "Deleting subject $NEW_SUBJECT_ID..."
    DELETE_SUBJECT_RESPONSE=$(api_request "DELETE" "/api/v1/semesters/$NEW_SEMESTER_ID/subjects/$NEW_SUBJECT_ID" "" "$ACCESS_TOKEN")
    
    if check_success "$DELETE_SUBJECT_RESPONSE"; then
        print_success "Deleted subject"
    else
        print_failure "Failed to delete subject"
        echo "$DELETE_SUBJECT_RESPONSE" | jq .
    fi
fi

# 6.3 Delete semester
if [ -n "$NEW_COURSE_ID" ]; then
    print_test "Deleting semester 1 of course $NEW_COURSE_ID..."
    DELETE_SEMESTER_RESPONSE=$(api_request "DELETE" "/api/v1/courses/$NEW_COURSE_ID/semesters/1" "" "$ACCESS_TOKEN")
    
    if check_success "$DELETE_SEMESTER_RESPONSE"; then
        print_success "Deleted semester"
    else
        print_failure "Failed to delete semester"
        echo "$DELETE_SEMESTER_RESPONSE" | jq .
    fi
fi

# 6.4 Delete course (should work now)
if [ -n "$NEW_COURSE_ID" ]; then
    print_test "Deleting course $NEW_COURSE_ID..."
    DELETE_COURSE_RESPONSE=$(api_request "DELETE" "/api/v1/courses/$NEW_COURSE_ID" "" "$ACCESS_TOKEN")
    
    if check_success "$DELETE_COURSE_RESPONSE"; then
        print_success "Deleted course"
    else
        print_failure "Failed to delete course"
        echo "$DELETE_COURSE_RESPONSE" | jq .
    fi
fi

# 6.5 Delete university (should work now)
if [ -n "$NEW_UNIVERSITY_ID" ]; then
    print_test "Deleting university $NEW_UNIVERSITY_ID..."
    DELETE_UNI_RESPONSE=$(api_request "DELETE" "/api/v1/universities/$NEW_UNIVERSITY_ID" "" "$ACCESS_TOKEN")
    
    if check_success "$DELETE_UNI_RESPONSE"; then
        print_success "Deleted university"
    else
        print_failure "Failed to delete university"
        echo "$DELETE_UNI_RESPONSE" | jq .
    fi
fi

# =============================================================================
# Step 7: Verify Deletions
# =============================================================================

print_header "STEP 7: Verify Deletions"

# Verify university is deleted
if [ -n "$NEW_UNIVERSITY_ID" ]; then
    print_test "Verifying university $NEW_UNIVERSITY_ID is deleted..."
    VERIFY_UNI_RESPONSE=$(api_request "GET" "/api/v1/universities/$NEW_UNIVERSITY_ID" "" "$ACCESS_TOKEN")
    
    if ! check_success "$VERIFY_UNI_RESPONSE"; then
        print_success "University correctly not found (deleted)"
    else
        print_failure "University should have been deleted"
    fi
fi

# Verify course is deleted
if [ -n "$NEW_COURSE_ID" ]; then
    print_test "Verifying course $NEW_COURSE_ID is deleted..."
    VERIFY_COURSE_RESPONSE=$(api_request "GET" "/api/v1/courses/$NEW_COURSE_ID" "" "$ACCESS_TOKEN")
    
    if ! check_success "$VERIFY_COURSE_RESPONSE"; then
        print_success "Course correctly not found (deleted)"
    else
        print_failure "Course should have been deleted"
    fi
fi

# =============================================================================
# Step 8: Test Edge Cases
# =============================================================================

print_header "STEP 8: Edge Cases & Error Handling"

# 8.1 Try to create duplicate university code
print_test "Attempting to create university with duplicate code..."
DUPLICATE_UNI_RESPONSE=$(api_request "POST" "/api/v1/universities" '{
    "name": "Duplicate University",
    "code": "AKTU",
    "location": "Somewhere"
}' "$ACCESS_TOKEN")

if ! check_success "$DUPLICATE_UNI_RESPONSE"; then
    print_success "Correctly rejected duplicate university code"
else
    print_failure "Should have rejected duplicate university code"
    # Clean up if it was created
    DUP_ID=$(get_id "$DUPLICATE_UNI_RESPONSE")
    if [ -n "$DUP_ID" ]; then
        api_request "DELETE" "/api/v1/universities/$DUP_ID" "" "$ACCESS_TOKEN" > /dev/null
    fi
fi

# 8.2 Try to get non-existent resource
print_test "Attempting to get non-existent university..."
NONEXISTENT_RESPONSE=$(api_request "GET" "/api/v1/universities/99999" "" "$ACCESS_TOKEN")

if ! check_success "$NONEXISTENT_RESPONSE"; then
    print_success "Correctly returned not found for non-existent university"
else
    print_failure "Should have returned not found"
fi

# 8.3 Test validation - missing required fields
print_test "Attempting to create course with missing required fields..."
INVALID_COURSE_RESPONSE=$(api_request "POST" "/api/v1/courses" '{
    "name": "Incomplete Course"
}' "$ACCESS_TOKEN")

if ! check_success "$INVALID_COURSE_RESPONSE"; then
    print_success "Correctly rejected incomplete course data"
else
    print_failure "Should have rejected incomplete course data"
fi

# =============================================================================
# Summary
# =============================================================================

print_header "TEST SUMMARY"

TOTAL_TESTS=$((TESTS_PASSED + TESTS_FAILED))

echo ""
echo -e "Total Tests: ${BLUE}$TOTAL_TESTS${NC}"
echo -e "Passed: ${GREEN}$TESTS_PASSED${NC}"
echo -e "Failed: ${RED}$TESTS_FAILED${NC}"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed. Please review the output above.${NC}"
    exit 1
fi
