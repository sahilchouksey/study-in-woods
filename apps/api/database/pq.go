package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
	"github.com/sahilchouksey/go-init-setup/config"
	"github.com/sahilchouksey/go-init-setup/model"
)

// Storage defines the interface that all database implementations must satisfy
type Storage interface {
	// Lifecycle methods
	Init() error
	Close() error
	HealthCheck() error

	// GORM DB access
	GetDB() interface{} // Returns *gorm.DB for GORMStore, *sql.DB for PostgreSQLStore

	// Todo methods (legacy - will be replaced with Repository pattern)
	GetTodos() ([]model.Todo, error)
	AddTodo(todo model.Todo) error
	UpdateTodo(todo model.Todo) error
	DeleteTodo(id int64) error
}

type PostgreSQLStore struct {
	db *sql.DB
}

func Start() (*PostgreSQLStore, error) {
	getEnv, err := config.Get()

	if err != nil {
		return nil, err
	}

	// connectStr := fmt.Sprintf("user=postgres password=lol dbname=postgres sslmode=disable", )
	connectStr := fmt.Sprintf("user=%s password=%s dbname=%s sslmode=%s", getEnv.DB_USER_NAME, getEnv.DB_PASSWORD, getEnv.DB_NAME, getEnv.DB_SSL_MODE)

	db, err := sql.Open("postgres", connectStr)
	if err != nil {
		fmt.Println("Unable to Start PostgresSQL Databse.")
		return nil, err
	}

	log.Println("Successfully connected to PostgresSQL Database.")
	return &PostgreSQLStore{
		db: db,
	}, nil
}

func (s *PostgreSQLStore) Init() error {
	log.Println("Initializing PostgresSQL Database.")
	err := s.Initialize()
	return err
}

func (s *PostgreSQLStore) Close() error {
	log.Println("Closing PostgresSQL Database.")
	return s.db.Close()
}

// HealthCheck verifies the database connection is alive
func (s *PostgreSQLStore) HealthCheck() error {
	return s.db.Ping()
}
