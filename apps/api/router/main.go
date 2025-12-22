package router

import (
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/database"
	"github.com/sahilchouksey/go-init-setup/handlers"
	admin_handlers "github.com/sahilchouksey/go-init-setup/handlers/admin"
	analytics_handlers "github.com/sahilchouksey/go-init-setup/handlers/analytics"
	apikey_handlers "github.com/sahilchouksey/go-init-setup/handlers/apikey"
	auth_handlers "github.com/sahilchouksey/go-init-setup/handlers/auth"
	chat_handlers "github.com/sahilchouksey/go-init-setup/handlers/chat"
	course_handlers "github.com/sahilchouksey/go-init-setup/handlers/course"
	document_handlers "github.com/sahilchouksey/go-init-setup/handlers/document"
	ingest_handlers "github.com/sahilchouksey/go-init-setup/handlers/ingest"
	notification_handlers "github.com/sahilchouksey/go-init-setup/handlers/notification"
	pyq_handlers "github.com/sahilchouksey/go-init-setup/handlers/pyq"
	semester_handlers "github.com/sahilchouksey/go-init-setup/handlers/semester"
	subject_handlers "github.com/sahilchouksey/go-init-setup/handlers/subject"
	syllabus_handlers "github.com/sahilchouksey/go-init-setup/handlers/syllabus"
	todo_handlers "github.com/sahilchouksey/go-init-setup/handlers/todo"
	university_handlers "github.com/sahilchouksey/go-init-setup/handlers/university"
	"github.com/sahilchouksey/go-init-setup/services"
	"github.com/sahilchouksey/go-init-setup/utils"
	"github.com/sahilchouksey/go-init-setup/utils/auth"
	"github.com/sahilchouksey/go-init-setup/utils/cache"
	"github.com/sahilchouksey/go-init-setup/utils/middleware"
	"gorm.io/gorm"
)

func SetupRoutes(app *fiber.App, store database.Storage) {
	// Get JWT secret from environment
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is not set")
	}

	jwtIssuer := os.Getenv("JWT_ISSUER")
	if jwtIssuer == "" {
		jwtIssuer = "study-in-woods-api"
	}

	// Initialize JWT manager with config
	jwtConfig := auth.JWTConfig{
		Secret:        jwtSecret,
		Expiry:        24 * time.Hour,     // Access token expires in 24 hours
		RefreshExpiry: 7 * 24 * time.Hour, // Refresh token expires in 7 days
		Issuer:        jwtIssuer,
	}
	jwtManager := auth.NewJWTManager(jwtConfig)

	// Get DB instance (type assert from interface)
	db, ok := store.GetDB().(*gorm.DB)
	if !ok {
		log.Fatal("Failed to get GORM DB instance")
	}

	// Initialize Redis cache for brute force protection
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379/0"
	}

	redisCache, err := cache.NewRedisCache(redisURL)
	if err != nil {
		log.Printf("Warning: Failed to connect to Redis: %v. Brute force protection will be disabled.", err)
	}

	// Initialize brute force protection
	var bruteForceProtection *middleware.BruteForceProtection
	if redisCache != nil {
		bruteForceProtection = middleware.NewBruteForceProtection(redisCache)
	}

	// Initialize auth middleware with DB for blacklist checking
	authMiddleware := middleware.NewAuthMiddleware(jwtManager, db)

	// Initialize auth handler with brute force protection
	authHandler := auth_handlers.NewAuthHandler(db, jwtManager, bruteForceProtection)

	// Initialize Phase 4 handlers
	universityHandler := university_handlers.NewUniversityHandler(db)
	courseHandler := course_handlers.NewCourseHandler(db)
	semesterHandler := semester_handlers.NewSemesterHandler(db)

	// Initialize Phase 5 handlers with SubjectService
	subjectService := services.NewSubjectService(db)
	subjectHandler := subject_handlers.NewSubjectHandler(db, subjectService)

	// Initialize Phase 6 handlers with DocumentService
	documentService := services.NewDocumentService(db)
	documentHandler := document_handlers.NewDocumentHandler(db, documentService)

	// Initialize Phase 7 handlers with ChatService, ChatContextService, and ChatMemoryService
	chatContextService := services.NewChatContextService(db)
	chatMemoryService := services.NewChatMemoryService(db)
	chatService := services.NewChatService(db)
	chatHandler := chat_handlers.NewChatHandler(db, chatService)
	chatContextHandler := chat_handlers.NewChatContextHandler(chatContextService)
	chatHistoryHandler := chat_handlers.NewChatHistoryHandler(db, chatMemoryService)
	// Set context service and memory service on chat service
	chatService.SetContextService(chatContextService)
	chatService.SetMemoryService(chatMemoryService)

	// Initialize Phase 8 handlers with AnalyticsService
	analyticsService := services.NewAnalyticsService(db)
	analyticsHandler := analytics_handlers.NewAnalyticsHandler(db, analyticsService)

	// Initialize Phase 9 handlers with APIKeyService
	apiKeyService := services.NewAPIKeyService(db)
	apiKeyHandler := apikey_handlers.NewAPIKeyHandler(db, apiKeyService)

	// Initialize Syllabus handlers with SyllabusService and DocumentService
	syllabusService := services.NewSyllabusService(db)
	syllabusHandler := syllabus_handlers.NewSyllabusHandler(db, syllabusService, documentService)

	// Initialize Progress Tracker for SSE streaming (if Redis is available)
	var progressTracker *services.ProgressTracker
	if redisCache != nil {
		progressTracker = services.NewProgressTracker(redisCache)
		syllabusHandler.SetProgressTracker(progressTracker)
		log.Println("Progress tracker initialized for SSE streaming support")
	}

	// Initialize PYQ handlers with PYQService
	pyqService := services.NewPYQService(db)
	pyqHandler := pyq_handlers.NewPYQHandler(db, pyqService)

	// Initialize Notification service and handler
	notificationService := services.NewNotificationService(db)
	notificationHandler := notification_handlers.NewNotificationHandler(notificationService)

	// Initialize Batch Ingest service and handler
	batchIngestService := services.NewBatchIngestService(db, notificationService, pyqService)
	batchIngestHandler := ingest_handlers.NewBatchIngestHandler(batchIngestService)

	// Initialize Batch Document Upload service and handler
	batchDocumentService := services.NewBatchDocumentService(db, notificationService)
	batchDocumentUploadHandler := document_handlers.NewBatchDocumentUploadHandler(batchDocumentService)

	// Apply security middleware
	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		allowedOrigins = "http://localhost:3000,http://localhost:3001"
	}

	middleware.SetupSecurity(app, middleware.SecurityConfig{
		AllowedOrigins:    allowedOrigins,
		RateLimitRequests: 30,          // 30 requests
		RateLimitWindow:   time.Minute, // per minute
	})

	// Health check endpoints (public)
	app.Get("/ping", utils.MakeHTTPHandleFunc(handlers.HandleCheckHealth, store))
	app.Get("/health/detailed", utils.MakeHTTPHandleFunc(handlers.HandleDetailedHealth, store))

	// API v1 group
	api := app.Group("/api/v1")

	// Auth rate limiter: 5 requests per 2 minutes
	authRateLimiter := middleware.NewRateLimiter(5, 2*time.Minute, "auth")

	// Auth routes (public) with rate limiting
	authGroup := api.Group("/auth")
	authGroup.Post("/register", authRateLimiter, authHandler.Register)

	// Login with brute force protection AND rate limiting
	if bruteForceProtection != nil {
		authGroup.Post("/login", authRateLimiter, bruteForceProtection.CheckAndRecordAttempt(), authHandler.Login)
	} else {
		authGroup.Post("/login", authRateLimiter, authHandler.Login)
	}

	authGroup.Post("/refresh", authHandler.RefreshToken)
	authGroup.Post("/forgot-password", authRateLimiter, authHandler.ForgotPassword)
	authGroup.Post("/reset-password", authRateLimiter, authHandler.ResetPassword)

	// Protected auth routes
	authGroup.Post("/logout", authMiddleware.Required(), authHandler.Logout)
	authGroup.Post("/change-password", authMiddleware.Required(), authHandler.ChangePassword)

	// Profile routes (protected)
	profileGroup := api.Group("/profile", authMiddleware.Required())
	profileGroup.Get("/", authHandler.GetProfile)
	profileGroup.Put("/", authHandler.UpdateProfile)

	// ==================== Phase 4: Core API Endpoints ====================

	// Universities routes
	universities := api.Group("/universities")
	universities.Get("/", universityHandler.ListUniversities)                                      // Public: List all universities
	universities.Get("/:id", universityHandler.GetUniversity)                                      // Public: Get university by ID
	universities.Post("/", authMiddleware.RequireAdmin(), universityHandler.CreateUniversity)      // Admin only: Create university
	universities.Put("/:id", authMiddleware.RequireAdmin(), universityHandler.UpdateUniversity)    // Admin only: Update university
	universities.Delete("/:id", authMiddleware.RequireAdmin(), universityHandler.DeleteUniversity) // Admin only: Delete university

	// Courses routes
	courses := api.Group("/courses")
	courses.Get("/", courseHandler.ListCourses)                                       // Public: List all courses
	courses.Get("/:id", courseHandler.GetCourse)                                      // Public: Get course by ID
	courses.Post("/", authMiddleware.RequireAdmin(), courseHandler.CreateCourse)      // Admin only: Create course
	courses.Put("/:id", authMiddleware.RequireAdmin(), courseHandler.UpdateCourse)    // Admin only: Update course
	courses.Delete("/:id", authMiddleware.RequireAdmin(), courseHandler.DeleteCourse) // Admin only: Delete course

	// Semesters routes (nested under courses)
	semesters := courses.Group("/:course_id/semesters")
	semesters.Get("/", semesterHandler.ListSemesters)                                           // Public: List semesters for a course
	semesters.Get("/:number", semesterHandler.GetSemester)                                      // Public: Get semester by number
	semesters.Post("/", authMiddleware.RequireAdmin(), semesterHandler.CreateSemester)          // Admin only: Create semester
	semesters.Put("/:number", authMiddleware.RequireAdmin(), semesterHandler.UpdateSemester)    // Admin only: Update semester
	semesters.Delete("/:number", authMiddleware.RequireAdmin(), semesterHandler.DeleteSemester) // Admin only: Delete semester

	// ==================== Phase 5: Subjects with AI Integration ====================

	// Subjects routes (nested under semesters)
	subjects := api.Group("/semesters/:semester_id/subjects")
	subjects.Get("/", subjectHandler.ListSubjects)                                               // Public: List subjects for a semester
	subjects.Get("/:id", subjectHandler.GetSubject)                                              // Public: Get subject by ID
	subjects.Post("/", authMiddleware.Required(), subjectHandler.CreateSubject)                  // Protected: Create subject with AI
	subjects.Put("/:id", authMiddleware.Required(), subjectHandler.UpdateSubject)                // Protected: Update subject
	subjects.Patch("/:id/star", authMiddleware.RequireAdmin(), subjectHandler.ToggleSubjectStar) // Admin: Toggle subject star status
	subjects.Delete("/", authMiddleware.RequireAdmin(), subjectHandler.DeleteAllSubjects)        // Admin: Delete all subjects in semester
	subjects.Delete("/:id", authMiddleware.Required(), subjectHandler.DeleteSubject)             // Protected: Delete subject with cleanup

	// Semester-level syllabus upload (creates subjects automatically)
	semesterSyllabus := api.Group("/semesters/:semester_id/syllabus")
	semesterSyllabus.Post("/upload", authMiddleware.Required(), syllabusHandler.UploadAndExtractSyllabus) // Protected: Upload syllabus and create subjects

	// ===============================================================================

	// ==================== Phase 6: Document Management ====================

	// Documents routes (nested under subjects)
	documents := api.Group("/subjects/:subject_id/documents")
	documents.Get("/", documentHandler.ListDocuments)                              // Public: List documents for a subject
	documents.Post("/", authMiddleware.Required(), documentHandler.UploadDocument) // Protected: Upload document

	// Batch document upload routes - MUST be before /:id routes to avoid conflict
	documents.Post("/batch-upload", authMiddleware.Required(), batchDocumentUploadHandler.BatchUploadDocuments)         // Protected: Start batch upload
	documents.Get("/upload-jobs", authMiddleware.Required(), batchDocumentUploadHandler.GetDocumentUploadJobsBySubject) // Protected: List upload jobs for subject

	// Parameterized routes (must come after specific routes like /batch-upload and /upload-jobs)
	documents.Get("/:id", documentHandler.GetDocument)                                                      // Public: Get document details
	documents.Put("/:id", authMiddleware.Required(), documentHandler.UpdateDocument)                        // Protected: Update document metadata
	documents.Delete("/:id", authMiddleware.Required(), documentHandler.DeleteDocument)                     // Protected: Delete document
	documents.Get("/:id/download", documentHandler.GetDownloadURL)                                          // Public: Get download URL
	documents.Post("/:id/refresh-status", authMiddleware.Required(), documentHandler.RefreshIndexingStatus) // Protected: Refresh indexing status

	// ==================== Syllabus Extraction ====================

	// Syllabus routes (nested under subjects)
	subjectSyllabus := api.Group("/subjects/:subject_id/syllabus")
	subjectSyllabus.Get("/", syllabusHandler.GetSyllabusBySubject) // Public: Get syllabus for a subject
	subjectSyllabus.Get("/search", syllabusHandler.SearchTopics)   // Public: Search topics in syllabus

	// Multiple syllabuses per subject route
	api.Get("/subjects/:subject_id/syllabuses", syllabusHandler.GetAllSyllabusesBySubject) // Public: Get all syllabuses for a subject

	// Document syllabus extraction
	api.Post("/documents/:document_id/extract-syllabus", authMiddleware.Required(), syllabusHandler.ExtractSyllabus) // Protected: Extract syllabus from document

	// Syllabus management routes
	syllabus := api.Group("/syllabus")
	syllabus.Get("/:id", syllabusHandler.GetSyllabusById)                                   // Public: Get syllabus by ID
	syllabus.Get("/:id/status", syllabusHandler.GetExtractionStatus)                        // Public: Get extraction status
	syllabus.Post("/:id/retry", authMiddleware.Required(), syllabusHandler.RetryExtraction) // Protected: Retry failed extraction
	syllabus.Delete("/:id", authMiddleware.RequireAdmin(), syllabusHandler.DeleteSyllabus)  // Admin: Delete syllabus
	syllabus.Get("/:id/units", syllabusHandler.ListUnits)                                   // Public: List units
	syllabus.Get("/:id/units/:unit_number", syllabusHandler.GetUnit)                        // Public: Get unit by number
	syllabus.Get("/:id/books", syllabusHandler.ListBooks)                                   // Public: List book references

	// ==================== PYQ Extraction ====================

	// PYQ routes (nested under subjects)
	subjectPYQs := api.Group("/subjects/:subject_id/pyqs")
	subjectPYQs.Get("/", pyqHandler.GetPYQsBySubject)                                   // Public: Get PYQ papers for a subject
	subjectPYQs.Get("/search", pyqHandler.SearchQuestions)                              // Public: Search questions in PYQs
	subjectPYQs.Get("/search-available", pyqHandler.SearchAvailablePYQs)                // Public: Search available PYQs from crawlers
	subjectPYQs.Post("/ingest", authMiddleware.Required(), pyqHandler.IngestCrawledPYQ) // Protected: Ingest a crawled PYQ paper

	// Document PYQ extraction
	api.Post("/documents/:document_id/extract-pyq", authMiddleware.Required(), pyqHandler.ExtractPYQ) // Protected: Extract PYQ from document

	// PYQ management routes
	pyqs := api.Group("/pyqs")
	pyqs.Get("/crawler-sources", pyqHandler.GetCrawlerSources)                     // Public: Get available crawler sources
	pyqs.Get("/:id", pyqHandler.GetPYQById)                                        // Public: Get PYQ paper by ID
	pyqs.Get("/:id/status", pyqHandler.GetExtractionStatus)                        // Public: Get extraction status
	pyqs.Get("/:id/questions", pyqHandler.ListQuestions)                           // Public: List questions
	pyqs.Post("/:id/retry", authMiddleware.Required(), pyqHandler.RetryExtraction) // Protected: Retry failed extraction
	pyqs.Delete("/:id", authMiddleware.RequireAdmin(), pyqHandler.DeletePYQ)       // Admin: Delete PYQ paper

	// ==================== Batch Ingest ====================

	// Batch ingest for subjects (add to existing subject PYQs group)
	subjectPYQs.Post("/batch-ingest", authMiddleware.Required(), batchIngestHandler.BatchIngestPYQs)  // Protected: Start batch ingest
	subjectPYQs.Get("/indexing-jobs", authMiddleware.Required(), batchIngestHandler.GetJobsBySubject) // Protected: List indexing jobs for subject

	// Indexing job management routes
	indexingJobs := api.Group("/indexing-jobs", authMiddleware.Required())
	indexingJobs.Get("/:job_id", batchIngestHandler.GetJobStatus)      // Protected: Get job status with items
	indexingJobs.Post("/:job_id/cancel", batchIngestHandler.CancelJob) // Protected: Cancel active job

	// ==================== Notifications ====================

	// Notification routes (all protected - require authentication)
	notifications := api.Group("/notifications", authMiddleware.Required())
	notifications.Get("/", notificationHandler.GetNotifications)           // Protected: List user's notifications
	notifications.Get("/unread-count", notificationHandler.GetUnreadCount) // Protected: Get unread notification count
	notifications.Post("/:id/read", notificationHandler.MarkAsRead)        // Protected: Mark notification as read
	notifications.Post("/read-all", notificationHandler.MarkAllAsRead)     // Protected: Mark all as read
	notifications.Delete("/:id", notificationHandler.DeleteNotification)   // Protected: Delete notification
	notifications.Delete("/", notificationHandler.DeleteAllNotifications)  // Protected: Delete all notifications

	// ======================================================================

	// ==================== Phase 7: Chat Functionality ====================

	// Chat rate limiter: 30 requests per minute (covers AI/chat operations)
	chatRateLimiter := middleware.NewRateLimiter(30, time.Minute, "chat")

	// Chat routes (all protected - require authentication)
	chat := api.Group("/chat", authMiddleware.Required())

	// Chat context for dropdown selection (single API call to get all dropdown data)
	chat.Get("/context", chatContextHandler.GetChatContext) // Protected: Get all dropdown data for chat setup

	// Chat context individual endpoints (for lazy loading if needed)
	chatContext := chat.Group("/context")
	chatContext.Get("/universities", chatContextHandler.GetUniversities)                            // Protected: List universities
	chatContext.Get("/universities/:university_id/courses", chatContextHandler.GetCourses)          // Protected: List courses for university
	chatContext.Get("/courses/:course_id/semesters", chatContextHandler.GetSemesters)               // Protected: List semesters for course
	chatContext.Get("/semesters/:semester_id/subjects", chatContextHandler.GetSubjects)             // Protected: List subjects with KB+Agent
	chatContext.Get("/subjects/:subject_id/syllabus", chatContextHandler.GetSubjectSyllabusContext) // Protected: Get syllabus context for prompts

	// Session management
	chat.Get("/sessions", chatHandler.ListSessions)                // Protected: List user's chat sessions
	chat.Post("/sessions", chatHandler.CreateSession)              // Protected: Create new chat session
	chat.Get("/sessions/:id", chatHandler.GetSession)              // Protected: Get session details
	chat.Delete("/sessions/:id", chatHandler.DeleteSession)        // Protected: Delete session
	chat.Post("/sessions/:id/archive", chatHandler.ArchiveSession) // Protected: Archive session

	// Message management
	chat.Get("/sessions/:id/messages", chatHandler.GetMessages)                              // Protected: Get session messages (truncated citations by default)
	chat.Get("/sessions/:id/messages/:messageId/citations", chatHandler.GetMessageCitations) // Protected: Get full citations for a message
	chat.Post("/sessions/:id/messages", chatRateLimiter, chatHandler.SendMessage)            // Protected: Send message (rate limited)

	// Chat history routes (for history page with infinite scroll)
	chatHistory := chat.Group("/history")
	chatHistory.Get("/", chatHistoryHandler.GetAllSessions)                   // Protected: Get all sessions with pagination
	chatHistory.Get("/:id", chatHistoryHandler.GetSessionHistory)             // Protected: Get full session history with messages
	chatHistory.Post("/:id/search", chatHistoryHandler.SearchMemory)          // Protected: Search conversation memory
	chatHistory.Get("/:id/contexts", chatHistoryHandler.GetCompactedContexts) // Protected: Get compacted contexts
	chatHistory.Get("/:id/batches", chatHistoryHandler.GetBatches)            // Protected: Get message batches

	// ======================================================================

	// ==================== Phase 8: Analytics, Monitoring & Reporting ====================

	// Analytics routes (user-specific endpoints, require authentication)
	analytics := api.Group("/analytics", authMiddleware.Required())
	analytics.Get("/me", analyticsHandler.GetMyStats)                                                            // Protected: Get current user's stats
	analytics.Get("/users/:id", analyticsHandler.GetUserStats)                                                   // Protected: Get user stats (self or admin)
	analytics.Get("/subjects/:id", analyticsHandler.GetSubjectStats)                                             // Protected: Get subject stats
	analytics.Get("/subjects/top", analyticsHandler.GetTopSubjects)                                              // Protected: Get top subjects
	analytics.Get("/activities", analyticsHandler.GetUserActivities)                                             // Protected: Get user activities (paginated)
	analytics.Post("/activity", analyticsHandler.LogActivity)                                                    // Protected: Manual activity logging
	analytics.Get("/activity/timeseries", authMiddleware.RequireAdmin(), analyticsHandler.GetActivityTimeSeries) // Admin: Activity time series
	analytics.Get("/chat/timeseries", authMiddleware.RequireAdmin(), analyticsHandler.GetChatUsageTimeSeries)    // Admin: Chat usage time series

	// Admin-only analytics routes
	admin := api.Group("/admin", authMiddleware.RequireAdmin())
	admin.Get("/dashboard", analyticsHandler.GetDashboard)  // Admin: Dashboard statistics
	admin.Get("/audit-logs", analyticsHandler.GetAuditLogs) // Admin: Audit logs
	admin.Get("/health", analyticsHandler.GetSystemHealth)  // Admin: System health check

	// ==================== Phase 11: Admin Panel Endpoints ====================

	// Admin User Management
	admin.Get("/users/stats", func(c *fiber.Ctx) error { return admin_handlers.GetUserStats(c, store) })
	admin.Get("/users", func(c *fiber.Ctx) error { return admin_handlers.ListUsers(c, store) })
	admin.Get("/users/:id", func(c *fiber.Ctx) error { return admin_handlers.GetUser(c, store) })
	admin.Put("/users/:id", middleware.AdminAuditLog(store, "user_update", "users"), func(c *fiber.Ctx) error { return admin_handlers.UpdateUser(c, store) })
	admin.Delete("/users/:id", middleware.AdminAuditLog(store, "user_delete", "users"), func(c *fiber.Ctx) error { return admin_handlers.DeleteUser(c, store) })
	admin.Post("/users/:id/reset-password", middleware.AdminAuditLog(store, "password_reset", "users"), func(c *fiber.Ctx) error { return admin_handlers.ResetUserPassword(c, store) })

	// Admin Analytics
	admin.Get("/analytics/overview", func(c *fiber.Ctx) error { return admin_handlers.GetOverviewAnalytics(c, store) })
	admin.Get("/analytics/users", func(c *fiber.Ctx) error { return admin_handlers.GetUserAnalytics(c, store) })
	admin.Get("/analytics/api-keys", func(c *fiber.Ctx) error { return admin_handlers.GetAPIKeyAnalytics(c, store) })
	admin.Get("/analytics/documents", func(c *fiber.Ctx) error { return admin_handlers.GetDocumentAnalytics(c, store) })
	admin.Get("/analytics/chats", func(c *fiber.Ctx) error { return admin_handlers.GetChatAnalytics(c, store) })

	// Admin Audit Logs
	admin.Get("/audit", func(c *fiber.Ctx) error { return admin_handlers.ListAuditLogs(c, store) })
	admin.Get("/audit/:id", func(c *fiber.Ctx) error { return admin_handlers.GetAuditLog(c, store) })

	// Admin Settings Management
	admin.Get("/settings", func(c *fiber.Ctx) error { return admin_handlers.ListSettings(c, store) })
	admin.Get("/settings/:key", func(c *fiber.Ctx) error { return admin_handlers.GetSetting(c, store) })
	admin.Put("/settings/:key", middleware.AdminAuditLog(store, "setting_update", "settings"), func(c *fiber.Ctx) error { return admin_handlers.UpdateSetting(c, store) })
	admin.Delete("/settings/:key", middleware.AdminAuditLog(store, "setting_delete", "settings"), func(c *fiber.Ctx) error { return admin_handlers.DeleteSetting(c, store) })

	// ===========================================================================

	// ===============================================================================

	// ==================== Phase 9: External API Access ====================

	// API Keys routes (all protected - require authentication)
	apiKeys := api.Group("/api-keys", authMiddleware.Required())
	apiKeys.Post("/", apiKeyHandler.CreateAPIKey)           // Protected: Create new API key
	apiKeys.Get("/", apiKeyHandler.ListAPIKeys)             // Protected: List user's API keys
	apiKeys.Get("/:id", apiKeyHandler.GetAPIKey)            // Protected: Get API key details
	apiKeys.Put("/:id", apiKeyHandler.UpdateAPIKey)         // Protected: Update API key
	apiKeys.Post("/:id/revoke", apiKeyHandler.RevokeAPIKey) // Protected: Revoke API key
	apiKeys.Delete("/:id", apiKeyHandler.DeleteAPIKey)      // Protected: Delete API key
	apiKeys.Get("/:id/usage", apiKeyHandler.GetUsageStats)  // Protected: Get API key usage stats
	apiKeys.Post("/:id/extend", apiKeyHandler.ExtendExpiry) // Protected: Extend API key expiry

	// ===========================================================================

	// ==================== API v2: SSE Streaming Support ====================

	apiv2 := app.Group("/api/v2")

	// Semester-level syllabus upload (Step 1 of two-step SSE process)
	v2Semesters := apiv2.Group("/semesters", authMiddleware.Required())
	v2Semesters.Post("/:semester_id/syllabus/upload", syllabusHandler.UploadSyllabusForStreaming) // Upload only, returns document_id

	// Document extraction with SSE streaming (Step 2 of two-step SSE process)
	v2Documents := apiv2.Group("/documents", authMiddleware.Required())
	v2Documents.Get("/:document_id/extract-syllabus", syllabusHandler.ExtractSyllabusStream) // SSE streaming extraction

	// Extraction job management
	v2Jobs := apiv2.Group("/extraction-jobs", authMiddleware.Required())
	v2Jobs.Get("/active", syllabusHandler.GetMyActiveJob)         // Get user's active job
	v2Jobs.Get("/:job_id", syllabusHandler.GetJobStatus)          // Get job status
	v2Jobs.Get("/:job_id/stream", syllabusHandler.ReconnectToJob) // Reconnect to job stream
	v2Jobs.Post("/:job_id/cancel", syllabusHandler.CancelJob)     // Cancel job

	// ===========================================================================

	// Todo endpoints (keeping existing routes for backward compatibility)
	app.Get("/todos", utils.MakeHTTPHandleFunc(todo_handlers.GetAllTodos, store))
	app.Get("/add/todo", utils.MakeHTTPHandleFunc(todo_handlers.AddTodoHandler, store))
	app.Get("/update/todo", utils.MakeHTTPHandleFunc(todo_handlers.UpdateTodoHandler, store))
	app.Get("/delete/todo", utils.MakeHTTPHandleFunc(todo_handlers.DeleteTodoHandler, store))
}
