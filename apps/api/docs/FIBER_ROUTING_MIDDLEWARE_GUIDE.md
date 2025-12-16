# Fiber Framework Routing & Middleware Guide

## Table of Contents
1. [Router Structure](#router-structure)
2. [Adding API v2 Routes](#adding-api-v2-routes)
3. [Middleware Patterns](#middleware-patterns)
4. [User Context Extraction](#user-context-extraction)
5. [Response Format Standards](#response-format-standards)
6. [Handler Patterns](#handler-patterns)
7. [Best Practices](#best-practices)

---

## Router Structure

### Current Organization (`router/main.go`)

The router follows a **hierarchical, feature-based** organization:

```go
// Health check endpoints (public)
app.Get("/ping", ...)
app.Get("/health/detailed", ...)

// API v1 group - all versioned endpoints
api := app.Group("/api/v1")

// Auth routes (public + protected)
authGroup := api.Group("/auth")

// Protected routes with middleware
profileGroup := api.Group("/profile", authMiddleware.Required())

// Admin-only routes
admin := api.Group("/admin", authMiddleware.RequireAdmin())
```

### Route Grouping Patterns

#### 1. **Public Routes** (No Authentication)
```go
// Health checks
app.Get("/ping", handler)

// Auth endpoints
authGroup.Post("/register", authHandler.Register)
authGroup.Post("/login", authHandler.Login)
authGroup.Post("/forgot-password", authHandler.ForgotPassword)
```

#### 2. **Protected Routes** (Requires Authentication)
```go
// Apply middleware to group
profileGroup := api.Group("/profile", authMiddleware.Required())
profileGroup.Get("/", authHandler.GetProfile)
profileGroup.Put("/", authHandler.UpdateProfile)

// Or individual routes
chat := api.Group("/chat", authMiddleware.Required())
```

#### 3. **Admin-Only Routes**
```go
admin := api.Group("/admin", authMiddleware.RequireAdmin())
admin.Get("/dashboard", analyticsHandler.GetDashboard)
admin.Get("/users", listUsersHandler)
```

#### 4. **Nested Resource Routes**
```go
// Courses -> Semesters -> Subjects pattern
courses := api.Group("/courses")
semesters := courses.Group("/:course_id/semesters")
subjects := api.Group("/semesters/:semester_id/subjects")
```

### Current API Version Structure

```
/api/v1/
├── auth/
│   ├── register (POST)
│   ├── login (POST)
│   ├── logout (POST - protected)
│   └── refresh (POST)
├── profile/ (protected)
├── universities/
├── courses/
│   └── :course_id/semesters/
├── subjects/
│   └── :subject_id/
│       ├── documents/
│       ├── syllabus/
│       └── pyqs/
├── chat/ (protected)
│   └── sessions/
├── analytics/ (protected)
└── admin/ (admin-only)
```

---

## Adding API v2 Routes

### Step 1: Create v2 Group in `router/main.go`

```go
func SetupRoutes(app *fiber.App, store database.Storage) {
    // ... existing v1 setup ...
    
    // API v1 group (existing)
    api := app.Group("/api/v1")
    
    // API v2 group (new)
    apiv2 := app.Group("/api/v2")
    
    // Setup v2 routes
    setupV2Routes(apiv2, authMiddleware, handlers...)
}
```

### Step 2: Create v2 Route Setup Function

```go
func setupV2Routes(
    api fiber.Router, 
    authMiddleware *middleware.AuthMiddleware,
    // ... add handlers as needed
) {
    // Public v2 endpoints
    api.Get("/status", v2handlers.GetStatus)
    
    // Protected v2 endpoints
    syllabusV2 := api.Group("/syllabus", authMiddleware.Required())
    
    // SSE endpoint for streaming extraction
    syllabusV2.Get("/extract/:id/stream", 
        syllabusV2Handler.StreamExtractionProgress)
    
    // Job tracking endpoints
    syllabusV2.Get("/jobs/:job_id/status", 
        syllabusV2Handler.GetJobStatus)
    
    syllabusV2.Get("/jobs/my-jobs", 
        syllabusV2Handler.GetUserJobs)
}
```

### Step 3: Example SSE Endpoint Implementation

```go
// handlers/syllabus/stream.go
package syllabus

import (
    "fmt"
    "time"
    
    "github.com/gofiber/fiber/v2"
    "github.com/sahilchouksey/go-init-setup/utils/middleware"
    "github.com/sahilchouksey/go-init-setup/utils/response"
)

// StreamExtractionProgress handles GET /api/v2/syllabus/extract/:id/stream
// Server-Sent Events endpoint for real-time extraction progress
func (h *SyllabusHandler) StreamExtractionProgress(c *fiber.Ctx) error {
    // Get user from context
    user, ok := middleware.GetUser(c)
    if !ok || user == nil {
        return response.Unauthorized(c, "User not authenticated")
    }
    
    jobID := c.Params("id")
    
    // Set SSE headers
    c.Set("Content-Type", "text/event-stream")
    c.Set("Cache-Control", "no-cache")
    c.Set("Connection", "keep-alive")
    c.Set("Transfer-Encoding", "chunked")
    
    c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
        // Stream progress updates
        for {
            progress, err := h.jobTracker.GetProgress(jobID, user.ID)
            if err != nil {
                fmt.Fprintf(w, "event: error\ndata: {\"error\": \"%s\"}\n\n", err.Error())
                w.Flush()
                return
            }
            
            // Send progress event
            data := fmt.Sprintf(`{"status": "%s", "progress": %d, "message": "%s"}`,
                progress.Status, progress.Percentage, progress.Message)
            fmt.Fprintf(w, "event: progress\ndata: %s\n\n", data)
            w.Flush()
            
            // Check if completed or failed
            if progress.Status == "completed" || progress.Status == "failed" {
                fmt.Fprintf(w, "event: done\ndata: %s\n\n", data)
                w.Flush()
                return
            }
            
            time.Sleep(500 * time.Millisecond)
        }
    })
    
    return nil
}
```

---

## Middleware Patterns

### Available Middleware

#### 1. **Authentication Middleware** (`utils/middleware/auth.go`)

```go
// Creates auth middleware instance
authMiddleware := middleware.NewAuthMiddleware(jwtManager, db)

// Middleware functions:
// - Required() - Requires valid JWT token
// - Optional() - Allows requests with/without token
// - RequireRole(...roles) - Requires specific role(s)
// - RequireAdmin() - Requires admin role
```

**Usage Examples:**

```go
// Require authentication
api.Get("/protected", authMiddleware.Required(), handler)

// Optional authentication (user info available if authenticated)
api.Get("/public", authMiddleware.Optional(), handler)

// Require admin role
api.Post("/admin-action", authMiddleware.RequireAdmin(), handler)

// Require specific roles
api.Post("/moderator", authMiddleware.RequireRole("moderator", "admin"), handler)
```

#### 2. **Security Middleware** (`utils/middleware/security.go`)

Applied globally in `router/main.go`:

```go
middleware.SetupSecurity(app, middleware.SecurityConfig{
    AllowedOrigins:    "http://localhost:3000,http://localhost:3001",
    RateLimitRequests: 100,
    RateLimitWindow:   1 * time.Minute,
})
```

**Features:**
- Request ID generation
- Request logging
- Panic recovery
- Security headers (Helmet)
- CORS configuration
- Rate limiting

#### 3. **Brute Force Protection** (`utils/middleware/brute_force.go`)

```go
bruteForceProtection := middleware.NewBruteForceProtection(redisCache)

// Apply to login endpoint
authGroup.Post("/login", 
    bruteForceProtection.CheckAndRecordAttempt(), 
    authHandler.Login)
```

**Features:**
- Progressive lockouts (5, 10, 25+ attempts)
- Redis-based tracking
- IP-based rate limiting
- Automatic attempt clearing on success

#### 4. **Admin Audit Log** (`utils/middleware/admin.go`)

```go
// Logs admin actions for audit trail
admin.Put("/users/:id", 
    middleware.AdminAuditLog(store, "user_update", "users"),
    updateUserHandler)
```

### Middleware Application Order

**Global Order (defined in `app/setup.go` and `router/main.go`):**

```
1. Request ID
2. Logger
3. Recover (panic handler)
4. Helmet (security headers)
5. CORS
6. Rate Limiter
7. Route-specific middleware (auth, brute force, etc.)
8. Handler
```

### Creating Custom Middleware

```go
package middleware

import "github.com/gofiber/fiber/v2"

// CustomMiddleware example
func CustomMiddleware() fiber.Handler {
    return func(c *fiber.Ctx) error {
        // Pre-processing
        // ... your logic ...
        
        // Continue to next middleware/handler
        if err := c.Next(); err != nil {
            return err
        }
        
        // Post-processing
        // ... your logic ...
        
        return nil
    }
}
```

---

## User Context Extraction

### How User Info is Stored in Context

The `authMiddleware.Required()` middleware validates JWT and stores user info:

```go
// Stored by auth middleware (middleware/auth.go:84-89)
c.Locals("user_id", claims.UserID)           // uint
c.Locals("user_email", claims.Email)         // string
c.Locals("user_role", claims.Role)           // string
c.Locals("claims", claims)                   // *auth.Claims
c.Locals("user", &user)                      // *model.User (full user object)
c.Locals("token_jti", claims.ID)             // string (token ID)
```

### Helper Functions for Context Extraction

**Location:** `utils/middleware/auth.go`

```go
// Get user ID
userID, ok := middleware.GetUserID(c)
if !ok {
    return response.Unauthorized(c, "User not authenticated")
}

// Get user email
email, ok := middleware.GetUserEmail(c)

// Get user role
role, ok := middleware.GetUserRole(c)

// Get full user object (RECOMMENDED for most cases)
user, ok := middleware.GetUser(c)
if !ok || user == nil {
    return response.Unauthorized(c, "User not authenticated")
}

// Get JWT claims
claims, ok := middleware.GetClaims(c)

// Get token JTI (for revocation)
jti, ok := middleware.GetTokenJTI(c)
```

### Best Practice: Use GetUser()

**Recommended pattern in handlers:**

```go
func (h *Handler) SomeProtectedEndpoint(c *fiber.Ctx) error {
    // Get user from context
    user, ok := middleware.GetUser(c)
    if !ok || user == nil {
        return response.Unauthorized(c, "User not authenticated")
    }
    
    // Now you have access to:
    // - user.ID (uint)
    // - user.Email (string)
    // - user.Role (string)
    // - user.TokenVersion (int)
    // - All other user fields
    
    // Use user ID for queries
    results, err := h.service.GetUserData(c.Context(), user.ID)
    
    // Check permissions
    if user.Role != "admin" && resource.OwnerID != user.ID {
        return response.Forbidden(c, "Access denied")
    }
    
    return response.Success(c, results)
}
```

### Example: Per-User Job Tracking

```go
// handlers/syllabus/jobs.go

// GetUserJobs handles GET /api/v2/syllabus/jobs/my-jobs
func (h *SyllabusHandler) GetUserJobs(c *fiber.Ctx) error {
    // Extract user ID from context
    user, ok := middleware.GetUser(c)
    if !ok || user == nil {
        return response.Unauthorized(c, "User not authenticated")
    }
    
    // Get jobs for this specific user
    jobs, err := h.jobTracker.GetJobsByUserID(c.Context(), user.ID)
    if err != nil {
        return response.InternalServerError(c, "Failed to fetch jobs")
    }
    
    return response.Success(c, fiber.Map{
        "user_id": user.ID,
        "jobs":    jobs,
    })
}

// GetJobStatus handles GET /api/v2/syllabus/jobs/:job_id/status
func (h *SyllabusHandler) GetJobStatus(c *fiber.Ctx) error {
    user, ok := middleware.GetUser(c)
    if !ok || user == nil {
        return response.Unauthorized(c, "User not authenticated")
    }
    
    jobID := c.Params("job_id")
    
    // Get job and verify ownership
    job, err := h.jobTracker.GetJob(c.Context(), jobID)
    if err != nil {
        return response.NotFound(c, "Job not found")
    }
    
    // Security check: verify user owns this job
    if job.UserID != user.ID && user.Role != "admin" {
        return response.Forbidden(c, "Access denied")
    }
    
    return response.Success(c, job)
}
```

---

## Response Format Standards

### Location
`utils/response/response.go`

### Standard Response Structure

```go
type Response struct {
    Success bool         `json:"success"`
    Message string       `json:"message,omitempty"`
    Data    interface{}  `json:"data,omitempty"`
    Error   *ErrorDetail `json:"error,omitempty"`
}

type ErrorDetail struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Details string `json:"details,omitempty"`
}
```

### Success Responses

```go
// Basic success (200 OK)
return response.Success(c, fiber.Map{
    "id": 123,
    "name": "Example",
})
// Response: {"success": true, "data": {"id": 123, "name": "Example"}}

// Success with message
return response.SuccessWithMessage(c, "Operation completed", data)
// Response: {"success": true, "message": "Operation completed", "data": {...}}

// Created (201)
return response.Created(c, newResource)
// Response: {"success": true, "message": "Resource created successfully", "data": {...}}

// No content (204)
return response.NoContent(c)
// Response: Empty with 204 status
```

### Error Responses

```go
// Bad Request (400)
return response.BadRequest(c, "Invalid input")
// Response: {"success": false, "error": {"code": "BAD_REQUEST", "message": "Invalid input"}}

// Unauthorized (401)
return response.Unauthorized(c, "Missing token")
// Response: {"success": false, "error": {"code": "UNAUTHORIZED", "message": "Missing token"}}

// Forbidden (403)
return response.Forbidden(c, "Insufficient permissions")
// Response: {"success": false, "error": {"code": "FORBIDDEN", "message": "Insufficient permissions"}}

// Not Found (404)
return response.NotFound(c, "Resource not found")
// Response: {"success": false, "error": {"code": "NOT_FOUND", "message": "Resource not found"}}

// Conflict (409)
return response.Conflict(c, "Resource already exists")
// Response: {"success": false, "error": {"code": "CONFLICT", "message": "Resource already exists"}}

// Validation Error (422)
return response.ValidationError(c, err)
// Response: {"success": false, "error": {"code": "VALIDATION_ERROR", "message": "Validation failed", "details": "..."}}

// Internal Server Error (500)
return response.InternalServerError(c, "Database error")
// Response: {"success": false, "error": {"code": "INTERNAL_ERROR", "message": "Database error"}}

// Service Unavailable (503)
return response.ServiceUnavailable(c, "Service temporarily down")
// Response: {"success": false, "error": {"code": "SERVICE_UNAVAILABLE", "message": "Service temporarily down"}}
```

### Paginated Responses

```go
type PaginationMeta struct {
    CurrentPage int   `json:"current_page"`
    PerPage     int   `json:"per_page"`
    Total       int64 `json:"total"`
    TotalPages  int   `json:"total_pages"`
}

// Usage in handler:
page, _ := strconv.Atoi(c.Query("page", "1"))
limit, _ := strconv.Atoi(c.Query("limit", "20"))

// Get total count
var total int64
h.db.Model(&model.Resource{}).Count(&total)

// Calculate pagination
pagination := response.CalculatePagination(page, limit, total)

// Fetch data
var items []model.Resource
h.db.Limit(limit).Offset((page - 1) * limit).Find(&items)

// Return paginated response
return response.Paginated(c, items, pagination)
// Response: {
//   "success": true,
//   "data": [...],
//   "pagination": {
//     "current_page": 1,
//     "per_page": 20,
//     "total": 100,
//     "total_pages": 5
//   }
// }
```

---

## Handler Patterns

### Standard Handler Structure

```go
package myhandler

import (
    "github.com/gofiber/fiber/v2"
    "github.com/sahilchouksey/go-init-setup/services"
    "github.com/sahilchouksey/go-init-setup/utils/middleware"
    "github.com/sahilchouksey/go-init-setup/utils/response"
    "github.com/sahilchouksey/go-init-setup/utils/validation"
    "gorm.io/gorm"
)

// Handler struct
type MyHandler struct {
    db        *gorm.DB
    service   *services.MyService
    validator *validation.Validator
}

// Constructor
func NewMyHandler(db *gorm.DB, service *services.MyService) *MyHandler {
    return &MyHandler{
        db:        db,
        service:   service,
        validator: validation.NewValidator(),
    }
}

// Request DTOs
type CreateRequest struct {
    Name string `json:"name" validate:"required,min=3,max=100"`
    Type string `json:"type" validate:"required,oneof=typeA typeB"`
}

// Handler method
func (h *MyHandler) Create(c *fiber.Ctx) error {
    // 1. Get user from context (if protected)
    user, ok := middleware.GetUser(c)
    if !ok || user == nil {
        return response.Unauthorized(c, "User not authenticated")
    }
    
    // 2. Parse request body
    var req CreateRequest
    if err := c.BodyParser(&req); err != nil {
        return response.BadRequest(c, "Invalid request body")
    }
    
    // 3. Validate request
    if err := h.validator.Validate(req); err != nil {
        return response.ValidationError(c, err)
    }
    
    // 4. Call service layer
    result, err := h.service.Create(c.Context(), req, user.ID)
    if err != nil {
        // Handle specific errors
        if errors.Is(err, services.ErrDuplicate) {
            return response.Conflict(c, "Resource already exists")
        }
        return response.InternalServerError(c, "Failed to create resource")
    }
    
    // 5. Return response
    return response.Created(c, result)
}
```

### Parameter Extraction Patterns

```go
// Path parameters
id := c.Params("id")                    // /resources/:id
subjectID := c.Params("subject_id")     // /subjects/:subject_id/documents

// Parse to uint
idNum, err := strconv.ParseUint(id, 10, 32)
if err != nil {
    return response.BadRequest(c, "Invalid ID")
}

// Query parameters
page := c.Query("page", "1")            // /resources?page=2
filter := c.Query("type", "")           // /resources?type=syllabus
async := c.Query("async", "false") == "true"  // Boolean query param

// Form data
file, err := c.FormFile("file")
if err != nil {
    return response.BadRequest(c, "File is required")
}

// Headers
authHeader := c.Get("Authorization")
contentType := c.Get("Content-Type")
```

### File Upload Pattern

```go
func (h *Handler) UploadDocument(c *fiber.Ctx) error {
    // Get user
    user, ok := middleware.GetUser(c)
    if !ok || user == nil {
        return response.Unauthorized(c, "User not authenticated")
    }
    
    // Get file
    file, err := c.FormFile("file")
    if err != nil {
        return response.BadRequest(c, "File is required")
    }
    
    // Validate file size
    const maxFileSize = 50 * 1024 * 1024 // 50MB
    if file.Size > maxFileSize {
        return response.BadRequest(c, "File size exceeds 50MB limit")
    }
    
    // Open file
    fileContent, err := file.Open()
    if err != nil {
        return response.InternalServerError(c, "Failed to open file")
    }
    defer fileContent.Close()
    
    // Upload via service
    result, err := h.service.Upload(c.Context(), fileContent, file, user.ID)
    if err != nil {
        return response.InternalServerError(c, "Failed to upload file")
    }
    
    return response.Created(c, result)
}
```

### Permission Check Pattern

```go
func (h *Handler) Update(c *fiber.Ctx) error {
    user, ok := middleware.GetUser(c)
    if !ok || user == nil {
        return response.Unauthorized(c, "User not authenticated")
    }
    
    id := c.Params("id")
    
    // Get existing resource
    resource, err := h.service.GetByID(c.Context(), id)
    if err != nil {
        return response.NotFound(c, "Resource not found")
    }
    
    // Permission check: admin or owner
    if user.Role != "admin" && resource.OwnerID != user.ID {
        return response.Forbidden(c, "You don't have permission to update this resource")
    }
    
    // Continue with update...
    var req UpdateRequest
    if err := c.BodyParser(&req); err != nil {
        return response.BadRequest(c, "Invalid request body")
    }
    
    updated, err := h.service.Update(c.Context(), id, req)
    if err != nil {
        return response.InternalServerError(c, "Failed to update resource")
    }
    
    return response.Success(c, updated)
}
```

---

## Best Practices

### 1. **Route Organization**
- Group related routes together
- Use consistent naming conventions
- Version your API (`/api/v1`, `/api/v2`)
- Use RESTful patterns where appropriate

### 2. **Middleware Usage**
- Apply authentication middleware at the group level
- Use specific middleware (e.g., `RequireAdmin()`) instead of manual checks
- Order middleware from general to specific
- Keep middleware focused and single-purpose

### 3. **User Context**
- Always use `middleware.GetUser(c)` in protected endpoints
- Check `ok` and `nil` before using user object
- Store user ID in service calls, not the entire user object
- Verify ownership before allowing modifications

### 4. **Error Handling**
- Use appropriate HTTP status codes
- Return consistent error response format
- Don't leak sensitive information in error messages
- Log detailed errors server-side, return generic messages to client

### 5. **Response Format**
- Use response helper functions (`response.Success`, `response.BadRequest`, etc.)
- Keep response structures consistent across endpoints
- Include pagination metadata for list endpoints
- Use `fiber.Map` for dynamic response objects

### 6. **Validation**
- Validate all input at the handler level
- Use struct tags for validation rules
- Return validation errors with details
- Sanitize user input

### 7. **Database Queries**
- Use `c.Context()` for context-aware database operations
- Preload relationships efficiently
- Handle `gorm.ErrRecordNotFound` specifically
- Use transactions for multi-step operations

### 8. **Security**
- Never trust user input
- Verify ownership before operations
- Use HTTPS in production
- Implement rate limiting on sensitive endpoints
- Log security events (failed logins, permission denials)

### 9. **Performance**
- Use pagination for large datasets
- Implement caching where appropriate
- Avoid N+1 queries (use Preload)
- Use background jobs for long-running tasks

### 10. **Code Organization**
```
handlers/
├── auth/           # Authentication handlers
├── syllabus/       # Syllabus-related handlers
│   ├── syllabus.go
│   ├── stream.go   # SSE endpoints
│   └── jobs.go     # Job tracking
├── ...

services/
├── syllabus_service.go
├── job_tracker.go
└── ...

utils/
├── middleware/     # All middleware
├── response/       # Response helpers
└── validation/     # Validation utilities
```

---

## Quick Reference Checklist

### Adding a New Protected Endpoint

- [ ] Create handler function
- [ ] Extract user from context using `middleware.GetUser(c)`
- [ ] Validate user exists (`!ok || user == nil`)
- [ ] Parse and validate request parameters
- [ ] Check permissions if needed
- [ ] Call service layer with `c.Context()`
- [ ] Return appropriate response using `response.*` helpers
- [ ] Add route to router with `authMiddleware.Required()`

### Adding a New Admin Endpoint

- [ ] Create handler function
- [ ] Use `middleware.GetUser(c)` to get admin user
- [ ] Call service layer
- [ ] Return response
- [ ] Add to admin group with `authMiddleware.RequireAdmin()`
- [ ] Optional: Add audit logging with `AdminAuditLog` middleware

### Adding SSE Endpoint

- [ ] Set SSE headers (`Content-Type`, `Cache-Control`, `Connection`)
- [ ] Use `c.Context().SetBodyStreamWriter()`
- [ ] Send events in format: `event: <name>\ndata: <json>\n\n`
- [ ] Flush after each write
- [ ] Handle cleanup on connection close
- [ ] Protect with authentication middleware

---

## Example: Complete v2 Syllabus Extraction API

```go
// router/main.go - Add v2 routes
func SetupRoutes(app *fiber.App, store database.Storage) {
    // ... existing setup ...
    
    // API v2 group
    apiv2 := app.Group("/api/v2")
    
    // Initialize v2 handlers
    syllabusV2Handler := syllabus_v2.NewSyllabusV2Handler(db, syllabusService, jobTracker)
    
    // Syllabus v2 endpoints (all protected)
    syllabusV2 := apiv2.Group("/syllabus", authMiddleware.Required())
    
    // Async extraction
    syllabusV2.Post("/extract/:document_id", syllabusV2Handler.StartExtraction)
    
    // SSE streaming
    syllabusV2.Get("/jobs/:job_id/stream", syllabusV2Handler.StreamProgress)
    
    // Job management
    syllabusV2.Get("/jobs/:job_id", syllabusV2Handler.GetJobStatus)
    syllabusV2.Get("/jobs", syllabusV2Handler.GetMyJobs)
    syllabusV2.Delete("/jobs/:job_id", syllabusV2Handler.CancelJob)
}
```

```go
// handlers/syllabus/v2/handler.go
package syllabusv2

import (
    "bufio"
    "fmt"
    "time"
    
    "github.com/gofiber/fiber/v2"
    "github.com/sahilchouksey/go-init-setup/services"
    "github.com/sahilchouksey/go-init-setup/utils/middleware"
    "github.com/sahilchouksey/go-init-setup/utils/response"
    "gorm.io/gorm"
)

type SyllabusV2Handler struct {
    db              *gorm.DB
    syllabusService *services.SyllabusService
    jobTracker      *services.JobTracker
}

func NewSyllabusV2Handler(
    db *gorm.DB,
    syllabusService *services.SyllabusService,
    jobTracker *services.JobTracker,
) *SyllabusV2Handler {
    return &SyllabusV2Handler{
        db:              db,
        syllabusService: syllabusService,
        jobTracker:      jobTracker,
    }
}

// StartExtraction handles POST /api/v2/syllabus/extract/:document_id
func (h *SyllabusV2Handler) StartExtraction(c *fiber.Ctx) error {
    user, ok := middleware.GetUser(c)
    if !ok || user == nil {
        return response.Unauthorized(c, "User not authenticated")
    }
    
    documentID := c.Params("document_id")
    
    // Create job
    job, err := h.jobTracker.CreateJob(c.Context(), services.CreateJobRequest{
        UserID:     user.ID,
        Type:       "syllabus_extraction",
        ResourceID: documentID,
    })
    if err != nil {
        return response.InternalServerError(c, "Failed to create job")
    }
    
    // Start extraction in background
    go h.syllabusService.ExtractAsync(job.ID, documentID)
    
    return response.Created(c, fiber.Map{
        "job_id":  job.ID,
        "status":  job.Status,
        "message": "Extraction started",
    })
}

// StreamProgress handles GET /api/v2/syllabus/jobs/:job_id/stream
func (h *SyllabusV2Handler) StreamProgress(c *fiber.Ctx) error {
    user, ok := middleware.GetUser(c)
    if !ok || user == nil {
        return response.Unauthorized(c, "User not authenticated")
    }
    
    jobID := c.Params("job_id")
    
    // Verify job ownership
    job, err := h.jobTracker.GetJob(c.Context(), jobID)
    if err != nil {
        return response.NotFound(c, "Job not found")
    }
    
    if job.UserID != user.ID && user.Role != "admin" {
        return response.Forbidden(c, "Access denied")
    }
    
    // Set SSE headers
    c.Set("Content-Type", "text/event-stream")
    c.Set("Cache-Control", "no-cache")
    c.Set("Connection", "keep-alive")
    c.Set("Transfer-Encoding", "chunked")
    
    c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
        ticker := time.NewTicker(500 * time.Millisecond)
        defer ticker.Stop()
        
        for {
            select {
            case <-ticker.C:
                progress, err := h.jobTracker.GetProgress(jobID)
                if err != nil {
                    fmt.Fprintf(w, "event: error\ndata: {\"error\":\"%s\"}\n\n", err.Error())
                    w.Flush()
                    return
                }
                
                data := fmt.Sprintf(`{"status":"%s","progress":%d,"message":"%s"}`,
                    progress.Status, progress.Percentage, progress.Message)
                
                fmt.Fprintf(w, "event: progress\ndata: %s\n\n", data)
                w.Flush()
                
                if progress.Status == "completed" || progress.Status == "failed" {
                    fmt.Fprintf(w, "event: done\ndata: %s\n\n", data)
                    w.Flush()
                    return
                }
            case <-c.Context().Done():
                return
            }
        }
    })
    
    return nil
}

// GetJobStatus handles GET /api/v2/syllabus/jobs/:job_id
func (h *SyllabusV2Handler) GetJobStatus(c *fiber.Ctx) error {
    user, ok := middleware.GetUser(c)
    if !ok || user == nil {
        return response.Unauthorized(c, "User not authenticated")
    }
    
    jobID := c.Params("job_id")
    
    job, err := h.jobTracker.GetJob(c.Context(), jobID)
    if err != nil {
        return response.NotFound(c, "Job not found")
    }
    
    if job.UserID != user.ID && user.Role != "admin" {
        return response.Forbidden(c, "Access denied")
    }
    
    return response.Success(c, job)
}

// GetMyJobs handles GET /api/v2/syllabus/jobs
func (h *SyllabusV2Handler) GetMyJobs(c *fiber.Ctx) error {
    user, ok := middleware.GetUser(c)
    if !ok || user == nil {
        return response.Unauthorized(c, "User not authenticated")
    }
    
    jobs, err := h.jobTracker.GetUserJobs(c.Context(), user.ID)
    if err != nil {
        return response.InternalServerError(c, "Failed to fetch jobs")
    }
    
    return response.Success(c, fiber.Map{
        "user_id": user.ID,
        "count":   len(jobs),
        "jobs":    jobs,
    })
}
```

---

## Summary

This guide covers:
- ✅ Router organization and structure
- ✅ How to add API v2 routes
- ✅ All middleware types and usage
- ✅ User context extraction patterns
- ✅ Standard response formats
- ✅ Handler patterns and best practices
- ✅ Complete SSE example for streaming

Use this as your reference when building the SSE-based syllabus extraction endpoints!
