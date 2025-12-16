package database

import (
	"fmt"
	"log"
	"time"

	"github.com/sahilchouksey/go-init-setup/config"
	"github.com/sahilchouksey/go-init-setup/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type GORMStore struct {
	db *gorm.DB
}

// StartGORM initializes a GORM connection to PostgreSQL
func StartGORM() (*GORMStore, error) {
	getEnv, err := config.Get()
	if err != nil {
		return nil, err
	}

	// Build DSN (Data Source Name)
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=UTC",
		getEnv.DB_HOST,
		getEnv.DB_USER_NAME,
		getEnv.DB_PASSWORD,
		getEnv.DB_NAME,
		getEnv.DB_PORT,
		getEnv.DB_SSL_MODE,
	)

	// Configure GORM logger
	gormLogger := logger.Default.LogMode(logger.Info)
	if getEnv.GO_ENV == "production" {
		gormLogger = logger.Default.LogMode(logger.Error)
	}

	// Open GORM connection
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:                 gormLogger,
		SkipDefaultTransaction: false,
		PrepareStmt:            true, // Prepare statements for better performance
	})
	if err != nil {
		log.Println("Unable to connect to PostgreSQL with GORM:", err)
		return nil, err
	}

	// Get underlying *sql.DB to configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// Connection pool settings
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	log.Println("Successfully connected to PostgreSQL Database with GORM.")

	return &GORMStore{db: db}, nil
}

// Init runs the AutoMigrate to create/update tables
func (s *GORMStore) Init() error {
	log.Println("Running GORM AutoMigrate for all models...")

	err := s.db.AutoMigrate(
		// User-related models
		&model.User{},
		&model.UserCourse{},
		&model.APIKeyUsageLog{}, // Tracks client-side API key usage (keys stored in browser)

		// Institution & Course hierarchy models
		&model.University{},
		&model.Course{},
		&model.Semester{},
		&model.Subject{},

		// Document model
		&model.Document{},

		// Chat models
		&model.ChatSession{},
		&model.ChatMessage{},

		// Payment model
		&model.CoursePayment{},

		// Application settings
		&model.AppSetting{},

		// Token blacklist
		&model.JWTTokenBlacklist{},

		// Audit & logging models
		&model.CronJobLog{},
		&model.AdminAuditLog{},

		// Syllabus extraction models
		&model.Syllabus{},
		&model.SyllabusUnit{},
		&model.SyllabusTopic{},
		&model.BookReference{},

		// PYQ extraction models
		&model.PYQPaper{},
		&model.PYQQuestion{},
		&model.PYQQuestionChoice{},

		// Indexing job models (for batch operations)
		&model.IndexingJob{},
		&model.IndexingJobItem{},

		// User notification models
		&model.UserNotification{},
	)

	if err != nil {
		log.Println("Error running AutoMigrate:", err)
		return err
	}

	log.Println("GORM AutoMigrate completed successfully!")
	return nil
}

// Close closes the database connection
func (s *GORMStore) Close() error {
	log.Println("Closing GORM PostgreSQL connection...")
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// GetDB returns the GORM DB instance for use in repositories/handlers
func (s *GORMStore) GetDB() interface{} {
	return s.db
}

// HealthCheck verifies the database connection is alive
func (s *GORMStore) HealthCheck() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// GetTodos retrieves all todos from the database
func (s *GORMStore) GetTodos() ([]model.Todo, error) {
	var todos []model.Todo
	result := s.db.Find(&todos)
	return todos, result.Error
}

// AddTodo adds a new todo to the database
func (s *GORMStore) AddTodo(todo model.Todo) error {
	result := s.db.Create(&todo)
	return result.Error
}

// UpdateTodo updates an existing todo in the database
func (s *GORMStore) UpdateTodo(todo model.Todo) error {
	result := s.db.Model(&model.Todo{}).Where("id = ?", todo.ID).Updates(todo)
	return result.Error
}

// DeleteTodo deletes a todo by ID from the database
func (s *GORMStore) DeleteTodo(id int64) error {
	result := s.db.Delete(&model.Todo{}, id)
	return result.Error
}
