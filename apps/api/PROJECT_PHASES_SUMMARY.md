# Study in Woods - Project Phases Summary

**Project**: Study in Woods Backend API v2
**Last Updated**: November 2, 2025
**Status**: All phases documented and ready for implementation

---

## 📋 Project Tech Stack

- **Language**: Go 1.24.0+
- **Web Framework**: Fiber v3
- **Database**: PostgreSQL 15+ (with GORM v2)
- **Cache/Session**: Redis 7+
- **Authentication**: JWT with bcrypt
- **Encryption**: AES-256-GCM
- **Migration**: golang-migrate/v4
- **Hot Reload**: Air (development)
- **Containerization**: Docker + Docker Compose

---

## Phase 0: Infrastructure & DevOps Setup

**Priority**: 🔴 CRITICAL
**Estimated Time**: 4 hours
**Dependencies**: None (First phase)

### What This Phase Does
Set up the foundational infrastructure required for development:
- Local development environment with Docker
- Redis for caching and distributed operations
- Hot reload for development
- Environment configuration
- Basic CI/CD pipeline

### Technologies/Services Used
- **Docker & Docker Compose**: Container orchestration
- **PostgreSQL 15-alpine**: Relational database (port 5432)
- **Redis 7-alpine**: Caching and session management (port 6379)
- **Air**: Hot reload for Go development
- **GitHub Actions**: CI/CD pipeline
- **Makefile**: Common development tasks

### External Services
None (All local infrastructure)

### Result
- ✅ Docker Compose running PostgreSQL and Redis
- ✅ Development environment with hot reload
- ✅ Comprehensive `.env` configuration
- ✅ Basic CI/CD pipeline
- ✅ Healthchecks for all services
- ✅ Ready to start Phase 0.5

### Environment Variables Required
```env
# Application
APP_ENV=development
APP_PORT=8080
APP_URL=http://localhost:8080
APP_NAME=Study-in-Woods

# Database
DATABASE_URL=postgresql://postgres:postgres@localhost:5432/study_in_woods?sslmode=disable
DB_MAX_CONNECTIONS=100
DB_MAX_IDLE_CONNECTIONS=10
DB_CONN_MAX_LIFETIME=1h
DB_CONN_MAX_IDLE_TIME=10m

# Redis
REDIS_URL=redis://localhost:6379
REDIS_PASSWORD=
REDIS_DB=0
REDIS_MAX_RETRIES=3
REDIS_POOL_SIZE=10

# Logging
LOG_LEVEL=debug
LOG_FORMAT=text
DEBUG=true
```

---

## Phase 0.5: Security Setup

**Priority**: 🔴 CRITICAL
**Estimated Time**: 3 hours
**Dependencies**: Phase 0 ✅

### What This Phase Does
Implement critical security components before building features:
- JWT token management
- Password hashing
- Redis-based caching
- Security middleware (rate limiting, brute force protection, CORS)
- Request validation
- Encryption for sensitive data

### Technologies/Services Used
- **bcrypt**: Password hashing (golang.org/x/crypto/bcrypt) - cost=12
- **JWT**: Token authentication (github.com/golang-jwt/jwt/v5)
- **Redis Client**: go-redis/v9 for distributed caching
- **AES-256-GCM**: Symmetric encryption for sensitive data
- **Fiber Middleware**: Rate limiting, CORS, Helmet
- **gofiber/storage/redis**: Distributed rate limiting storage

### Services Implemented
- **Password Utility**: Secure hashing and verification
- **JWT Service**: Token generation, validation, refresh
- **Redis Cache**: Distributed caching and rate limiting
- **Encryption Service**: Sensitive data encryption (AES-256-GCM)
- **11 Security Middleware**:
  1. Rate Limiter (25 req/min per IP)
  2. Auth Rate Limiter (5 req/15min)
  3. Brute Force Protection (escalating lockouts)
  4. CORS configuration
  5. Body Limit (10MB)
  6. HTTPS Enforcement
  7. Request ID tracking
  8. Structured Logger
  9. Helmet (security headers)
  10. Recovery middleware
  11. Request timeout

### External Services
None (Uses local Redis)

### Result
- ✅ Secure password hashing utilities
- ✅ JWT generation and validation
- ✅ Redis cache service
- ✅ Security middleware suite (11 middleware)
- ✅ Encryption service for sensitive settings
- ✅ Ready to implement authentication (Phase 3)

### Environment Variables Required
```env
# JWT Configuration
JWT_SECRET=<base64-32-bytes>  # Generate: openssl rand -base64 32
JWT_EXPIRY=24h
JWT_REFRESH_EXPIRY=168h
JWT_ISSUER=study-in-woods-api

# Encryption
ENCRYPTION_KEY=<base64-32-bytes>  # Generate: openssl rand -base64 32

# Security & CORS
ALLOWED_ORIGINS=http://localhost:3000,http://localhost:3001
CORS_ALLOW_CREDENTIALS=true
CORS_MAX_AGE=86400

# Rate Limiting
RATE_LIMIT_REQUESTS=25
RATE_LIMIT_WINDOW=1m
RATE_LIMIT_AUTH_REQUESTS=5
RATE_LIMIT_AUTH_WINDOW=15m

# Brute Force Protection
BRUTE_FORCE_MAX_ATTEMPTS=5
BRUTE_FORCE_LOCKOUT_DURATION=15m
```

---

## Phase 1: Database Schema & Models

**Priority**: 🔴 CRITICAL
**Estimated Time**: 6 hours
**Dependencies**: Phase 0 ✅ + Phase 0.5 ✅

### What This Phase Does
Implement complete database schema with:
- 14 core tables with proper relationships
- Connection pooling for performance
- Transaction management for data integrity
- SQL migrations for version control
- GORM models with hooks and validation
- Proper indexing for query performance

### Technologies/Services Used
- **GORM v2**: ORM with auto-migration
- **golang-migrate/v4**: SQL migration management
- **PostgreSQL Extensions**: uuid-ossp, pgcrypto
- **lib/pq**: PostgreSQL driver
- **Connection Pooling**: Max 100 connections, optimized settings
- **Transaction Service**: Atomic operations with auto-rollback

### Database Tables (14 Total)
1. `users` - User accounts and authentication
2. `universities` - University/institution data
3. `courses` - Academic courses
4. `semesters` - Course semesters
5. `subjects` - Subject/module data with AI agent info
6. `documents` - File storage metadata
7. `chat_sessions` - Chat conversation sessions
8. `chat_messages` - Individual chat messages
9. `course_payments` - Payment records with storage quotas
10. `api_key_usage_logs` - API usage tracking
11. `app_settings` - Application configuration (encrypted)
12. `jwt_token_blacklist` - Revoked JWT tokens
13. `cron_job_logs` - Background job execution logs
14. `admin_audit_logs` - Admin action audit trail

### External Services
None (Pure database operations)

### Result
- ✅ Database connection with optimized pool settings
- ✅ Transaction service for atomic operations
- ✅ SQL migration system (golang-migrate)
- ✅ All 14 GORM models implemented
- ✅ Proper foreign keys and cascades
- ✅ Validation hooks on models
- ✅ Indexes for performance
- ✅ Ready for implementing API handlers (Phase 2+)

### Environment Variables Required
```env
# Database (from Phase 0)
DATABASE_URL=postgresql://postgres:postgres@localhost:5432/study_in_woods?sslmode=disable
DB_MAX_CONNECTIONS=100
DB_MAX_IDLE_CONNECTIONS=10
DB_CONN_MAX_LIFETIME=1h
DB_CONN_MAX_IDLE_TIME=10m

# Debug (optional)
DEBUG_SQL=false  # Enable to see SQL queries
```

---

## Phase 2: DigitalOcean Integration

**Priority**: 🔴 CRITICAL
**Estimated Time**: 5 hours
**Dependencies**: Phase 0 ✅ + Phase 0.5 ✅ + Phase 1 ✅

### What This Phase Does
Implement robust DigitalOcean API client with:
- GenAI API integration (Knowledge Bases, Agents, Data Sources)
- Spaces (S3-compatible) client for file storage
- Circuit breaker pattern for fault tolerance
- Exponential backoff retry logic
- Rate limiting to prevent API abuse
- Request/response validation
- Comprehensive error handling

### Technologies/Services Used
- **DigitalOcean API**: GenAI Platform REST API
- **AWS SDK v2**: S3-compatible Spaces client (github.com/aws/aws-sdk-go-v2)
- **Circuit Breaker**: sony/gobreaker for fault tolerance
- **Retry Logic**: Exponential backoff with jitter
- **Rate Limiter**: Token bucket algorithm
- **HTTP Client**: Custom client with timeout and retry

### External Services Used
**1. DigitalOcean GenAI Platform**
   - Knowledge Bases: Document indexing and RAG
   - Agents: AI-powered chat agents
   - Data Sources: File upload to knowledge bases
   - Embeddings: Text vectorization
   - Rate Limit: 5000 requests/hour

**2. DigitalOcean Spaces**
   - S3-compatible object storage (use AWS SDK v2)
   - Bearer token authentication with `DIGITALOCEAN_TOKEN`
   - CDN-enabled file delivery
   - Multi-region support (blr1 - Bangalore)
   - File versioning
   - Upload via AWS SDK v2, returns object key for Knowledge Base integration

### File Upload Integration Flow
**Important**: DigitalOcean Spaces is S3-compatible. The complete flow is:
1. Upload file to Spaces using **AWS SDK v2** → Get object key
2. Create Data Source in Knowledge Base using object key
3. Knowledge Base fetches and indexes file from Spaces

### Result
- ✅ DigitalOcean GenAI client with retry logic
- ✅ Circuit breaker preventing cascading failures
- ✅ Rate limiter for API calls
- ✅ Spaces client (AWS SDK v2) for file uploads/downloads
- ✅ Knowledge Base CRUD operations
- ✅ Agent CRUD operations
- ✅ Data Source management with object key support
- ✅ Comprehensive error handling
- ✅ Ready for file upload and chat features
### Environment Variables Required
```env
# DigitalOcean API (Universal Token - used for GenAI API and Spaces)
DIGITALOCEAN_TOKEN=dop_v1_XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX

# Circuit Breaker Settings
DO_API_MAX_RETRIES=3
DO_API_TIMEOUT=30s
DO_API_CIRCUIT_BREAKER_THRESHOLD=5
DO_API_CIRCUIT_BREAKER_TIMEOUT=60s

# Spaces Configuration (uses DIGITALOCEAN_TOKEN for bearer auth)
DO_SPACES_BUCKET=study-in-woods
DO_SPACES_REGION=blr1
DO_SPACES_ENDPOINT=https://blr1.digitaloceanspaces.com
DO_SPACES_CDN_ENDPOINT=https://study-in-woods.blr1.cdn.digitaloceanspaces.com

# Auto-synced by cron (optional)
DO_VPC_UUID=
DO_PROJECT_ID=
DO_EMBEDDING_MODEL_UUID=
DO_REGION_SLUG=blr1
```

---

## Phase 3: Authentication System

**Priority**: 🔴 CRITICAL
**Estimated Time**: 4 hours
**Dependencies**: Phase 0 ✅ + Phase 0.5 ✅ + Phase 1 ✅ + Phase 2 ✅

### What This Phase Does
Implement complete authentication and authorization system with:
- User registration and login
- JWT token management with revocation
- Role-based access control (RBAC)
- Brute force protection
- Password reset functionality
- Session management
- Security middleware stack

### Technologies/Services Used
- **JWT**: golang-jwt/jwt/v5 (from Phase 0.5)
- **bcrypt**: Password hashing (from Phase 0.5)
- **Redis**: Token blacklist and session management
- **Fiber Middleware**: Authentication and RBAC
- **JTI (JWT ID)**: Token revocation tracking
- **Email Service**: SMTP for password reset (optional)

### Authentication Features
1. **Registration** - Email/password with validation
2. **Login** - JWT token generation with brute force protection
3. **Token Management** - JTI-based revocation system
4. **Role-Based Access** - normal, admin, superadmin roles
5. **Password Reset** - Secure reset flow with email
6. **Session Management** - Token refresh and logout
7. **Security** - Rate limiting, CORS, HTTPS enforcement

### External Services Used
- **SMTP Server** (optional): Email delivery for password reset
  - Gmail SMTP
  - SendGrid
  - AWS SES

### Result
- ✅ User registration endpoint with validation
- ✅ Login endpoint with brute force protection
- ✅ JWT middleware with token revocation
- ✅ Role-based access control middleware
- ✅ Password reset flow
- ✅ Logout with token revocation
- ✅ Admin-only endpoints protection
- ✅ Complete security middleware stack
- ✅ Ready for protected API endpoints

### Environment Variables Required
```env
# JWT (from Phase 0.5)
JWT_SECRET=<base64-32-bytes>
JWT_EXPIRY=24h
JWT_REFRESH_EXPIRY=168h
JWT_ISSUER=study-in-woods-api

# Email (for password reset - optional)
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=your-email@gmail.com
SMTP_PASSWORD=your-app-password
SMTP_FROM=noreply@studyinwoods.com

# Auth Rate Limiting
RATE_LIMIT_AUTH_REQUESTS=5
RATE_LIMIT_AUTH_WINDOW=15m
BRUTE_FORCE_MAX_ATTEMPTS=5
BRUTE_FORCE_LOCKOUT_DURATION=15m
```

---

## Phase 4: Core API Endpoints

**Priority**: 🔴 CRITICAL
**Estimated Time**: 6 hours
**Dependencies**: Phase 0 ✅ + Phase 0.5 ✅ + Phase 1 ✅ + Phase 2 ✅ + Phase 3 ✅

### What This Phase Does
Implement core API endpoints for the academic structure with:
- Universities CRUD operations
- Courses CRUD operations
- Semesters CRUD operations
- Input validation system
- Standardized error responses
- API versioning (/api/v1/)
- Role-based access control
- Comprehensive testing

### Technologies/Services Used
- **Fiber v3**: HTTP routing and handlers
- **GORM**: Database operations with transactions
- **go-playground/validator/v10**: Request validation
- **Slug Generation**: gosimple/slug for URL-friendly slugs
- **Pagination**: Cursor-based pagination
- **Error Handling**: Standardized error responses

### API Endpoints Implemented
```
# Universities (5 endpoints)
GET    /api/v1/universities           - List universities (public)
GET    /api/v1/universities/:slug     - Get university (public)
POST   /api/v1/universities           - Create (admin only)
PUT    /api/v1/universities/:slug     - Update (admin only)
DELETE /api/v1/universities/:slug     - Delete (admin only)

# Courses (5 endpoints)
GET    /api/v1/courses                - List courses (public)
GET    /api/v1/courses/:slug          - Get course (public)
POST   /api/v1/courses                - Create (authenticated)
PUT    /api/v1/courses/:slug          - Update (creator or admin)
DELETE /api/v1/courses/:slug          - Delete (creator or admin)

# Semesters (5 endpoints)
GET    /api/v1/courses/:slug/semesters       - List semesters (public)
GET    /api/v1/courses/:slug/semesters/:num  - Get semester (public)
POST   /api/v1/courses/:slug/semesters       - Create (creator or admin)
PUT    /api/v1/courses/:slug/semesters/:num  - Update (creator or admin)
DELETE /api/v1/courses/:slug/semesters/:num  - Delete (creator or admin)

# Health & Metrics (2 endpoints)
GET    /health                        - Health check
GET    /metrics                       - Prometheus metrics
```

### External Services Used
None (Pure CRUD operations)

### Result
- ✅ Input validation utilities
- ✅ Standardized error response system
- ✅ Universities API endpoints (CRUD)
- ✅ Courses API endpoints (CRUD)
- ✅ Semesters API endpoints (CRUD)
- ✅ API versioning structure
- ✅ Role-based endpoint protection
- ✅ Comprehensive request/response validation
- ✅ Ready for subjects and file upload features

### Environment Variables Required
```env
# No additional env vars required
# Uses existing database and auth configuration from previous phases
```

---

## Phase 5: Subjects with AI Integration

**Priority**: 🔴 CRITICAL
**Estimated Time**: 6 hours
**Dependencies**: Phase 0 ✅ + Phase 0.5 ✅ + Phase 1 ✅ + Phase 2 ✅ + Phase 3 ✅ + Phase 4 ✅

### What This Phase Does
Implement subject management with automatic AI integration:
- Subject CRUD operations with validation
- Automatic Knowledge Base creation via DigitalOcean GenAI API
- Automatic AI Agent creation and configuration
- Transactional subject creation with rollback
- Idempotent operations to prevent duplicates
- Settings-based DigitalOcean configuration
- Subject approval workflow

### Technologies/Services Used
- **DigitalOcean GenAI API**: Knowledge Base + Agent creation (from Phase 2)
- **Encryption Service**: AES-256-GCM for agent keys (from Phase 0.5)
- **Transaction Service**: Atomic DB + API operations (from Phase 1)
- **GORM Transactions**: Multi-step operations with rollback
- **Settings Service**: Encrypted app_settings management
- **Idempotency**: UUID-based duplicate prevention

### Subject Creation Flow
1. **Validate Input** - Check semester exists, user permissions
2. **Create Subject** - Database record with generated slug
3. **Create Knowledge Base** - DigitalOcean GenAI API call
4. **Create AI Agent** - Connected to Knowledge Base
5. **Create API Key** - For agent access (encrypted)
6. **Update Subject** - Store all UUIDs and URLs
7. **Handle Rollback** - Clean up on any failure

### External Services Used
**DigitalOcean GenAI Platform Resources Created Per Subject:**
- **Knowledge Base**: Document storage and RAG
  - UUID stored in `subjects.knowledge_base_uuid`
  - Name: `<course>-<semester>-<subject>`
  - Embedding Model: Auto-configured

- **AI Agent**: Chat interface
  - UUID stored in `subjects.agent_uuid`
  - URL stored in `subjects.agent_url`
  - Key UUID: Encrypted in `subjects.agent_key_uuid`
  - Model: Claude 3 Sonnet

### Result
- ✅ Subject CRUD API endpoints
- ✅ Automatic Knowledge Base creation
- ✅ Automatic AI Agent creation with API keys
- ✅ Transactional subject creation service
- ✅ Idempotent DigitalOcean resource creation
- ✅ Settings service for DO configuration
- ✅ Subject approval workflow
- ✅ Comprehensive error handling and rollback
- ✅ Ready for file upload and chat features

### Environment Variables Required
```env
# From Phase 2
DIGITALOCEAN_TOKEN=dop_v1_xxxxx
DO_EMBEDDING_MODEL_UUID=<auto-synced by cron>

# From Phase 0.5
ENCRYPTION_KEY=<base64-32-bytes>
```

---

## Phase 6: File Upload & Management

**Priority**: 🔴 CRITICAL
**Estimated Time**: 5 hours
**Dependencies**: Phase 0 ✅ + Phase 0.5 ✅ + Phase 1 ✅ + Phase 2 ✅ + Phase 3 ✅ + Phase 4 ✅ + Phase 5 ✅

### What This Phase Does
Implement secure file upload and storage system with automatic document indexing:
- Secure file upload with presigned URLs
- File validation and virus scanning
- Automatic document indexing to Knowledge Base
- File storage management with DigitalOcean Spaces
- Document versioning and metadata extraction
- File access control and permissions
- Background processing for large files
- Document search and retrieval

### Technologies/Services Used
- **AWS SDK v2**: S3-compatible Spaces upload with bearer token auth (from Phase 2)
- **DigitalOcean GenAI API**: Data Source creation endpoint (from Phase 2)
- **Fiber**: Multipart form handling
- **File Type Detection**: h2non/filetype for MIME detection
- **Bearer Authentication**: Uses `DIGITALOCEAN_TOKEN` for Spaces API access
- **Background Jobs**: Async document processing
- **Virus Scanning**: ClamAV integration (optional)

### File Upload Flow
1. **Validate Request** - Check file type, size, user permissions
2. **Upload to Spaces** - Backend uploads file to DigitalOcean Spaces using AWS SDK v2 with bearer token authentication
3. **Get Object Key** - Retrieve Spaces object key from upload response
4. **Process File** - Extract metadata, validate content, scan for viruses
5. **Create Data Source** - Add file to subject's Knowledge Base via `/v2/gen-ai/knowledge_bases/{kb_uuid}/data_sources` endpoint
6. **Store Metadata** - Save document record with Spaces object key and indexing status
7. **Background Processing** - Handle large files and complex operations

### External Services Used
**DigitalOcean Services:**
- **Spaces**: S3-compatible file storage with CDN delivery
  - Authentication: Bearer token (`Authorization: Bearer $DIGITALOCEAN_TOKEN`)
  - Path: `courses/<course>/<subject>/<filename>`
  - Versioning enabled
  - CDN URL: `https://study-in-woods.blr1.cdn.digitaloceanspaces.com`
  - Upload: AWS SDK v2 with bearer auth

- **Knowledge Base Data Source**: Document indexing via GenAI API
  - Endpoint: `/v2/gen-ai/knowledge_bases/{kb_uuid}/data_sources`
  - Input: Spaces object key from upload
  - Automatic embedding generation
  - Full-text search capability
  - RAG-ready documents

### Result
- ✅ Secure file upload API with presigned URLs
- ✅ File validation and security scanning
- ✅ Automatic document indexing to DigitalOcean Knowledge Base
- ✅ File storage management with Spaces
- ✅ Document metadata extraction and storage
- ✅ File access control and permissions
- ✅ Document versioning system
- ✅ Background processing for file operations
- ✅ Document search and retrieval API
- ✅ File cleanup and storage optimization

### Environment Variables Required
```env
# From Phase 2 (Spaces uses DIGITALOCEAN_TOKEN for bearer auth)
DIGITALOCEAN_TOKEN=dop_v1_xxxxx
DO_SPACES_BUCKET=study-in-woods
DO_SPACES_ENDPOINT=https://blr1.digitaloceanspaces.com
DO_SPACES_CDN_ENDPOINT=https://study-in-woods.blr1.cdn.digitaloceanspaces.com

# File Upload Limits
MAX_FILE_SIZE_MB=50
ALLOWED_FILE_EXTENSIONS=pdf,docx,txt,md,csv,xlsx,html,json,pptx,doc,xls
COURSE_STORAGE_LIMIT=536870912  # 0.5GB in bytes

# Rate Limiting
RATE_LIMIT_UPLOAD=10  # uploads per minute
```

---

## Phase 7: Chat System with AI

**Priority**: 🔴 CRITICAL
**Estimated Time**: 6 hours
**Dependencies**: Phase 0 ✅ + Phase 0.5 ✅ + Phase 1 ✅ + Phase 2 ✅ + Phase 3 ✅ + Phase 4 ✅ + Phase 5 ✅ + Phase 6 ✅

### What This Phase Does
Implement real-time chat system with AI agent integration:
- Chat session management with subjects
- Real-time messaging with WebSocket support
- AI agent integration via DigitalOcean GenAI API
- Message history and context management
- Streaming responses from AI
- Chat analytics and usage tracking
- Rate limiting and quota management
- Message search and export

### Technologies/Services Used
- **DigitalOcean Agent API**: RAG-powered chat (from Phase 2)
- **WebSocket**: Real-time bidirectional communication (fasthttp/websocket)
- **SSE (Server-Sent Events)**: Alternative streaming option
- **Redis**: Chat session caching (from Phase 0)
- **GORM**: Message persistence (from Phase 1)
- **Token Counting**: tiktoken-go for usage tracking
- **Context Management**: Sliding window for chat history

### Chat Flow
1. **Create Session** - User starts chat with subject's AI agent
2. **Send Message** - User sends message to AI
3. **Stream Response** - AI agent responds with streaming
4. **Save History** - Store messages in database
5. **Track Usage** - Monitor API usage and token costs
6. **Apply Limits** - Enforce rate limits and quotas

### External Services Used
**DigitalOcean GenAI Platform:**
- **AI Agents**: Chat completion API with streaming
  - Model: Claude 3 Sonnet
  - Context window: 200k tokens
  - Streaming: Real-time token delivery

- **Knowledge Base**: Context retrieval from documents
  - RAG (Retrieval-Augmented Generation)
  - Semantic search on uploaded documents
  - Automatic context injection

- **Usage Tracking**: Monitor API calls and tokens
  - Input tokens counted
  - Output tokens counted
  - Cost calculation

### Result
- ✅ Chat session CRUD operations
- ✅ Real-time messaging with WebSocket
- ✅ AI agent integration with streaming
- ✅ Message history and pagination
- ✅ Context-aware AI responses
- ✅ Chat analytics and statistics
- ✅ Rate limiting per user/subject
- ✅ Message search and filtering
- ✅ Chat export functionality
- ✅ Usage tracking and quotas

### Environment Variables Required
```env
# From Phase 2
DIGITALOCEAN_TOKEN=dop_v1_xxxxx

# From Phase 5 (encrypted agent keys)
ENCRYPTION_KEY=<base64-32-bytes>

# Chat Configuration
RATE_LIMIT_CHAT=20  # messages per minute
CHAT_MAX_CONTEXT_LENGTH=4000  # tokens
CHAT_HISTORY_LIMIT=50  # messages per session
```

---

## Phase 8: Payment Integration

**Priority**: 🔴 CRITICAL
**Estimated Time**: 8 hours
**Dependencies**: Phase 0 ✅ + Phase 0.5 ✅ + Phase 1 ✅ + Phase 3 ✅ + Phase 4 ✅ + Phase 7 ✅

### What This Phase Does
Implement payment processing and subscription management:
- Payment gateway integration (Razorpay/Stripe)
- Course payment with storage quota allocation
- Webhook handling for payment verification
- Payment history and receipts
- Refund handling
- Payment status tracking
- Invoice generation
- Transaction security

### Technologies/Services Used
- **Razorpay SDK**: razorpay/razorpay-go (primary gateway)
- **Stripe SDK**: stripe/stripe-go (secondary gateway)
- **Webhook Verification**: HMAC signature validation
- **Transaction Service**: Atomic payment operations (from Phase 1)
- **GORM Transactions**: Payment record atomicity
- **PDF Generation**: go-pdf/fpdf for invoices (optional)
- **Email Service**: Payment confirmation emails (optional)

### Payment Flow
1. **Select Course** - User selects course to purchase
2. **Create Order** - Backend creates Razorpay/Stripe order
3. **Process Payment** - User completes payment on gateway
4. **Verify Webhook** - Razorpay/Stripe sends webhook to backend
5. **Validate Signature** - Backend verifies webhook signature
6. **Update Database** - Update payment status in database
7. **Allocate Quota** - Allocate storage quota to user's course
8. **Send Confirmation** - Send payment confirmation email

### External Services Used
**Payment Gateways:**
1. **Razorpay** (Primary - India)
   - Payment Gateway: Course purchases
   - Webhooks: payment.captured, payment.failed
   - Refunds: Cancellation handling
   - Dashboard: Payment analytics
   - Test Mode: Test keys for development

2. **Stripe** (Secondary - International)
   - Payment Gateway: Course purchases
   - Webhooks: checkout.session.completed
   - Refunds: Cancellation handling
   - Dashboard: Payment analytics
   - Test Mode: Test keys for development

### Result
- ✅ Payment gateway integration (Razorpay/Stripe)
- ✅ Secure payment processing
- ✅ Webhook verification with HMAC
- ✅ Payment history and receipts
- ✅ Refund handling
- ✅ Storage quota allocation
- ✅ Transaction atomicity
- ✅ Payment confirmation emails

### Environment Variables Required
```env
# Razorpay Configuration (Primary)
RAZORPAY_KEY_ID=rzp_test_XXXXXXXXXXXX
RAZORPAY_KEY_SECRET=XXXXXXXXXXXXXXXXXXXXXXXX
RAZORPAY_WEBHOOK_SECRET=XXXXXXXXXXXXXXXXXXXXXXXX

# Stripe Configuration (Secondary - optional)
STRIPE_PUBLIC_KEY=pk_test_XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
STRIPE_SECRET_KEY=sk_test_XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
STRIPE_WEBHOOK_SECRET=whsec_XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX

# Payment Settings
PAYMENT_CURRENCY=INR
PAYMENT_TIMEOUT=15m
RAZORPAY_COURSE_PRICE=10000  # ₹100 in paise
```

---

## Phase 9: External API Access

**Priority**: 🟡 HIGH
**Estimated Time**: 4 hours
**Dependencies**: Phase 0 ✅ + Phase 0.5 ✅ + Phase 1 ✅ + Phase 3 ✅ + Phase 4 ✅

### What This Phase Does
Implement external API access for third-party integrations:
- API key generation and management
- RESTful API endpoints for external access
- API authentication and authorization
- Rate limiting per API key
- API usage tracking and analytics
- API documentation (OpenAPI/Swagger)
- Webhook management for events
- API versioning support
- Developer portal and dashboard

### Technologies/Services Used
- **UUID**: API key generation (google/uuid)
- **Redis**: API key caching (from Phase 0)
- **GORM**: API key storage (from Phase 1)
- **Fiber Middleware**: API key authentication
- **Swagger**: API documentation (swaggo/swag)
- **Rate Limiter**: Per-key rate limiting (from Phase 0.5)
- **Usage Logger**: Request tracking and analytics

### API Access Flow
1. **Generate API Key** - User creates API key with scopes
2. **Authenticate** - External app uses API key in header
3. **Validate Key** - Check key validity and scopes
4. **Check Limits** - Verify rate limits and quotas
5. **Make Request** - Execute API operation
6. **Track Usage** - Log API call with metadata
7. **Return Response** - Send JSON response

### External Services Used
None (Provides external access to internal services)

### Result
- ✅ API key CRUD operations
- ✅ External API endpoints
- ✅ API key authentication middleware
- ✅ Rate limiting per key
- ✅ Usage tracking and quotas
- ✅ API documentation (Swagger)
- ✅ Webhook management
- ✅ Developer dashboard
- ✅ API analytics

### Environment Variables Required
```env
# API Key Configuration
API_KEY_EXPIRY=365d  # 1 year default
API_KEY_PREFIX=sk_live_  # or sk_test_

# Rate Limiting (per API key)
API_KEY_RATE_LIMIT=100  # requests per minute
API_KEY_QUOTA_MONTHLY=10000  # requests per month
```

---

## Phase 10: Cron Jobs & Background Tasks

**Priority**: 🟡 HIGH
**Estimated Time**: 4 hours
**Dependencies**: Phase 0 ✅ + Phase 1 ✅ + Phase 2 ✅ + Phase 5 ✅ + Phase 6 ✅ + Phase 7 ✅ + Phase 8 ✅

### What This Phase Does
Implement scheduled background tasks and cron jobs:
- Cron job scheduler setup
- Database cleanup tasks
- Usage statistics aggregation
- DigitalOcean config sync
- Document indexing status checks
- Email notification sending
- Report generation
- Health check monitoring

### Technologies/Services Used
- **Cron Scheduler**: robfig/cron/v3
- **GORM**: Database cleanup operations (from Phase 1)
- **DigitalOcean API**: Config fetching (from Phase 2)
- **Redis**: Distributed locks for cron jobs (from Phase 0)
- **Email Service**: Notification delivery (optional)
- **Background Jobs**: Async task processing

### Scheduled Tasks
1. **Every 5 minutes**: Process email queue
2. **Every 15 minutes**: Check document indexing status
3. **Every 30 minutes**: Cleanup pending uploads
4. **Every hour**: Aggregate usage statistics
5. **Every 6 hours**: Sync DigitalOcean resources (Model UUIDs)
6. **Daily at 2 AM**: Clean up old data (sessions, blacklist)
7. **Daily at 6:30 AM**: DO config sync (VPC UUID, Project ID)
8. **Weekly**: Generate weekly analytics reports
9. **Monthly**: Process monthly billing

### External Services Used
**DigitalOcean API (Config Sync):**
- **VPC UUID**: Fetch for OpenSearch clusters
- **Project ID**: Fetch for resource organization
- **Embedding Model UUID**: Fetch latest embedding model
- **API Limits**: Check rate limit status

### Result
- ✅ Cron job scheduler running
- ✅ Automated cleanup tasks
- ✅ Usage statistics aggregation
- ✅ DigitalOcean config sync
- ✅ Document sync jobs
- ✅ Email queue processing
- ✅ Report generation
- ✅ Health monitoring
- ✅ Job logging and error handling

### Environment Variables Required
```env
# Cron Configuration
CRON_ENABLED=true

# Cron Schedules (cron format)
CRON_DO_CONFIG_SYNC=30 6 * * *           # Daily 6:30 AM
CRON_MODEL_UUID_SYNC=0 */6 * * *         # Every 6 hours
CRON_CLEANUP_PENDING_UPLOADS=*/30 * * * *  # Every 30 min
CRON_CLEANUP_OLD_SESSIONS=0 2 * * *      # Daily 2 AM
CRON_USAGE_AGGREGATION=0 * * * *         # Every hour
CRON_EMAIL_QUEUE=*/5 * * * *             # Every 5 min
```

---

## Phase 11: Admin Panel Endpoints

**Priority**: 🟡 HIGH
**Estimated Time**: 5 hours
**Dependencies**: Phase 0 ✅ + Phase 1 ✅ + Phase 3 ✅ + Phase 4 ✅ + Phase 5 ✅ + Phase 7 ✅ + Phase 8 ✅

### What This Phase Does
Implement comprehensive admin panel and management features:
- Admin dashboard with key metrics
- User management (view, edit, suspend, delete)
- Content moderation (subjects, documents, chats)
- System settings management
- Analytics and reporting
- Audit log viewing
- Support ticket management
- System health monitoring
- Bulk operations

### Technologies/Services Used
- **RBAC Middleware**: Role-based access (from Phase 3)
- **GORM**: Database operations (from Phase 1)
- **Audit Logger**: Admin action tracking
- **Analytics Service**: Metrics aggregation
- **Fiber**: Admin API endpoints
- **Encryption**: Settings encryption (from Phase 0.5)

### Admin Features
1. **Dashboard**: Overview of system metrics
   - Total users, courses, subjects
   - Payment statistics
   - Storage usage
   - API usage

2. **User Management**: Manage all users
   - List with filters and search
   - Ban/unban users
   - Promote to admin
   - View user activity

3. **Content Moderation**: Moderate subjects, documents, chats
   - Approve/reject courses
   - Approve/reject subjects
   - Delete inappropriate content
   - Flag suspicious activity

4. **System Settings**: Configure system settings
   - Update app_settings (encrypted)
   - Manage feature flags
   - Configure limits and quotas

5. **Analytics**: View detailed reports
   - User growth
   - Revenue metrics
   - Storage analytics
   - API usage trends

6. **Audit Logs**: Track all admin actions
   - Who did what and when
   - IP address tracking
   - Action rollback (where applicable)

### External Services Used
None (Pure admin operations)

### Result
- ✅ Admin dashboard with metrics
- ✅ User management interface
- ✅ Content moderation tools
- ✅ System settings CRUD
- ✅ Analytics reports
- ✅ Audit log viewer
- ✅ Support ticket system
- ✅ Health monitoring
- ✅ Bulk operations

### Environment Variables Required
```env
# No additional env vars required
# Uses existing auth (Phase 3) and encryption (Phase 0.5) configuration
```

---

## Phase 12: Database Seeding

**Priority**: 🟢 MEDIUM
**Estimated Time**: 2 hours
**Dependencies**: Phase 0 ✅ + Phase 1 ✅ + Phase 3 ✅ + Phase 5 ✅ + Phase 8 ✅

### What This Phase Does
Create seed data and initial setup scripts for development and testing:
- Admin user creation
- Default subscription plans
- Sample universities and courses
- Sample subjects and documents
- Test users with different roles
- Default system settings
- Sample API keys
- Development data for testing

### Technologies/Services Used
- **GORM**: Database seeding (from Phase 1)
- **faker**: Test data generation (go-faker/faker) - optional
- **bcrypt**: Password hashing for seed users (from Phase 0.5)
- **Idempotency**: Safe to run multiple times

### Seed Data Categories
1. **Essential Data**:
   - Admin user: admin@studyinwoods.com / Admin@123
   - System settings
   - Default feature flags

2. **Sample Universities**:
   - Delhi University
   - Mumbai University
   - Bangalore University

3. **Sample Courses**:
   - Computer Science (4 years, 8 semesters)
   - Business Administration (3 years, 6 semesters)
   - Engineering (4 years, 8 semesters)

4. **Sample Subjects**:
   - Complete course structure
   - Subject names and codes
   - **Note**: NO DigitalOcean resources created for seed subjects

5. **Test Users**:
   - user1@test.com / Test@123 (normal user)
   - user2@test.com / Test@123 (normal user)
   - admin@test.com / Test@123 (admin user)

### External Services Used
None (Pure database operations, no DO resources created)

### Result
- ✅ Admin user created
- ✅ Sample universities seeded
- ✅ Sample courses and semesters created
- ✅ Sample subjects created (without DO resources)
- ✅ Test users created
- ✅ Default settings configured
- ✅ Development environment ready
- ✅ Idempotent seeding (safe to re-run)

### Environment Variables Required
```env
# Development Only
AUTO_SEED=false  # Set to true for auto-seeding on startup
SEED_ADMIN_EMAIL=admin@studyinwoods.com
SEED_ADMIN_PASSWORD=Admin@123
```

---

## 📊 Complete Environment Variables Reference

### Required (Production)
```env
# Database
DATABASE_URL=postgresql://user:pass@host:5432/dbname
DB_MAX_CONNECTIONS=100
DB_MAX_IDLE_CONNECTIONS=10

# Security
JWT_SECRET=<openssl rand -base64 32>
ENCRYPTION_KEY=<openssl rand -base64 32>

# Redis
REDIS_URL=redis://localhost:6379

# DigitalOcean (Universal token for GenAI API and Spaces)
DIGITALOCEAN_TOKEN=dop_v1_xxxxx
DO_SPACES_BUCKET=study-in-woods
DO_SPACES_REGION=blr1
DO_SPACES_ENDPOINT=https://blr1.digitaloceanspaces.com
DO_SPACES_CDN_ENDPOINT=https://study-in-woods.blr1.cdn.digitaloceanspaces.com

# Payment (Razorpay)
RAZORPAY_KEY_ID=rzp_test_xxxxx
RAZORPAY_KEY_SECRET=xxxxx
RAZORPAY_WEBHOOK_SECRET=xxxxx
RAZORPAY_COURSE_PRICE=10000
```

### Optional (Auto-fetched or Has Defaults)
```env
# Auto-synced by cron (Phase 10)
DO_VPC_UUID=<auto>
DO_PROJECT_ID=<auto>
DO_EMBEDDING_MODEL_UUID=<auto>

# Has defaults
PORT=3000
APP_ENV=development
LOG_LEVEL=info
RATE_LIMIT_REQUESTS=25
MAX_FILE_SIZE_MB=50
COURSE_STORAGE_LIMIT=536870912  # 0.5GB
```

---

## 🚀 Implementation Timeline

| Phase | Priority | Time | Dependencies |
|-------|----------|------|--------------|
| Phase 0 | 🔴 Critical | 4h | None |
| Phase 0.5 | 🔴 Critical | 3h | Phase 0 |
| Phase 1 | 🔴 Critical | 6h | Phase 0, 0.5 |
| Phase 2 | 🔴 Critical | 5h | Phase 0, 0.5, 1 |
| Phase 3 | 🔴 Critical | 4h | Phase 0, 0.5, 1, 2 |
| Phase 4 | 🔴 Critical | 6h | Phase 0, 0.5, 1, 2, 3 |
| Phase 5 | 🔴 Critical | 6h | Phase 0, 0.5, 1, 2, 3, 4 |
| Phase 6 | 🔴 Critical | 5h | Phase 0, 0.5, 1, 2, 3, 4, 5 |
| Phase 7 | 🔴 Critical | 6h | Phase 0, 0.5, 1, 2, 3, 4, 5, 6 |
| Phase 8 | 🔴 Critical | 8h | Phase 0, 0.5, 1, 3, 4, 7 |
| Phase 9 | 🟡 High | 4h | Phase 0, 0.5, 1, 3, 4 |
| Phase 10 | 🟡 High | 4h | Phase 0, 1, 2, 5, 6, 7, 8 |
| Phase 11 | 🟡 High | 5h | Phase 0, 1, 3, 4, 5, 7, 8 |
| Phase 12 | 🟢 Medium | 2h | Phase 0, 1, 3, 5, 8 |
| **Total** | | **68h** | |

---

## 📚 Key External Service Summary

### DigitalOcean GenAI Platform (Phases 2, 5, 6, 7, 10)
- **Knowledge Bases**: Document indexing, RAG, semantic search
- **Agents**: AI chat with Claude 3 Sonnet
- **Data Sources**: File upload and processing
- **Embeddings**: Text vectorization
- **API Rate Limit**: 5000 requests/hour

### DigitalOcean Spaces (Phases 2, 6)
- **Storage**: S3-compatible object storage
- **Authentication**: Bearer token using `DIGITALOCEAN_TOKEN`
- **CDN**: Global content delivery
- **Region**: blr1 (Bangalore)
- **Features**: Versioning, direct upload via AWS SDK v2

### Razorpay (Phase 8)
- **Payment Gateway**: INR payments (India)
- **Webhooks**: Real-time payment events
- **Features**: Refunds, recurring payments

### Redis (Phases 0, 0.5, 3, 7, 9, 10)
- **Caching**: Distributed cache
- **Sessions**: JWT blacklist
- **Rate Limiting**: Distributed rate limiting
- **Locks**: Cron job coordination

---

**Last Updated**: November 2, 2025
**Status**: ✅ All 14 phases fully documented with consistent format
**Next Step**: Begin Phase 0 implementation
