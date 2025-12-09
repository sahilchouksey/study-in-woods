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

	// Initialize Phase 7 handlers with ChatService
	chatService := services.NewChatService(db)
	chatHandler := chat_handlers.NewChatHandler(db, chatService)

	// Initialize Phase 8 handlers with AnalyticsService
	analyticsService := services.NewAnalyticsService(db)
	analyticsHandler := analytics_handlers.NewAnalyticsHandler(db, analyticsService)

	// Initialize Phase 9 handlers with APIKeyService
	apiKeyService := services.NewAPIKeyService(db)
	apiKeyHandler := apikey_handlers.NewAPIKeyHandler(db, apiKeyService)

	// Initialize Syllabus handlers with SyllabusService
	syllabusService := services.NewSyllabusService(db)
	syllabusHandler := syllabus_handlers.NewSyllabusHandler(db, syllabusService)

	// Initialize PYQ handlers with PYQService
	pyqService := services.NewPYQService(db)
	pyqHandler := pyq_handlers.NewPYQHandler(db, pyqService)

	// Apply security middleware
	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		allowedOrigins = "http://localhost:3000,http://localhost:3001"
	}

	middleware.SetupSecurity(app, middleware.SecurityConfig{
		AllowedOrigins:    allowedOrigins,
		RateLimitRequests: 100,             // 100 requests
		RateLimitWindow:   1 * time.Minute, // per minute
	})

	// Health check endpoint (public)
	app.Get("/ping", utils.MakeHTTPHandleFunc(handlers.HandleCheckHealth, store))

	// API v1 group
	api := app.Group("/api/v1")

	// Auth routes (public)
	authGroup := api.Group("/auth")
	authGroup.Post("/register", authHandler.Register)

	// Login with brute force protection
	if bruteForceProtection != nil {
		authGroup.Post("/login", bruteForceProtection.CheckAndRecordAttempt(), authHandler.Login)
	} else {
		authGroup.Post("/login", authHandler.Login)
	}

	authGroup.Post("/refresh", authHandler.RefreshToken)
	authGroup.Post("/forgot-password", authHandler.ForgotPassword)
	authGroup.Post("/reset-password", authHandler.ResetPassword)

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
	subjects.Get("/", subjectHandler.ListSubjects)                                   // Public: List subjects for a semester
	subjects.Get("/:id", subjectHandler.GetSubject)                                  // Public: Get subject by ID
	subjects.Post("/", authMiddleware.Required(), subjectHandler.CreateSubject)      // Protected: Create subject with AI
	subjects.Put("/:id", authMiddleware.Required(), subjectHandler.UpdateSubject)    // Protected: Update subject
	subjects.Delete("/:id", authMiddleware.Required(), subjectHandler.DeleteSubject) // Protected: Delete subject with cleanup

	// ===============================================================================

	// ==================== Phase 6: Document Management ====================

	// Documents routes (nested under subjects)
	documents := api.Group("/subjects/:subject_id/documents")
	documents.Get("/", documentHandler.ListDocuments)                                                       // Public: List documents for a subject
	documents.Get("/:id", documentHandler.GetDocument)                                                      // Public: Get document details
	documents.Post("/", authMiddleware.Required(), documentHandler.UploadDocument)                          // Protected: Upload document
	documents.Put("/:id", authMiddleware.Required(), documentHandler.UpdateDocument)                        // Protected: Update document metadata
	documents.Delete("/:id", authMiddleware.Required(), documentHandler.DeleteDocument)                     // Protected: Delete document
	documents.Get("/:id/download", documentHandler.GetDownloadURL)                                          // Public: Get download URL
	documents.Post("/:id/refresh-status", authMiddleware.Required(), documentHandler.RefreshIndexingStatus) // Protected: Refresh indexing status

	// ==================== Syllabus Extraction ====================

	// Syllabus routes (nested under subjects)
	subjectSyllabus := api.Group("/subjects/:subject_id/syllabus")
	subjectSyllabus.Get("/", syllabusHandler.GetSyllabusBySubject) // Public: Get syllabus for a subject
	subjectSyllabus.Get("/search", syllabusHandler.SearchTopics)   // Public: Search topics in syllabus

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
	subjectPYQs.Get("/", pyqHandler.GetPYQsBySubject)      // Public: Get PYQ papers for a subject
	subjectPYQs.Get("/search", pyqHandler.SearchQuestions) // Public: Search questions in PYQs

	// Document PYQ extraction
	api.Post("/documents/:document_id/extract-pyq", authMiddleware.Required(), pyqHandler.ExtractPYQ) // Protected: Extract PYQ from document

	// PYQ management routes
	pyqs := api.Group("/pyqs")
	pyqs.Get("/:id", pyqHandler.GetPYQById)                                        // Public: Get PYQ paper by ID
	pyqs.Get("/:id/status", pyqHandler.GetExtractionStatus)                        // Public: Get extraction status
	pyqs.Get("/:id/questions", pyqHandler.ListQuestions)                           // Public: List questions
	pyqs.Post("/:id/retry", authMiddleware.Required(), pyqHandler.RetryExtraction) // Protected: Retry failed extraction
	pyqs.Delete("/:id", authMiddleware.RequireAdmin(), pyqHandler.DeletePYQ)       // Admin: Delete PYQ paper

	// ======================================================================

	// ==================== Phase 7: Chat Functionality ====================

	// Chat routes (all protected - require authentication)
	chat := api.Group("/chat", authMiddleware.Required())

	// Session management
	chat.Get("/sessions", chatHandler.ListSessions)                // Protected: List user's chat sessions
	chat.Post("/sessions", chatHandler.CreateSession)              // Protected: Create new chat session
	chat.Get("/sessions/:id", chatHandler.GetSession)              // Protected: Get session details
	chat.Delete("/sessions/:id", chatHandler.DeleteSession)        // Protected: Delete session
	chat.Post("/sessions/:id/archive", chatHandler.ArchiveSession) // Protected: Archive session

	// Message management
	chat.Get("/sessions/:id/messages", chatHandler.GetMessages)  // Protected: Get session messages
	chat.Post("/sessions/:id/messages", chatHandler.SendMessage) // Protected: Send message (supports streaming)

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

	// Todo endpoints (keeping existing routes for backward compatibility)
	app.Get("/todos", utils.MakeHTTPHandleFunc(todo_handlers.GetAllTodos, store))
	app.Get("/add/todo", utils.MakeHTTPHandleFunc(todo_handlers.AddTodoHandler, store))
	app.Get("/update/todo", utils.MakeHTTPHandleFunc(todo_handlers.UpdateTodoHandler, store))
	app.Get("/delete/todo", utils.MakeHTTPHandleFunc(todo_handlers.DeleteTodoHandler, store))
}
